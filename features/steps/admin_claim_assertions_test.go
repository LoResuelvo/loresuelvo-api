package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

func registerAdminClaimAssertions(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^el sistema responde con estado 200 y contiene exactamente los reclamos "([^"]*)"$`, s.claimListExact)
	sc.Step(`^el sistema responde con estado 200 y contiene únicamente reclamos en estado "([^"]*)"$`, s.adminClaimListState)
	sc.Step(`^el sistema responde con estado 400 sin datos de reclamos$`, func() error { return s.claimError(400) })
	sc.Step(`^la respuesta no contiene datos de expedientes$`, s.adminClaimErrorNoData)
	sc.Step(`^la respuesta no contiene datos del expediente ni URLs temporales$`, s.adminClaimErrorNoData)
	sc.Step(`^la respuesta no contiene detalle ni evidencias$`, s.adminClaimErrorNoData)
	sc.Step(`^no se genera ni entrega ninguna URL temporal$`, func() error {
		if len(s.adminClaimCapture.ids) != 0 {
			return fmt.Errorf("evidence resolver called after audit failure")
		}
		return s.claimResponseHasNoURL()
	})
	sc.Step(`^cada reclamo aparece una sola vez e informa ID, fecha de reporte, tipo de reclamante, referencia de operación, rubro disponible y estado$`, s.adminClaimSummaryFields)
	sc.Step(`^cada resultado conserva la identidad canónica de su operación y la referencia específica persistida$`, s.adminClaimSummaryReferences)
	sc.Step(`^el total de coincidencias es exactamente (\d+)$`, func(total int) error {
		p, err := s.claimPage()
		if err != nil {
			return err
		}
		if p.Total != total {
			return fmt.Errorf("expected total %d, got %d", total, p.Total)
		}
		return nil
	})
	sc.Step(`^la respuesta informa la página solicitada (\d+) y el límite 2$`, func(page int) error {
		p, err := s.claimPage()
		if err != nil {
			return err
		}
		if p.Page != page || p.Limit != 2 {
			return fmt.Errorf("wrong page or limit")
		}
		return nil
	})
	sc.Step(`^la antigüedad de las filas de la página es exactamente "([^"]*)" desde el reloj observado, sin SLA ni prioridad$`, s.adminClaimAges)
	sc.Step(`^la consulta no registra eventos persistentes de auditoría$`, s.adminClaimAuditUnchanged)
	sc.Step(`^el sistema responde con estado 200 y el detalle de "([^"]*)"$`, s.adminClaimDetailID)
	sc.Step(`^el detalle informa el tipo de reclamante "([^"]*)"$`, s.adminClaimDetailParty)
	sc.Step(`^el detalle presenta el motivo "([^"]*)", el testimonio original "([^"]*)" y la fecha de reporte "([^"]*)"$`, s.adminClaimDetailOriginal)
	sc.Step(`^el detalle conserva la identidad canónica "([^"]*)" seguida del ID persistido de "([^"]*)" y la referencia específica "([^"]*)" de "([^"]*)"$`, s.adminClaimDetailCanonical)
	sc.Step(`^el detalle incluye la imagen vinculada "([^"]*)" con su identificador y una URL temporal privada$`, s.claimImageAuthorized)
	sc.Step(`^el detalle presenta el tipo, fundamentación, fecha, compensación sugerida e historial persistidos$`, s.adminClaimDetailResolution)
	sc.Step(`^la respuesta no contiene mensajes de chat ni transacciones financieras completas$`, s.adminClaimProjectionPrivate)
	sc.Step(`^la compensación se identifica como sugerida y no ejecutada$`, s.adminClaimCompensationSuggested)
	sc.Step(`^la compensación no se identifica como pagada ni ejecutada$`, s.adminClaimCompensationSuggested)
	sc.Step(`^el detalle informa las evidencias como una colección vacía, no nula$`, func() error {
		r, err := s.claimResponse()
		if err != nil {
			return err
		}
		if string(r["images"]) != "[]" {
			return fmt.Errorf("images must be []")
		}
		return nil
	})
	sc.Step(`^el detalle no incluye "([^"]*)" ni una URL temporal para ese archivo$`, s.claimDetailExcludesImage)
	sc.Step(`^antes de generar cualquier URL temporal queda persistido exactamente un evento de acceso al reclamo "([^"]*)" por "([^"]*)" y la correlación "([^"]*)" con resultado "prepared"$`, s.adminClaimAccessPrepared)
	sc.Step(`^el expediente y sus actuaciones permanecen sin cambios$`, func() error { return s.claimUnchanged(s.adminClaims.selected) })
	sc.Step(`^el sistema responde con estado 200 y el reclamo queda en estado "([^"]*)"$`, s.adminClaimStatus)
	sc.Step(`^el sistema responde con estado 200 y el reclamo conserva el estado final "([^"]*)"$`, s.adminClaimStatus)
	sc.Step(`^el sistema responde con estado 200 y el reclamo "([^"]*)" conserva el estado "([^"]*)"$`, func(label, status string) error {
		if label != s.adminClaims.selected {
			return fmt.Errorf("wrong selected claim")
		}
		return s.adminClaimStatus(status)
	})
	sc.Step(`^el sistema responde con estado 200 y el expediente queda resuelto$`, func() error { return s.adminClaimStatus("resolved") })
	sc.Step(`^la actuación registra al operador autenticado y el instante "([^"]*)" del servidor$`, s.adminClaimActionMatches)
	sc.Step(`^el motivo, testimonio, referencias, evidencias y fechas originales del reclamo permanecen sin cambios$`, s.adminClaimOriginalFields)
	sc.Step(`^el expediente conserva su testimonio y referencias originales$`, s.adminClaimOriginalFields)
	sc.Step(`^queda registrada una sola actuación de revisión$`, func() error { return s.adminClaimActionCount("review_started", 1) })
	sc.Step(`^queda registrada una sola actuación final$`, func() error { return s.adminClaimActionCount("resolved", 1) })
	sc.Step(`^queda persistido un evento de ejecución exitoso del reclamo "([^"]*)" por "([^"]*)" en el instante "([^"]*)" y con la correlación "([^"]*)"$`, s.adminClaimExecutionAudit)
	sc.Step(`^el evento sólo registra el cambio de estado de "([^"]*)" a "([^"]*)", sin motivo ni testimonio$`, s.adminClaimAuditTransition)
	sc.Step(`^el evento sólo registra el cambio de estado de "([^"]*)" a "([^"]*)", sin fundamentación ni compensación sugerida$`, s.adminClaimAuditTransition)
	sc.Step(`^la operación, sus propuestas, órdenes y pagos permanecen sin cambios$`, s.adminClaimBusinessUnchanged)
	sc.Step(`^el estado y las actuaciones previas del reclamo "([^"]*)" permanecen sin cambios$`, s.claimUnchanged)
	sc.Step(`^el reclamo "([^"]*)" permanece abierto sin actuaciones administrativas nuevas$`, func(label string) error {
		if s.claims.claims[label].snapshot.Status != claim.StatusOpen {
			return fmt.Errorf("fixture was not open")
		}
		return s.claimUnchanged(label)
	})
	sc.Step(`^el reclamo "([^"]*)" permanece en estado "([^"]*)" sin dictamen ni nueva actuación$`, func(label, status string) error {
		if string(s.claims.claims[label].snapshot.Status) != status || s.claims.claims[label].snapshot.Resolution != nil {
			return fmt.Errorf("invalid original state")
		}
		return s.claimUnchanged(label)
	})
	sc.Step(`^el reclamo "([^"]*)" conserva su estado y dictamen previos$`, s.claimUnchanged)
	sc.Step(`^la respuesta recupera la actuación original con el operador y el instante originales$`, s.adminClaimReplayAction)
	sc.Step(`^la respuesta recupera el dictamen, operador e instante originales$`, s.adminClaimReplayResolution)
	sc.Step(`^no se duplica la actuación de revisión ni su auditoría$`, s.adminClaimReplayUnchanged)
	sc.Step(`^no se duplican el dictamen, la actuación ni su auditoría$`, s.adminClaimReplayUnchanged)
	sc.Step(`^no se registra un dictamen ni una actuación duplicada para "([^"]*)" o "([^"]*)"$`, func(first, second string) error {
		if err := s.claimUnchanged(first); err != nil {
			return err
		}
		if err := s.claimUnchanged(second); err != nil {
			return err
		}
		return s.adminClaimAuditUnchanged()
	})
	sc.Step(`^la respuesta y el reclamo persistido conservan exactamente el tipo, la fundamentación y el instante "([^"]*)" registrado por el servidor$`, s.adminClaimResolutionStored)
	sc.Step(`^el dictamen informa exactamente la compensación sugerida de (\d+) centavos ARS$`, s.claimCompensationMatches)
}

func (s *testSuite) adminClaimRawItems() ([]map[string]json.RawMessage, error) {
	var p struct{ Items []map[string]json.RawMessage }
	err := json.Unmarshal(s.lastBody, &p)
	return p.Items, err
}
func (s *testSuite) adminClaimFindFixture(raw map[string]json.RawMessage) (*claimFixture, error) {
	var id int
	if err := json.Unmarshal(raw["id"], &id); err != nil {
		return nil, err
	}
	for _, f := range s.claims.claims {
		if f.claim.ID == id {
			return f, nil
		}
	}
	return nil, fmt.Errorf("unknown persisted claim %d", id)
}
func (s *testSuite) adminClaimListState(status string) error {
	if s.lastStatus != 200 {
		return fmt.Errorf("list returned %d", s.lastStatus)
	}
	items, err := s.adminClaimRawItems()
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fmt.Errorf("expected exactly one state fixture, got %d", len(items))
	}
	for _, raw := range items {
		if string(raw["status"]) != strconv.Quote(status) {
			return fmt.Errorf("wrong list status")
		}
	}
	return nil
}
func (s *testSuite) adminClaimErrorNoData() error {
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	for _, key := range []string{"items", "id", "claimant", "operation_id", "reference", "description", "reason", "resolution", "actions", "images"} {
		if _, ok := raw[key]; ok {
			return fmt.Errorf("error exposes %s", key)
		}
	}
	return s.claimResponseHasNoURL()
}
func (s *testSuite) adminClaimSummaryReferences() error {
	items, err := s.adminClaimRawItems()
	if err != nil {
		return err
	}
	for _, raw := range items {
		f, err := s.adminClaimFindFixture(raw)
		if err != nil {
			return err
		}
		if err := assertAdminClaimReference(raw, f.claim); err != nil {
			return err
		}
	}
	return nil
}
func assertAdminClaimReference(raw map[string]json.RawMessage, c *claim.Claim) error {
	if string(raw["operation_id"]) != strconv.Quote(fmt.Sprintf("%s-%d", c.OperationID.Kind, c.OperationID.ResourceID)) {
		return fmt.Errorf("canonical operation differs")
	}
	var reference map[string]json.RawMessage
	if err := json.Unmarshal(raw["reference"], &reference); err != nil {
		return err
	}
	if len(reference) != 1 || string(reference[string(c.Reference.Kind())+"_id"]) != c.Reference.ID() {
		return fmt.Errorf("persisted reference differs")
	}
	return nil
}
func (s *testSuite) adminClaimSummaryFields() error {
	items, err := s.adminClaimRawItems()
	if err != nil {
		return err
	}
	seen := map[int]bool{}
	for _, raw := range items {
		f, err := s.adminClaimFindFixture(raw)
		if err != nil {
			return err
		}
		if seen[f.claim.ID] {
			return fmt.Errorf("duplicate claim")
		}
		seen[f.claim.ID] = true
		for _, key := range []string{"id", "created_on", "claimant_party", "operation_id", "reference", "category_id", "status"} {
			if _, ok := raw[key]; !ok {
				return fmt.Errorf("missing summary %s", key)
			}
		}
		var reported time.Time
		if err := json.Unmarshal(raw["created_on"], &reported); err != nil {
			return err
		}
		if !reported.Equal(f.claim.CreatedOn) || string(raw["claimant_party"]) != strconv.Quote(string(f.claim.ClaimantParty)) || string(raw["status"]) != strconv.Quote(string(f.claim.Status)) {
			return fmt.Errorf("summary does not match persisted claim")
		}
		var category *int
		if err := json.Unmarshal(raw["category_id"], &category); err != nil {
			return err
		}
		if category == nil || *category <= 0 {
			return fmt.Errorf("available category missing")
		}
	}
	return s.adminClaimSummaryReferences()
}
func (s *testSuite) adminClaimAges(expected string) error {
	items, err := s.adminClaimRawItems()
	if err != nil {
		return err
	}
	if expected == "ninguna" {
		if len(items) != 0 {
			return fmt.Errorf("expected no ages")
		}
		return nil
	}
	parts := strings.Split(expected, ",")
	if len(parts) != len(items) {
		return fmt.Errorf("age count differs")
	}
	for i, part := range parts {
		fields := strings.Split(strings.TrimSpace(part), ": ")
		if len(fields) != 2 {
			return fmt.Errorf("invalid expected age")
		}
		f := s.claims.claims[fields[0]]
		var age int64
		var id int
		if err := json.Unmarshal(items[i]["age_seconds"], &age); err != nil {
			return err
		}
		if err := json.Unmarshal(items[i]["id"], &id); err != nil {
			return err
		}
		hours, err := strconv.Atoi(strings.TrimSuffix(fields[1], " horas"))
		if err != nil {
			return err
		}
		if id != f.claim.ID || age != int64(hours)*3600 || age != int64(s.clock.Now().Sub(f.claim.CreatedOn).Seconds()) {
			return fmt.Errorf("incorrect exact age for %s", fields[0])
		}
		for _, key := range []string{"sla", "priority"} {
			if _, ok := items[i][key]; ok {
				return fmt.Errorf("unexpected %s", key)
			}
		}
	}
	return nil
}
func (s *testSuite) adminClaimAuditUnchanged() error {
	mark, err := s.dependencies.Persistence.AuditEventRepository.CaptureWatermark(context.Background())
	if err != nil {
		return err
	}
	if mark != s.adminClaims.mark {
		return fmt.Errorf("audit changed from %d to %d", s.adminClaims.mark, mark)
	}
	return nil
}
func (s *testSuite) adminClaimDetailID(label string) error {
	if s.lastStatus != 200 {
		return fmt.Errorf("detail returned %d: %s", s.lastStatus, s.lastBody)
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	if string(raw["id"]) != strconv.Itoa(s.claims.claims[label].claim.ID) {
		return fmt.Errorf("wrong detail ID")
	}
	return nil
}
func (s *testSuite) adminClaimDetailParty(party string) error {
	var raw struct {
		Claimant struct {
			ID           int
			Party, Email string
		}
	}
	if err := json.Unmarshal(s.lastBody, &raw); err != nil {
		return err
	}
	f := s.claims.claims[s.adminClaims.selected]
	if raw.Claimant.ID != f.claim.ClaimantID || raw.Claimant.Party != party || raw.Claimant.Email != f.owner {
		return fmt.Errorf("wrong claimant identity")
	}
	return nil
}
func (s *testSuite) adminClaimDetailOriginal(reason, text, instant string) error {
	var raw struct {
		Reason, Description string
		CreatedOn           time.Time `json:"created_on"`
	}
	if err := json.Unmarshal(s.lastBody, &raw); err != nil {
		return err
	}
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	if raw.Reason != reason || raw.Description != text || !raw.CreatedOn.Equal(now) {
		return fmt.Errorf("original detail changed")
	}
	return nil
}
func (s *testSuite) adminClaimDetailCanonical(prefix, operation, kind, reference string) error {
	if err := s.adminClaimCanonicalFixture(s.adminClaims.selected, prefix, operation, kind, reference); err != nil {
		return err
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	return assertAdminClaimReference(raw, s.claims.claims[s.adminClaims.selected].claim)
}
func (s *testSuite) adminClaimDetailResolution() error {
	if err := s.claimResolutionMatches(s.adminClaims.selected); err != nil {
		return err
	}
	if err := s.claimCompensationMatches(1500); err != nil {
		return err
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	return assertAdminClaimActions(raw["actions"], s.claims.claims[s.adminClaims.selected].claim.Actions)
}
func assertAdminClaimActions(raw json.RawMessage, actions []claim.Action) error {
	var actual []struct {
		ID         int
		Type       string
		ActorID    int         `json:"actor_id"`
		ActorParty claim.Party `json:"actor_party"`
		CreatedOn  time.Time   `json:"created_on"`
	}
	if err := json.Unmarshal(raw, &actual); err != nil {
		return err
	}
	if len(actual) != len(actions) {
		return fmt.Errorf("action history count differs")
	}
	for i, a := range actual {
		e := actions[i]
		if a.ID != e.ID || a.Type != e.Type || a.ActorID != e.ActorID || a.ActorParty != e.ActorParty || !a.CreatedOn.Equal(e.CreatedOn) {
			return fmt.Errorf("persisted action %d differs", i)
		}
	}
	return nil
}
func (s *testSuite) adminClaimCompensationSuggested() error {
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	var resolution map[string]json.RawMessage
	if err := json.Unmarshal(raw["resolution"], &resolution); err != nil {
		return err
	}
	if string(resolution["suggested_compensation"]) == "null" || len(resolution["suggested_compensation"]) == 0 {
		return fmt.Errorf("suggested compensation missing")
	}
	var visit func(any) error
	visit = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				switch k {
				case "paid", "paid_on", "executed", "executed_on", "payment_id", "payment_intent_id", "transaction_id":
					return fmt.Errorf("suggestion claims execution via %s", k)
				}
				if err := visit(v); err != nil {
					return err
				}
			}
		case []any:
			for _, v := range x {
				if err := visit(v); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var value any
	if err := json.Unmarshal(raw["resolution"], &value); err != nil {
		return err
	}
	return visit(value)
}
func (s *testSuite) adminClaimPersistedEvents(label string) ([]*audit.Event, error) {
	reader := s.dependencies.Persistence.AuditEventRepository
	mark, err := reader.CaptureWatermark(context.Background())
	if err != nil {
		return nil, err
	}
	kind, id := "claim", strconv.Itoa(s.claims.claims[label].claim.ID)
	return reader.FindPage(context.Background(), audit.LogFilter{ResourceType: &kind, ResourceID: &id}, mark, nil, 100)
}
func (s *testSuite) adminClaimAccessPrepared(label, email, correlation string) error {
	events, err := s.adminClaimPersistedEvents(label)
	if err != nil {
		return err
	}
	operator, err := s.userRepository.FindOperatorIDByAuthID(context.Background(), auth0IDForAdminEmail(email))
	if err != nil {
		return err
	}
	matches := 0
	for _, event := range events {
		if event.CorrelationID() != correlation {
			continue
		}
		matches++
		if event.OperatorID() != operator || event.Action() != audit.ActionAccess || event.Result() != audit.ResultPrepared || event.StateChange() != nil || event.Reason() != nil {
			return fmt.Errorf("wrong access audit")
		}
		if len(s.adminClaimCapture.verified) != 1 || s.adminClaimCapture.verified[0] != event.ID() {
			return fmt.Errorf("persisted access was not observed before evidence signing")
		}
	}
	total, err := testsupport.OperationDetailAuditCorrelationCount(context.Background(), s.database, correlation)
	if err != nil {
		return err
	}
	if matches != 1 || total != 1 || len(s.adminClaimCapture.ids) != 1 {
		return fmt.Errorf("expected one event before one signing, matches=%d total=%d signing=%d", matches, total, len(s.adminClaimCapture.ids))
	}
	return nil
}
func (s *testSuite) adminClaimStatus(status string) error {
	if s.lastStatus != 200 {
		return fmt.Errorf("write returned %d: %s", s.lastStatus, s.lastBody)
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	f := s.claims.claims[s.adminClaims.selected]
	found, err := s.claimRepo().FindOwnedByID(context.Background(), f.claim.ClaimantID, f.claim.ID)
	if err != nil {
		return err
	}
	if string(raw["id"]) != strconv.Itoa(found.ID) || string(raw["status"]) != strconv.Quote(status) || string(found.Status) != status {
		return fmt.Errorf("response and persisted state differ")
	}
	return nil
}
func (s *testSuite) adminClaimActionMatches(instant string) error {
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	var action struct {
		ID         int
		Type       string
		ActorID    int       `json:"actor_id"`
		ActorParty string    `json:"actor_party"`
		CreatedOn  time.Time `json:"created_on"`
	}
	if err := json.Unmarshal(raw["action"], &action); err != nil {
		return err
	}
	operator, err := s.userRepository.FindOperatorIDByAuthID(context.Background(), s.currentAuth0ID)
	if err != nil {
		return err
	}
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	if action.ID <= 0 || action.ActorID != operator || action.ActorParty != "operator" || !action.CreatedOn.Equal(now) {
		return fmt.Errorf("operator or server action date differs")
	}
	f := s.claims.claims[s.adminClaims.selected]
	found, err := s.claimRepo().FindOwnedByID(context.Background(), f.claim.ClaimantID, f.claim.ID)
	if err != nil {
		return err
	}
	for _, a := range found.Actions {
		if a.ID == action.ID {
			if a.ActorID != action.ActorID || string(a.ActorParty) != action.ActorParty || !a.CreatedOn.Equal(action.CreatedOn) || a.Type != action.Type {
				return fmt.Errorf("response action differs from persistence")
			}
			return nil
		}
	}
	return fmt.Errorf("response action not persisted")
}
func (s *testSuite) adminClaimActionCount(kind string, count int) error {
	f := s.claims.claims[s.adminClaims.selected]
	found, err := s.claimRepo().FindOwnedByID(context.Background(), f.claim.ClaimantID, f.claim.ID)
	if err != nil {
		return err
	}
	actual := 0
	for _, a := range found.Actions {
		if a.Type == kind {
			actual++
		}
	}
	if actual != count {
		return fmt.Errorf("expected %d %s actions, got %d", count, kind, actual)
	}
	return nil
}
func (s *testSuite) adminClaimExecutionAudit(label, email, instant, correlation string) error {
	events, err := s.adminClaimPersistedEvents(label)
	if err != nil {
		return err
	}
	operator, err := s.userRepository.FindOperatorIDByAuthID(context.Background(), auth0IDForAdminEmail(email))
	if err != nil {
		return err
	}
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	matches := 0
	for _, event := range events {
		if event.CorrelationID() != correlation {
			continue
		}
		matches++
		if event.Action() != audit.ActionExecute || event.Result() != audit.ResultSucceeded || event.OperatorID() != operator || !event.OccurredOn().Equal(now) {
			return fmt.Errorf("execution audit differs")
		}
		persisted, err := s.auditEvents.FindByID(context.Background(), event.ID())
		if err != nil {
			return err
		}
		if persisted == nil {
			return fmt.Errorf("execution audit not persisted")
		}
		s.adminClaims.event = persisted
	}
	if matches != 1 {
		correlations := make([]string, 0, len(events))
		for _, event := range events {
			correlations = append(correlations, event.CorrelationID())
		}
		return fmt.Errorf("expected one execution audit for %q, got %d (persisted correlations %v)", correlation, matches, correlations)
	}
	return nil
}
func (s *testSuite) adminClaimAuditTransition(from, to string) error {
	event := s.adminClaims.event
	if event == nil || event.StateChange() == nil || event.StateChange().Field() != "status" || event.StateChange().From() != from || event.StateChange().To() != to || event.Reason() != nil || event.ConversationID() != nil {
		return fmt.Errorf("audit must contain only state transition")
	}
	return nil
}
func (s *testSuite) adminClaimReplayAction() error {
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	var actual, expected any
	if err := json.Unmarshal(raw["action"], &actual); err != nil {
		return err
	}
	if err := json.Unmarshal(s.adminClaims.originalAction, &expected); err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("original action was not recovered")
	}
	return nil
}
func (s *testSuite) adminClaimReplayResolution() error {
	if err := s.adminClaimReplayAction(); err != nil {
		return err
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	var actual, expected any
	if err := json.Unmarshal(raw["resolution"], &actual); err != nil {
		return err
	}
	if err := json.Unmarshal(s.adminClaims.originalResolution, &expected); err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("original resolution was not recovered")
	}
	return nil
}
func (s *testSuite) adminClaimReplayUnchanged() error {
	if err := s.claimUnchanged(s.adminClaims.selected); err != nil {
		return err
	}
	return s.adminClaimAuditUnchanged()
}
func (s *testSuite) adminClaimResolutionStored(instant string) error {
	f := s.claims.claims[s.adminClaims.selected]
	found, err := s.claimRepo().FindOwnedByID(context.Background(), f.claim.ClaimantID, f.claim.ID)
	if err != nil {
		return err
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	var resolution struct {
		Type, Reasoning string
		ResolvedOn      time.Time `json:"resolved_on"`
	}
	if err := json.Unmarshal(raw["resolution"], &resolution); err != nil {
		return err
	}
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	if found.Resolution == nil || resolution.Type != string(found.Resolution.Type) || resolution.Reasoning != found.Resolution.Reasoning || !resolution.ResolvedOn.Equal(found.Resolution.ResolvedOn) || !resolution.ResolvedOn.Equal(now) {
		return fmt.Errorf("resolution persistence differs")
	}
	if s.adminClaims.expectedPayload != nil && (resolution.Type != s.adminClaims.expectedPayload["type"] || resolution.Reasoning != s.adminClaims.expectedPayload["reasoning"]) {
		return fmt.Errorf("resolution differs from submitted input")
	}
	return s.adminClaimActionMatches(instant)
}
func (s *testSuite) adminClaimBusinessCounts() error {
	actual, err := testsupport.OperationDetailBusinessCounts(context.Background(), s.database)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, s.adminClaims.counts) {
		return fmt.Errorf("business counts changed")
	}
	return nil
}
func (s *testSuite) adminClaimBusinessUnchanged() error {
	before := s.adminClaims.operation
	if before == nil {
		return fmt.Errorf("operation snapshot missing")
	}
	var label string
	for name, f := range s.operationInbox.requests {
		if f.id == before.request.ID {
			label = name
			break
		}
	}
	after, err := s.captureOperationDetailAuditSnapshot(label)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before.request, after.request) || !reflect.DeepEqual(before.conversation, after.conversation) || !reflect.DeepEqual(before.proposals, after.proposals) {
		return fmt.Errorf("operation values changed")
	}
	return s.adminClaimBusinessCounts()
}
