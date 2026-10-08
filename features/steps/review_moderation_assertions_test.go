package steps_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/cucumber/godog"
)

func registerReviewModerationAssertions(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^el sistema responde con estado 200 y el detalle de la reseña identificada por "([^"]*)"$`, func(label string) error {
		var body struct {
			Review reviewSummaryAcceptance `json:"review"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		if s.lastStatus != 200 || body.Review.WorkOrderID != s.reviewState().orders[label] {
			return fmt.Errorf("wrong review detail")
		}
		return nil
	})
	sc.Step(`^el detalle informa el comentario original "([^"]*)", (\d+) estrellas, visibilidad "([^"]*)", consumidor, prestador, orden y operación canónica$`, func(text string, rating int, visibility string) error {
		var body struct {
			Description string                  `json:"description"`
			Review      reviewSummaryAcceptance `json:"review"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		d, err := s.persistedReview(s.reviewState().selected)
		if err != nil {
			return err
		}
		if body.Description != text || body.Review.Rating != rating || body.Review.Visibility != visibility || body.Review.ConsumerID != d.Summary.ConsumerID || body.Review.ProviderID != d.Summary.ProviderID || body.Review.OperationID != d.Summary.OperationID {
			return fmt.Errorf("incomplete restricted detail")
		}
		return nil
	})
	sc.Step(`^la reseña está en la versión (\d+) y conserva el único reporte "([^"]*)" y tres decisiones en su historial$`, func(version int, report string) error {
		d, err := s.persistedReview(s.reviewState().selected)
		if err != nil {
			return err
		}
		if d.Review.Version() != version || d.Report == nil || d.Report.ID() != s.reviewState().reports[report] || d.Total != 3 {
			return fmt.Errorf("review context changed")
		}
		return nil
	})
	sc.Step(`^"([^"]*)" permanece atendido como procedente por la primera decisión$`, func(report string) error { return s.assertReportDecision(report, "upheld", true) })
	sc.Step(`^la página solicitada contiene exactamente una decisión de un total de (\d+) y señala que hay otra página$`, func(total int) error {
		var body struct {
			Decisions struct {
				Items   []json.RawMessage `json:"items"`
				Total   int               `json:"total"`
				HasMore bool              `json:"has_more"`
			} `json:"decisions"`
		}
		if err := json.Unmarshal(s.lastBody, &body); err != nil {
			return err
		}
		if len(body.Decisions.Items) != 1 || body.Decisions.Total != total || !body.Decisions.HasMore {
			return fmt.Errorf("unbounded/incorrect decisions page")
		}
		return nil
	})
	sc.Step(`^el detalle presenta explicaciones y motivos sólo en este acceso de moderación$`, func() error {
		if !strings.Contains(string(s.lastBody), "Private reporter explanation") || !strings.Contains(string(s.lastBody), "Se mantiene la restricción del comentario") {
			return fmt.Errorf("restricted context missing")
		}
		return nil
	})
	sc.Step(`^se confirma exactamente un evento de acceso asociado a la reseña "([^"]*)" con el operador, la acción, el instante real del servidor y la correlación, sin copiar el original, las explicaciones ni los motivos$`, func(label string) error {
		events, err := s.reviewAuditEvents("review", strconv.Itoa(s.reviewState().orders[label]))
		if err != nil {
			return err
		}
		if len(events) != 1 {
			return fmt.Errorf("expected one access audit, got%d", len(events))
		}
		return s.assertReviewEvent(events[0], audit.ActionAccess, audit.ResultPrepared)
	})
	sc.Step(`^la respuesta no contiene el comentario original, explicaciones ni motivos de moderación$`, func() error {
		for _, private := range []string{"Quedó impecable y limpio", "Private reporter explanation", "Restricted review fixture"} {
			if strings.Contains(string(s.lastBody), private) {
				return fmt.Errorf("private context leaked")
			}
		}
		return nil
	})
	sc.Step(`^el sistema responde con estado 200 y la visibilidad de la reseña pasa a "hidden"$`, func() error {
		if s.lastStatus != 200 {
			return fmt.Errorf("expected200")
		}
		return s.assertReviewVisibility(s.reviewState().selected, false, 2)
	})
	sc.Step(`^el sistema responde con estado 200 y la reseña pasa a visibilidad "hidden"$`, func() error {
		if s.lastStatus != 200 {
			return fmt.Errorf("expected200")
		}
		return s.assertReviewVisibility(s.reviewState().selected, false, 2)
	})
	sc.Step(`^se conserva el comentario original y queda registrada una decisión nueva vinculada a la reseña$`, s.assertNewReviewDecision)
	sc.Step(`^el comentario original se conserva y queda registrada la decisión sin reportes atendidos$`, func() error {
		if err := s.assertNewReviewDecision(); err != nil {
			return err
		}
		d, err := s.persistedReview(s.reviewState().selected)
		if err != nil {
			return err
		}
		if d.Decisions[0].ReportID() != 0 {
			return fmt.Errorf("direct hide attended unspecified report")
		}
		return nil
	})
	sc.Step(`^la decisión registra el motivo, el operador autenticado y el instante actual del servidor$`, s.assertDecisionIdentityAndTime)
	sc.Step(`^queda confirmado exactamente un evento asociado a esa decisión y a la reseña "([^"]*)", con operador, acción, instante real del servidor y correlación, sin copiar el original ni el motivo$`, func(label string) error {
		d, err := s.persistedReview(label)
		if err != nil {
			return err
		}
		events, err := s.reviewAuditEvents("review_decision", strconv.Itoa(d.Decisions[0].ID()))
		if err != nil {
			return err
		}
		if len(events) != 1 {
			return fmt.Errorf("decision audit cardinality")
		}
		if d.Decisions[0].WorkOrderID() != s.reviewState().orders[label] {
			return fmt.Errorf("decision/review linkage lost")
		}
		return s.assertReviewEvent(events[0], audit.ActionExecute, audit.ResultSucceeded)
	})
	sc.Step(`^"([^"]*)" queda atendido como (procedente|no procedente)$`, func(report, state string) error {
		status := "upheld"
		if state == "no procedente" {
			status = "dismissed"
		}
		return s.assertReportDecision(report, status, false)
	})
	sc.Step(`^el sistema responde con estado 200 y la reseña permanece visible$`, func() error {
		if s.lastStatus != 200 {
			return fmt.Errorf("expected200")
		}
		return s.assertReviewVisibility(s.reviewState().selected, true, 2)
	})
	sc.Step(`^queda registrada una decisión nueva sin ocultar la reseña ni modificar su calificación$`, func() error {
		if err := s.assertNewReviewDecision(); err != nil {
			return err
		}
		d, err := s.persistedReview(s.reviewState().selected)
		if err != nil {
			return err
		}
		if !d.Review.Visible() || d.Review.Rating() != 4 || d.Decisions[0].Action() != "dismiss_reports" {
			return fmt.Errorf("dismissal altered assessment")
		}
		return nil
	})
	sc.Step(`^el sistema responde con estado 200 y la reseña "([^"]*)" queda visible en la versión (\d+)$`, func(label string, version int) error {
		if s.lastStatus != 200 {
			return fmt.Errorf("expected200")
		}
		return s.assertReviewVisibility(label, true, version)
	})
	sc.Step(`^queda registrada la decisión nueva "D2" vinculada a la decisión de ocultación "D1"$`, func() error {
		d, err := s.persistedReview(s.reviewState().selected)
		if err != nil {
			return err
		}
		if d.Total != 2 || d.Decisions[0].PreviousHideID() != d.Decisions[1].ID() {
			return fmt.Errorf("unhide lost hiding decision reference")
		}
		return nil
	})
	sc.Step(`^el historial conserva intacta "D1" y "R1" sigue atendido como procedente$`, func() error {
		d, err := s.persistedReview(s.reviewState().selected)
		if err != nil {
			return err
		}
		if d.Decisions[1].Reason() != "El comentario publica un dato personal" || d.Decisions[1].ReportID() != s.reviewState().reports["R1"] || d.Report.Status() != "upheld" {
			return fmt.Errorf("prior decision/report changed")
		}
		return nil
	})
	sc.Step(`^la reseña permanece visible en la versión (\d+), con dos decisiones y dos eventos históricos, sin una nueva decisión ni otro evento$`, func(version int) error {
		if err := s.assertReviewVisibility(s.reviewState().selected, true, version); err != nil {
			return err
		}
		return s.assertDecisionEventCount(2)
	})
	sc.Step(`^el sistema responde con estado 400 y no registra una decisión ni cambia la reseña ni los reportes$`, func() error {
		if s.lastStatus != 400 {
			return fmt.Errorf("expected400")
		}
		d, err := s.persistedReview(s.reviewState().selected)
		if err != nil {
			return err
		}
		if d.Total != 0 || d.Review.Version() != 1 || !d.Review.Visible() || d.Report == nil || d.Report.Status() != "pending" {
			return fmt.Errorf("invalid input mutated review")
		}
		return nil
	})
	sc.Step(`^la reseña permanece visible y no se registra una decisión$`, func() error {
		d, err := s.persistedReview(s.reviewState().selected)
		if err != nil {
			return err
		}
		if !d.Review.Visible() || d.Total != 0 {
			return fmt.Errorf("unauthorized moderation changed review")
		}
		return nil
	})
	sc.Step(`^el sistema responde con estado 200 y "([^"]*)" permanece oculta en la versión (\d+)$`, func(label string, version int) error {
		if s.lastStatus != 200 {
			return fmt.Errorf("expected200")
		}
		return s.assertReviewVisibility(label, false, version)
	})
	sc.Step(`^"([^"]*)" queda atendido como procedente por esta nueva decisión$`, func(report string) error { return s.assertReportDecision(report, "upheld", false) })
	sc.Step(`^el historial conserva las dos decisiones y el evento asociado a cada una$`, func() error { return s.assertDecisionEventCount(2) })
}
func (s *testSuite) assertNewReviewDecision() error {
	label := s.reviewState().selected
	d, err := s.persistedReview(label)
	if err != nil {
		return err
	}
	if d.Review.Description() != s.reviewState().originals[label] || d.Total != 1 || d.Decisions[0].WorkOrderID() != s.reviewState().orders[label] {
		return fmt.Errorf("original/history linkage changed")
	}
	return nil
}
func (s *testSuite) assertDecisionIdentityAndTime() error {
	d, err := s.persistedReview(s.reviewState().selected)
	if err != nil {
		return err
	}
	operator, err := s.userRepository.FindOperatorIDByAuthID(s.scenarioContext, s.currentAuth0ID)
	if err != nil {
		return err
	}
	decision := d.Decisions[0]
	if decision.Reason() != "El comentario publica un dato personal" || decision.OperatorID() != operator || !decision.CreatedOn().Equal(s.clock.Now()) {
		return fmt.Errorf("decision identity/time not server-derived")
	}
	return nil
}
func (s *testSuite) assertReviewEvent(e *audit.Event, action audit.Action, result audit.Result) error {
	operator, err := s.userRepository.FindOperatorIDByAuthID(s.scenarioContext, auth0IDForAdminEmail("operador@example.com"))
	if err != nil {
		return err
	}
	if e.OperatorID() != operator || e.Action() != action || e.Result() != result || !e.OccurredOn().Equal(s.clock.Now()) || e.CorrelationID() != s.adminRequest.headers.Get("X-Request-ID") || e.CorrelationID() == "" || e.Reason() != nil {
		return fmt.Errorf("incorrect/sensitive review audit")
	}
	return nil
}
func (s *testSuite) assertReportDecision(label, status string, first bool) error {
	d, err := s.persistedReview(s.reviewState().selected)
	if err != nil {
		return err
	}
	if d.Report == nil || d.Report.ID() != s.reviewState().reports[label] || d.Report.Status() != status {
		return fmt.Errorf("report attention mismatch")
	}
	index := 0
	if first {
		index = len(d.Decisions) - 1
	}
	if d.Decisions[index].ReportID() != d.Report.ID() {
		return fmt.Errorf("decision did not identify report")
	}
	return nil
}
func (s *testSuite) assertDecisionEventCount(count int) error {
	d, err := s.persistedReview(s.reviewState().selected)
	if err != nil {
		return err
	}
	if d.Total != int64(count) {
		return fmt.Errorf("unexpected decisions count%d", d.Total)
	}
	for _, decision := range d.Decisions {
		events, err := s.reviewAuditEvents("review_decision", strconv.Itoa(decision.ID()))
		if err != nil {
			return err
		}
		if len(events) != 1 {
			return fmt.Errorf("expectedone eventperdecision")
		}
	}
	return nil
}
