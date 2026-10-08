package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

type reviewModerationState struct {
	orders    map[string]int
	reports   map[string]int
	originals map[string]string
	selected  string
}

func (s *testSuite) reviewState() *reviewModerationState {
	if s.reviewModeration.orders == nil {
		s.reviewModeration.orders = map[string]int{}
		s.reviewModeration.reports = map[string]int{}
		s.reviewModeration.originals = map[string]string{}
	}
	return &s.reviewModeration
}
func registerReviewModerationSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que la orden pagada "([^"]*)"(?:, programada más de 24 horas después de aceptar su propuesta,)? tiene una reseña (visible|oculta)(.*)$`, s.reviewFixtureText)
	sc.Step(`^que la orden pagada "([^"]*)", programada más de 24 horas después de aceptar su propuesta, ya tiene una reseña oculta de (\d+) estrellas$`, func(label string, rating int) error {
		return s.reviewFixture(label, rating, "Quedó impecable y limpio", true)
	})
	sc.Step(`^que la orden pagada "([^"]*)" pertenece a "([^"]*)" como consumidor y a "([^"]*)" como prestador, y tiene una reseña oculta de (\d+) estrellas con el comentario "([^"]*)"$`, func(label, consumer, provider string, rating int, text string) error {
		if consumer != "ana@example.com" || provider != "juan@example.com" {
			return fmt.Errorf("unexpected review participants")
		}
		return s.reviewFixture(label, rating, text, true)
	})
	sc.Step(`^que las órdenes "([^"]*)", "([^"]*)" y "([^"]*)" corresponden a servicios completados y pagados, programados más de 24 horas después de aceptar sus propuestas y con fecha ya transcurrida, con reseñas elegibles$`, func(a, b, c string) error {
		for _, label := range []string{a, b, c} {
			if err := s.reviewFixture(label, 5, "Quedó impecable y limpio", false); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^que las reseñas "([^"]*)" y "([^"]*)" están visibles y la reseña "([^"]*)" está oculta$`, func(a, b, c string) error {
		if err := s.assertReviewVisibility(a, true, 1); err != nil {
			return err
		}
		if err := s.assertReviewVisibility(b, true, 1); err != nil {
			return err
		}
		return s.fixtureHide(c)
	})
	sc.Step(`^que "([^"]*)" y "([^"]*)" no tienen reportes pendientes$`, func(a, b string) error {
		for _, label := range []string{a, b} {
			d, err := s.persistedReview(label)
			if err != nil {
				return err
			}
			if d.Report != nil && d.Report.Status() == "pending" {
				return fmt.Errorf("unexpected pending report")
			}
		}
		return nil
	})
	sc.Step(`^que existen tres órdenes pagadas creadas en este orden: "([^"]*)", "([^"]*)" y "([^"]*)", cada una con una reseña visible, y se captura para cada etiqueta el identificador generado por el sistema$`, func(a, b, c string) error {
		for _, label := range []string{a, b, c} {
			if err := s.reviewFixture(label, 5, "Quedó impecable y limpio", false); err != nil {
				return err
			}
		}
		if !(s.reviewState().orders[a] < s.reviewState().orders[b] && s.reviewState().orders[b] < s.reviewState().orders[c]) {
			return fmt.Errorf("fixture identities are not increasing")
		}
		return nil
	})
	sc.Step(`^que "([^"]*)", prestador calificado en "([^"]*)", reporta su reseña mientras está visible y su único reporte "([^"]*)" queda pendiente con fecha del servidor "([^"]*)"$`, s.fixtureReviewReportAt)
	sc.Step(`^que "([^"]*)", prestador calificado en "([^"]*)", reporta la reseña mientras está visible y su único reporte pendiente es "([^"]*)"$`, func(provider, label, report string) error { return s.fixtureReviewReport(provider, label, report) })
	sc.Step(`^que "([^"]*)", prestador calificado en "([^"]*)", reportó la reseña mientras estaba visible y su único reporte pendiente es "([^"]*)"$`, func(provider, label, report string) error { return s.fixtureReviewReport(provider, label, report) })
	sc.Step(`^que "([^"]*)", prestador calificado en "([^"]*)", reportó la reseña mientras estaba visible y el reporte pendiente "([^"]*)" existe$`, func(provider, label, report string) error { return s.fixtureReviewReport(provider, label, report) })
	sc.Step(`^que "([^"]*)", prestador calificado en "([^"]*)", reporta la reseña mientras sigue visible y su único reporte "([^"]*)" queda pendiente$`, func(provider, label, report string) error { return s.fixtureReviewReport(provider, label, report) })
	sc.Step(`^consulto la bandeja administrativa de reseñas con estado "([^"]*)"$`, func(status string) error {
		return s.sendAdminGet("/admin/reviews", url.Values{"status": {status}}, "review-correlation")
	})
	sc.Step(`^consulto la bandeja administrativa de reseñas en la página (\d+) con límite 2 y estado "([^"]*)"$`, func(page int, status string) error {
		return s.sendAdminGet("/admin/reviews", url.Values{"status": {status}, "page": {strconv.Itoa(page)}, "limit": {"2"}}, "review-correlation")
	})

	sc.Step(`^el sistema responde con estado 200 y contiene exactamente el conjunto de reseñas "([^"]*)", con una sola fila por reseña$`, func(labels string) error { return s.assertReviewPage(labels, false) })
	sc.Step(`^el sistema responde con estado 200 y entrega exactamente las reseñas "([^"]*)" en ese orden$`, func(labels string) error { return s.assertReviewPage(labels, true) })
	sc.Step(`^cada fila informa orden, consumidor, prestador, operación, calificación, visibilidad y cantidad de reportes pendientes "([^"]*)"$`, s.assertReviewSummaries)
	sc.Step(`^las filas con reporte pendiente indican "([^"]*)" como fecha y las demás no tienen fecha de reporte$`, s.assertReviewReportDates)
	sc.Step(`^la bandeja no incluye comentarios originales, explicaciones de reportes ni justificaciones administrativas$`, func() error {
		for _, field := range []string{"description", "explanation", "reason"} {
			if strings.Contains(string(s.lastBody), `"`+field+`"`) {
				return fmt.Errorf("summary exposes %s", field)
			}
		}
		return nil
	})
	sc.Step(`^la página devuelve las reseñas ordenadas por identificador generado descendente, sin repetirlas entre páginas$`, func() error {
		items, err := s.reviewPageRows()
		if err != nil {
			return err
		}
		for i := 1; i < len(items); i++ {
			if items[i-1].WorkOrderID <= items[i].WorkOrderID {
				return fmt.Errorf("unstable review order")
			}
		}
		return nil
	})
	sc.Step(`^consulto el detalle administrativo de la reseña de la orden "([^"]*)"(?: con decisiones página (\d+) límite (\d+))?$`, func(label, pageText, limitText string) error {
		page, limit := 0, 0
		if pageText != "" {
			var err error
			page, err = strconv.Atoi(pageText)
			if err != nil {
				return err
			}
			limit, err = strconv.Atoi(limitText)
			if err != nil {
				return err
			}
		}
		s.reviewState().selected = label
		query := url.Values{}
		if page > 0 {
			query.Set("decisions_page", strconv.Itoa(page))
			query.Set("decisions_limit", strconv.Itoa(limit))
		}
		return s.sendAdminGet(fmt.Sprintf("/admin/reviews/%d", s.reviewState().orders[label]), query, "review-correlation")
	})
	sc.Step(`^que el administrador "([^"]*)" consulta la reseña antes de que exista un reporte$`, func(email string) error {
		if err := s.authenticateReviewAdmin("read:admin_reviews"); err != nil {
			return err
		}
		return s.sendAdminGet(fmt.Sprintf("/admin/reviews/%d", s.reviewState().orders["O1"]), nil, "review-preview")
	})
	// Given decisions use the same moderation HTTP endpoint as When decisions.
	sc.Step(`^que oculté la reseña "([^"]*)" atendiendo "([^"]*)" con categoría "([^"]*)", motivo "([^"]*)" y versión esperada (\d+)$`, func(label, report, category, reason string, version int) error {
		return s.fixtureModeration(label, "hide", category, reason, version, report)
	})
	sc.Step(`^que restablecí la visibilidad de "([^"]*)" con motivo "([^"]*)" y versión esperada (\d+)$`, func(label, reason string, version int) error {
		return s.fixtureModeration(label, "unhide", "", reason, version, "")
	})
	sc.Step(`^que volví a ocultar "([^"]*)" sin atender reportes con categoría "([^"]*)", motivo "([^"]*)" y versión esperada (\d+)$`, func(label, category, reason string, version int) error {
		return s.fixtureModeration(label, "hide", category, reason, version, "")
	})
	sc.Step(`^que oculté "([^"]*)" con categoría "([^"]*)", motivo "([^"]*)" y versión esperada (\d+), atendiendo "([^"]*)" como procedente en la decisión "([^"]*)"$`, func(label, category, reason string, version int, report, decision string) error {
		return s.fixtureModeration(label, "hide", category, reason, version, report)
	})
	sc.Step(`^que oculté la reseña "([^"]*)" con categoría "([^"]*)", motivo "([^"]*)" y versión esperada (\d+), pero se perdió la respuesta$`, func(label, category, reason string, version int) error {
		return s.fixtureModeration(label, "hide", category, reason, version, "")
	})
	sc.Step(`^que el mismo operador restableció "([^"]*)" con motivo "([^"]*)" y versión esperada (\d+)$`, func(label, reason string, version int) error {
		return s.fixtureModeration(label, "unhide", "", reason, version, "")
	})
	sc.Step(`^que oculté "([^"]*)" sin seleccionar reportes con categoría "([^"]*)", motivo "([^"]*)" y versión esperada (\d+)$`, func(label, category, reason string, version int) error {
		return s.fixtureModeration(label, "hide", category, reason, version, "")
	})
	sc.Step(`^oculto la reseña de la orden "([^"]*)" con la categoría "([^"]*)", el motivo "([^"]*)" y la versión esperada (\d+), atendiendo sólo el reporte "([^"]*)"$`, func(label, category, reason string, version int, report string) error {
		return s.requestModeration(label, "hide", category, reason, version, report)
	})
	sc.Step(`^oculto la reseña de la orden "([^"]*)" con la categoría "([^"]*)", el motivo "([^"]*)" y la versión esperada (\d+) sin seleccionar reportes$`, func(label, category, reason string, version int) error {
		return s.requestModeration(label, "hide", category, reason, version, "")
	})
	sc.Step(`^desestimo el reporte "([^"]*)" de la reseña "([^"]*)" con el motivo "([^"]*)" y la versión esperada (\d+)$`, func(report, label, reason string, version int) error {
		return s.requestModeration(label, "dismiss_reports", "", reason, version, report)
	})
	sc.Step(`^restablezco la visibilidad de la reseña "([^"]*)" con el motivo "([^"]*)" y la versión esperada (\d+)$`, func(label, reason string, version int) error {
		return s.requestModeration(label, "unhide", "", reason, version, "")
	})
	sc.Step(`^repito la solicitud original de ocultación de "([^"]*)" con categoría "([^"]*)", motivo "([^"]*)" y versión esperada (\d+)$`, func(label, category, reason string, version int) error {
		return s.requestModeration(label, "hide", category, reason, version, "")
	})
	sc.Step(`^intento ocultar la reseña "([^"]*)" con la categoría "([^"]*)", el motivo "([^"]*)" y la versión esperada (\d+)$`, func(label, category, reason string, version int) error {
		return s.requestModeration(label, "hide", category, reason, version, "")
	})
	sc.Step(`^oculto nuevamente la reseña "([^"]*)", ya oculta, atendiendo "([^"]*)" con categoría "([^"]*)", motivo "([^"]*)" y versión esperada (\d+)$`, func(label, report, category, reason string, version int) error {
		return s.requestModeration(label, "hide", category, reason, version, report)
	})
	sc.Step(`^que para la entrada indicada el resto de la solicitud es válido y estoy autenticado como administrador "([^"]*)" con el permiso "([^"]*)"$`, func(email, permission string) error { return s.authenticateReviewAdmin(permission) })
	sc.Step(`^intento moderar la reseña "([^"]*)" con (.*)$`, s.requestInvalidModeration)
	registerReviewModerationAssertions(sc, s)
	registerReviewVisibilitySteps(sc, s)
}

func (s *testSuite) reviewFixtureText(label, visibility, tail string) error {
	rating := 5
	text := "Quedó impecable y limpio"
	for _, n := range []int{3, 4, 5} {
		if strings.Contains(tail, fmt.Sprintf("de %d estrellas", n)) {
			rating = n
		}
	}
	if index := strings.Index(tail, `con el comentario "`); index >= 0 {
		text = strings.TrimSuffix(tail[index+len(`con el comentario "`):], `"`)
	}
	if err := s.reviewFixture(label, rating, text, visibility == "oculta"); err != nil {
		return err
	}
	if strings.Contains(tail, `el único reporte pendiente "R1"`) {
		return s.fixtureReviewReport("juan@example.com", label, "R1")
	}
	return nil
}
func (s *testSuite) reviewFixture(label string, rating int, text string, hidden bool) error {
	state := s.reviewState()
	if _, exists := state.orders[label]; exists {
		return fmt.Errorf("duplicate review fixture %s", label)
	}
	if err := s.withInboxFixtureClock(func() error {
		accepted := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
		reported := accepted.Add(49 * time.Hour)
		paid := reported.Add(time.Hour)
		if err := s.createActivityOrder("moderation-"+label, "ana@example.com", "juan@example.com", 10000, 1000, accepted, reported, paid); err != nil {
			return err
		}
		id := s.operationInbox.orders["moderation-"+label]
		state.orders[label] = id
		s.operationInbox.orders[label] = id
		state.originals[label] = text
		return s.reputationAddReview(id, rating, text)
	}); err != nil {
		return err
	}
	if hidden {
		return s.fixtureHide(label)
	}
	return nil
}
func (s *testSuite) fixtureHide(label string) error {
	auth, permissions := s.currentAuth0ID, s.currentPermissions
	defer func() { s.currentAuth0ID, s.currentPermissions = auth, permissions }()
	if err := s.authenticateReviewAdmin("write:admin_reviews"); err != nil {
		return err
	}
	return s.fixtureModeration(label, "hide", "personal_data", "Restricted review fixture", 1, "")
}
func (s *testSuite) authenticateReviewAdmin(permission string) error {
	if err := s.thereIsProvisionedAdmin("operador@example.com", "Sofía", "López"); err != nil {
		return err
	}
	s.currentAuth0ID = auth0IDForAdminEmail("operador@example.com")
	s.currentPermissions = []string{permission}
	return nil
}
func (s *testSuite) fixtureReviewReportAt(provider, label, report, at string) error {
	instant, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return err
	}
	if !s.clock.Now().Equal(instant) {
		return fmt.Errorf("report fixture time does not match server clock")
	}
	return s.fixtureReviewReport(provider, label, report)
}
func (s *testSuite) fixtureReviewReport(provider, label, report string) error {
	order, err := s.workOrderRepository.FindByID(s.scenarioContext, s.reviewState().orders[label])
	if err != nil {
		return err
	}
	if !order.Review().Visible() {
		return fmt.Errorf("cannot prepare first report on hidden review")
	}
	response, err := s.postJSONWithAuth(auth0IDForProviderEmail(provider), fmt.Sprintf("/work-orders/%d/reviews/reports", order.ID()), map[string]any{"category": "personal_data", "explanation": "Private reporter explanation"})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var receipt struct {
		ID int `json:"id"`
	}
	if err = json.NewDecoder(response.Body).Decode(&receipt); err != nil {
		return err
	}
	if response.StatusCode != 201 || receipt.ID <= 0 {
		return fmt.Errorf("report fixture returned %d", response.StatusCode)
	}
	s.reviewState().reports[report] = receipt.ID
	return nil
}
func (s *testSuite) fixtureModeration(label, action, category, reason string, version int, report string) error {
	if err := s.requestModeration(label, action, category, reason, version, report); err != nil {
		return err
	}
	if s.lastStatus != 200 {
		return fmt.Errorf("moderation fixture returned %d: %s", s.lastStatus, s.lastBody)
	}
	return nil
}
func (s *testSuite) requestModeration(label, action, category, reason string, version int, report string) error {
	body := map[string]any{"action": action, "reason": reason, "expected_version": version}
	if category != "" {
		body["category"] = category
	}
	if report != "" {
		id, ok := s.reviewState().reports[report]
		if !ok {
			return fmt.Errorf("unknown report %s", report)
		}
		body["report_id"] = id
	}
	s.reviewState().selected = label
	return s.sendReviewPost(fmt.Sprintf("/admin/reviews/%d/moderate", s.reviewState().orders[label]), body)
}
func (s *testSuite) sendReviewPost(path string, body map[string]any) error {
	return s.sendAuthenticatedJSON(http.MethodPost, path, "", body)
}

func (s *testSuite) requestInvalidModeration(label, entry string) error {
	action, category, reason, report := "hide", "personal_data", "Valid reason", ""
	switch {
	case strings.Contains(entry, "repetitive_reporting"):
		category = "repetitive_reporting"
	case strings.Contains(entry, "motivo vacío"):
		reason = ""
	case strings.Contains(entry, "500 bytes"):
		reason = strings.Repeat("ñ", 251)
	case strings.Contains(entry, "desestimar"):
		action = "dismiss_reports"
		category = ""
	default:
		return fmt.Errorf("unsupported invalid input fixture %s", entry)
	}
	return s.requestModeration(label, action, category, reason, 1, report)
}
func (s *testSuite) persistedReview(label string) (*workorder.AdminReviewDetail, error) {
	return s.dependencies.Persistence.AdminReviewReader.FindByID(s.scenarioContext, s.reviewState().orders[label], workorder.ReviewPageInput{Page: 1, Limit: 100})
}
func (s *testSuite) assertReviewVisibility(label string, visible bool, version int) error {
	detail, err := s.persistedReview(label)
	if err != nil {
		return err
	}
	if detail.Review.Visible() != visible || (version > 0 && detail.Review.Version() != version) {
		return fmt.Errorf("unexpected review visibility/version for %s", label)
	}
	return nil
}

type reviewSummaryAcceptance struct {
	WorkOrderID        int        `json:"work_order_id"`
	ConsumerID         int        `json:"consumer_id"`
	ProviderID         int        `json:"provider_id"`
	OperationID        string     `json:"operation_id"`
	Rating             int        `json:"rating"`
	Visibility         string     `json:"visibility"`
	PendingReportCount int        `json:"pending_report_count"`
	ReportedOn         *time.Time `json:"reported_on"`
}

func (s *testSuite) reviewPageRows() ([]reviewSummaryAcceptance, error) {
	var page struct {
		Items []reviewSummaryAcceptance `json:"items"`
	}
	err := json.Unmarshal(s.lastBody, &page)
	return page.Items, err
}
func (s *testSuite) assertReviewPage(labels string, ordered bool) error {
	if s.lastStatus != 200 {
		return fmt.Errorf("expected200 got%d", s.lastStatus)
	}
	items, err := s.reviewPageRows()
	if err != nil {
		return err
	}
	actual := []int{}
	for _, item := range items {
		actual = append(actual, item.WorkOrderID)
	}
	expected := []int{}
	for _, label := range strings.Split(labels, ",") {
		expected = append(expected, s.reviewState().orders[strings.TrimSpace(label)])
	}
	if !ordered {
		sort.Ints(actual)
		sort.Ints(expected)
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("unexpected review identities %v want%v", actual, expected)
	}
	return nil
}
func (s *testSuite) assertReviewSummaries(counts string) error {
	items, err := s.reviewPageRows()
	if err != nil {
		return err
	}
	for _, assignment := range strings.Split(counts, ",") {
		pair := strings.Split(strings.TrimSpace(assignment), "=")
		id := s.reviewState().orders[pair[0]]
		count, err := strconv.Atoi(pair[1])
		if err != nil {
			return err
		}
		found := false
		for _, item := range items {
			if item.WorkOrderID != id {
				continue
			}
			found = true
			if item.ConsumerID <= 0 || item.ProviderID <= 0 || item.OperationID == "" || item.Rating <= 0 || item.Visibility == "" || item.PendingReportCount != count {
				return fmt.Errorf("invalid review summary %+v", item)
			}
		}
		if !found {
			return fmt.Errorf("missing summary %s", pair[0])
		}
	}
	return nil
}
func (s *testSuite) assertReviewReportDates(value string) error {
	expected, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}
	items, err := s.reviewPageRows()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.PendingReportCount > 0 {
			if item.ReportedOn == nil || !item.ReportedOn.Equal(expected) {
				return fmt.Errorf("report date is not actual receipt date")
			}
		} else if item.ReportedOn != nil {
			return fmt.Errorf("unexpected report date on unreported review")
		}
	}
	return nil
}
func (s *testSuite) reviewAuditEvents(resourceType, resourceID string) ([]*audit.Event, error) {
	repo := s.dependencies.Persistence.AuditEventRepository
	watermark, err := repo.CaptureWatermark(context.Background())
	if err != nil {
		return nil, err
	}
	return repo.FindPage(context.Background(), audit.LogFilter{ResourceType: &resourceType, ResourceID: &resourceID}, watermark, nil, 100)
}
