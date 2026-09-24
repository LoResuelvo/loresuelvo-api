package steps_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type auditQueryState struct {
	fixtures       map[string]*audit.Event
	correlation    string
	rangeStart     string
	rangeEnd       string
	headers        http.Header
	noBearer       bool
	badBearer      bool
	firstPage      auditPageResponse
	secondPage     auditPageResponse
	pendingPostCut *audit.EventParams
	postCutID      uuid.UUID
	validCursor    string
}

type auditPageResponse struct {
	Events     []auditEventResponse `json:"events"`
	NextCursor *string              `json:"next_cursor"`
}

type auditEventResponse struct {
	ID            uuid.UUID `json:"id"`
	OperatorID    int       `json:"operator_id"`
	Action        string    `json:"action"`
	ResourceType  string    `json:"resource_type"`
	ResourceID    string    `json:"resource_id"`
	OccurredOn    time.Time `json:"occurred_on"`
	Result        string    `json:"result"`
	CorrelationID string    `json:"correlation_id"`
	Reason        *string   `json:"reason"`
}

func registerAdminQueryAuditLogsSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existen los siguientes eventos de auditoría preexistentes:$`, suite.thereAreExistingAuditEvents)
	sc.Step(`^que existe un evento de auditoría preexistente del operador "([^"]*)"$`, suite.thereIsExistingAuditEventForOperator)
	sc.Step(`^que existe un evento de auditoría preexistente$`, suite.thereIsExistingAuditEvent)
	sc.Step(`^consulto el registro de auditoría administrativa para el operador "([^"]*)"$`, suite.queryAuditForOperator)
	sc.Step(`^consulto el registro de auditoría administrativa para el operador "([^"]*)" con la correlación "([^"]*)"$`, suite.queryAuditForOperatorWithCorrelation)
	sc.Step(`^intento consultar el registro de auditoría administrativa$`, suite.attemptAuditQuery)
	sc.Step(`^que no envío un token Bearer$`, suite.doNotSendBearerForAuditQuery)
	sc.Step(`^que envío un token Bearer inválido$`, suite.sendInvalidBearerForAuditQuery)
	sc.Step(`^la página contiene los eventos "([^"]*)" y "([^"]*)" en ese orden$`, suite.auditPageContainsTwoEventsInOrder)
	sc.Step(`^cada evento expone exactamente su identificador, referencia interna del operador, acción, tipo e ID de recurso, fecha UTC, resultado, correlación y motivo sólo cuando existe$`, suite.auditEventsExposeAllowedFields)
	sc.Step(`^el evento "([^"]*)" informa el motivo "([^"]*)" y el evento "([^"]*)" no informa motivo$`, suite.auditEventReasonsMatch)
	sc.Step(`^la respuesta no expone credenciales, identificadores externos de autenticación, cuerpos HTTP, mensajes de chat, biometría ni URLs firmadas$`, suite.auditResponseHasNoPrivateData)
	sc.Step(`^la respuesta incluye la cabecera "([^"]*)" con valor "([^"]*)"$`, suite.auditResponseHeaderEquals)
	sc.Step(`^los eventos "([^"]*)" y "([^"]*)" permanecen sin modificaciones$`, suite.auditEventsRemainUnchanged)
	sc.Step(`^la página contiene una colección vacía, no nula, y no tiene cursor siguiente$`, suite.auditPageIsEmpty)
	sc.Step(`^la respuesta no contiene eventos de auditoría$`, suite.auditErrorResponseHasNoEvents)
	sc.Step(`^el sistema responde con estado 200 y una colección vacía no nula$`, suite.auditQueryRespondsWithEmptyCollection)
	sc.Step(`^queda registrado exactamente un evento de acceso preparado a la colección de auditoría por "([^"]*)"$`, suite.onePreparedAuditAccessIsRecorded)
	sc.Step(`^ese evento contiene la correlación "([^"]*)" y no requiere motivo manual$`, suite.preparedAuditAccessHasCorrelationAndNoReason)
	sc.Step(`^el evento de esta consulta no aparece en la colección devuelta$`, suite.currentAuditAccessIsNotReturned)
	sc.Step(`^filtro el registro por el operador "([^"]*)", la acción "([^"]*)", el recurso "([^"]*)" con ID "([^"]*)", el resultado "([^"]*)" y el rango desde "([^"]*)" hasta "([^"]*)"$`, suite.filterAuditEvents)
	sc.Step(`^la página contiene solamente el evento "([^"]*)"$`, suite.auditPageContainsOnlyEvent)
	sc.Step(`^el inicio del rango es inclusivo y el fin es exclusivo$`, suite.auditRangeHasExpectedBounds)
	sc.Step(`^consulto el registro desde "([^"]*)" hasta "([^"]*)"$`, suite.queryAuditWithinRange)
	sc.Step(`^que se agregará un evento sintético del operador "([^"]*)" con acción "([^"]*)", recurso "([^"]*)" e ID "([^"]*)", resultado "([^"]*)" y correlación única después de la primera página$`, suite.schedulePostCutAuditEvent)
	sc.Step(`^recorro el registro para el operador "([^"]*)" y la acción "([^"]*)" con páginas de (\d+) eventos$`, suite.walkAuditPages)
	sc.Step(`^la primera página contiene los eventos "([^"]*)" y "([^"]*)" en ese orden y entrega un cursor siguiente$`, suite.firstAuditPageContainsEvents)
	sc.Step(`^la segunda página contiene los eventos "([^"]*)" y "([^"]*)" en ese orden y no tiene cursor siguiente$`, suite.secondAuditPageContainsEvents)
	sc.Step(`^ningún evento se repite ni aparecen el evento "([^"]*)" o el evento posterior al corte$`, suite.auditPagesExcludeUnwantedEvents)
	sc.Step(`^que existen (\d+) eventos sintéticos preexistentes del operador "([^"]*)" con acción "([^"]*)", recurso "([^"]*)" e ID "([^"]*)", resultado "([^"]*)", fechas anteriores a la consulta e identificadores y correlaciones únicos$`, suite.thereAreSyntheticAuditEvents)
	sc.Step(`^consulto el registro para el operador "([^"]*)" sin indicar límite$`, suite.queryAuditWithDefaultLimit)
	sc.Step(`^consulto el registro para el operador "([^"]*)" con límite (\d+)$`, suite.queryAuditWithExplicitLimit)
	sc.Step(`^la página contiene (\d+) eventos y entrega un cursor siguiente$`, suite.auditPageHasCountAndCursor)
	sc.Step(`^consulto el registro de auditoría con el parámetro "([^"]*)" igual a "([^"]*)"$`, suite.queryAuditWithInvalidParameter)
	sc.Step(`^que existen (\d+) eventos de auditoría preexistentes del operador "([^"]*)" con acción "([^"]*)"$`, suite.thereAreExistingAuditEventsForCursor)
	sc.Step(`^que obtuve un cursor válido al filtrar el registro por el operador "([^"]*)" y la acción "([^"]*)" con límite (\d+)$`, suite.obtainValidAuditCursor)
	sc.Step(`^consulto el registro con el cursor alterado, el mismo operador y la acción "([^"]*)"$`, suite.queryAuditWithTamperedCursor)
	sc.Step(`^consulto el registro con ese cursor, el mismo operador y la acción "([^"]*)"$`, suite.queryAuditWithUnchangedCursor)
}

func (suite *testSuite) auditOperatorID(email string) (int, error) {
	operatorID, err := suite.userRepository.FindOperatorIDByAuthID(suite.scenarioContext, auth0IDForAdminEmail(email))
	if err != nil {
		return 0, fmt.Errorf("finding audit operator %q: %w", email, err)
	}
	return operatorID, nil
}

func (suite *testSuite) thereAreExistingAuditEvents(table *godog.Table) error {
	headers := []string{"evento", "operador", "acción", "tipo de recurso", "ID de recurso", "fecha UTC", "resultado", "correlación"}
	orderedIDs := len(table.Rows) > 0 && len(table.Rows[0].Cells) == len(headers)+1 && table.Rows[0].Cells[8].Value == "orden de ID"
	if orderedIDs {
		headers[3] = "recurso"
	}
	withReason := !orderedIDs && len(table.Rows) > 0 && len(table.Rows[0].Cells) == len(headers)+1
	if withReason {
		headers = append(headers, "motivo")
	} else if orderedIDs {
		headers = append(headers, "orden de ID")
	}
	if err := requireTableHeaders(table, headers...); err != nil {
		return err
	}
	if len(table.Rows) < 2 {
		return fmt.Errorf("audit fixture table has no events")
	}
	suite.auditQuery.fixtures = make(map[string]*audit.Event, len(table.Rows)-1)
	orderedPrefix := uuid.New()
	usedRanks := make(map[uint32]bool)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != len(headers) {
			return fmt.Errorf("audit fixture row requires %d columns, got %d", len(headers), len(row.Cells))
		}
		cell := func(index int) string { return row.Cells[index].Value }
		if _, exists := suite.auditQuery.fixtures[cell(0)]; exists {
			return fmt.Errorf("duplicate audit fixture name %q", cell(0))
		}
		operatorID, err := suite.auditOperatorID(cell(1))
		if err != nil {
			return err
		}
		occurredOn, err := time.Parse(time.RFC3339Nano, cell(5))
		if err != nil {
			return fmt.Errorf("invalid audit fixture date %q: %w", cell(5), err)
		}
		var reason *audit.Reason
		if withReason && cell(8) != "" {
			reason, err = audit.NewReason(cell(8))
			if err != nil {
				return err
			}
		}
		id := uuid.New()
		if orderedIDs {
			rank, parseErr := strconv.ParseUint(cell(8), 10, 32)
			if parseErr != nil || rank == 0 || usedRanks[uint32(rank)] {
				return fmt.Errorf("invalid or duplicate audit fixture ID rank %q", cell(8))
			}
			usedRanks[uint32(rank)] = true
			id = orderedPrefix
			binary.BigEndian.PutUint32(id[12:], uint32(rank))
		}
		event, err := suite.auditEvents.Save(suite.scenarioContext, audit.EventParams{
			ID: id, OperatorID: operatorID, Action: audit.Action(cell(2)), ResourceType: cell(3),
			ResourceID: cell(4), OccurredOn: occurredOn, Result: audit.Result(cell(6)),
			CorrelationID: cell(7), Reason: reason,
		})
		if err != nil {
			return fmt.Errorf("saving audit fixture %q: %w", cell(0), err)
		}
		suite.auditQuery.fixtures[cell(0)] = event
	}
	return nil
}

func (suite *testSuite) thereIsExistingAuditEventForOperator(email string) error {
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return err
	}
	_, err = suite.auditEvents.Save(suite.scenarioContext, audit.EventParams{
		ID: uuid.New(), OperatorID: operatorID, Action: audit.ActionCreate,
		ResourceType: "category", ResourceID: "17", OccurredOn: time.Now().UTC().Add(-time.Hour),
		Result: audit.ResultSucceeded, CorrelationID: "fixture-" + uuid.NewString(),
	})
	return err
}

func (suite *testSuite) thereIsExistingAuditEvent() error {
	return suite.thereIsExistingAuditEventForOperator("supervisor@example.com")
}

func (suite *testSuite) queryAuditForOperator(email string) error {
	return suite.queryAuditForOperatorWithCorrelation(email, "")
}

func (suite *testSuite) queryAuditForOperatorWithCorrelation(email, correlation string) error {
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return err
	}
	query := url.Values{"operator_id": {strconv.Itoa(operatorID)}}
	return suite.sendAuditQuery(query, correlation)
}

func (suite *testSuite) filterAuditEvents(email, action, resourceType, resourceID, result, from, to string) error {
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return err
	}
	suite.auditQuery.rangeStart = from
	suite.auditQuery.rangeEnd = to
	return suite.sendAuditQuery(url.Values{
		"operator_id":   {strconv.Itoa(operatorID)},
		"action":        {action},
		"resource_type": {resourceType},
		"resource_id":   {resourceID},
		"result":        {result},
		"occurred_from": {from},
		"occurred_to":   {to},
	}, "")
}

func (suite *testSuite) queryAuditWithinRange(from, to string) error {
	return suite.sendAuditQuery(url.Values{"occurred_from": {from}, "occurred_to": {to}}, "")
}

func (suite *testSuite) attemptAuditQuery() error {
	return suite.sendAuditQuery(nil, "")
}

func (suite *testSuite) doNotSendBearerForAuditQuery() error {
	suite.auditQuery.noBearer = true
	return nil
}

func (suite *testSuite) sendInvalidBearerForAuditQuery() error {
	suite.auditQuery.badBearer = true
	return nil
}

func (suite *testSuite) sendAuditQuery(query url.Values, correlation string) error {
	path := "/admin/audit-logs"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	request, err := http.NewRequest(http.MethodGet, suite.server.URL+path, nil)
	if err != nil {
		return fmt.Errorf("building audit query request: %w", err)
	}
	if !suite.auditQuery.noBearer {
		if suite.auditQuery.badBearer {
			request.Header.Set("Authorization", "Bearer invalid-token")
		} else {
			request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, suite.currentPermissions))
		}
	}
	if correlation != "" {
		request.Header.Set("X-Request-ID", correlation)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("requesting administrative audit events: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading administrative audit response: %w", err)
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = body
	suite.auditQuery.headers = response.Header.Clone()
	suite.auditQuery.correlation = correlation
	return nil
}

func (suite *testSuite) decodedAuditPage() (auditPageResponse, []map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return auditPageResponse{}, nil, fmt.Errorf("invalid audit page JSON: %w", err)
	}
	if err := requireExactJSONFields(raw, "audit page", "events", "next_cursor"); err != nil {
		return auditPageResponse{}, nil, err
	}
	var page auditPageResponse
	if err := json.Unmarshal(suite.lastBody, &page); err != nil {
		return auditPageResponse{}, nil, fmt.Errorf("invalid audit page: %w", err)
	}
	var events []map[string]json.RawMessage
	if err := json.Unmarshal(raw["events"], &events); err != nil || events == nil {
		return auditPageResponse{}, nil, fmt.Errorf("audit events must be a non-null array: %w", err)
	}
	if len(page.Events) != len(events) {
		return auditPageResponse{}, nil, fmt.Errorf("decoded audit event count mismatch")
	}
	return page, events, nil
}

func (suite *testSuite) auditPageContainsTwoEventsInOrder(first, second string) error {
	page, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	if len(page.Events) != 2 {
		return fmt.Errorf("expected exactly two events, got %d", len(page.Events))
	}
	for index, name := range []string{first, second} {
		expected, ok := suite.auditQuery.fixtures[name]
		if !ok || page.Events[index].ID != expected.ID() {
			return fmt.Errorf("event at index %d is not fixture %q", index, name)
		}
	}
	return nil
}

func (suite *testSuite) auditPageContainsOnlyEvent(name string) error {
	page, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	expected := suite.auditQuery.fixtures[name]
	if expected == nil {
		return fmt.Errorf("unknown audit fixture %q", name)
	}
	if len(page.Events) != 1 || page.Events[0].ID != expected.ID() || page.NextCursor != nil {
		return fmt.Errorf("expected only fixture %q with no next cursor; got %d events, cursor present: %t", name, len(page.Events), page.NextCursor != nil)
	}
	return nil
}

func (suite *testSuite) auditRangeHasExpectedBounds() error {
	from, err := time.Parse(time.RFC3339Nano, suite.auditQuery.rangeStart)
	if err != nil {
		return fmt.Errorf("invalid lower range bound: %w", err)
	}
	to, err := time.Parse(time.RFC3339Nano, suite.auditQuery.rangeEnd)
	if err != nil {
		return fmt.Errorf("invalid upper range bound: %w", err)
	}
	start := suite.auditQuery.fixtures["A"]
	end := suite.auditQuery.fixtures["G"]
	if start == nil || end == nil || !start.OccurredOn().Equal(from) || !end.OccurredOn().Equal(to) {
		return fmt.Errorf("audit fixture dates do not exercise both range boundaries")
	}
	return suite.auditPageContainsOnlyEvent("A")
}

func (suite *testSuite) auditEventsExposeAllowedFields() error {
	page, rawEvents, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	if len(page.Events) == 0 {
		return fmt.Errorf("expected audit events to inspect")
	}
	for index, found := range page.Events {
		var expected *audit.Event
		for _, fixture := range suite.auditQuery.fixtures {
			if fixture.ID() == found.ID {
				expected = fixture
				break
			}
		}
		if expected == nil {
			return fmt.Errorf("unknown audit event %s", found.ID)
		}
		fields := []string{"id", "operator_id", "action", "resource_type", "resource_id", "occurred_on", "result", "correlation_id"}
		if expected.Reason() != nil {
			fields = append(fields, "reason")
		}
		if err := requireExactJSONFields(rawEvents[index], "audit event", fields...); err != nil {
			return err
		}
		if found.OperatorID != expected.OperatorID() || found.Action != string(expected.Action()) ||
			found.ResourceType != expected.ResourceType() || found.ResourceID != expected.ResourceID() ||
			!found.OccurredOn.Equal(expected.OccurredOn()) || found.OccurredOn.Location() != time.UTC ||
			found.Result != string(expected.Result()) || found.CorrelationID != expected.CorrelationID() {
			return fmt.Errorf("audit event %s differs from its immutable fixture", found.ID)
		}
		if expected.Reason() != nil && (found.Reason == nil || *found.Reason != expected.Reason().Text()) {
			return fmt.Errorf("audit event %s has wrong reason", found.ID)
		}
	}
	return nil
}

func (suite *testSuite) auditEventReasonsMatch(withReason, reason, withoutReason string) error {
	page, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	ids := map[uuid.UUID]*string{}
	for _, event := range page.Events {
		ids[event.ID] = event.Reason
	}
	with := suite.auditQuery.fixtures[withReason]
	without := suite.auditQuery.fixtures[withoutReason]
	if with == nil || without == nil || ids[with.ID()] == nil || *ids[with.ID()] != reason {
		return fmt.Errorf("event %q does not report its expected reason", withReason)
	}
	if _, present := ids[without.ID()]; !present || ids[without.ID()] != nil {
		return fmt.Errorf("event %q unexpectedly reports a reason", withoutReason)
	}
	return nil
}

func (suite *testSuite) auditResponseHasNoPrivateData() error {
	// The allowlist assertion covers every event key; the page itself is also exact.
	return suite.auditEventsExposeAllowedFields()
}

func (suite *testSuite) auditResponseHeaderEquals(name, value string) error {
	if got := suite.auditQuery.headers.Get(name); got != value {
		return fmt.Errorf("expected %s header %q, got %q", name, value, got)
	}
	return nil
}

func (suite *testSuite) auditEventsRemainUnchanged(first, second string) error {
	for _, name := range []string{first, second} {
		expected := suite.auditQuery.fixtures[name]
		if expected == nil {
			return fmt.Errorf("unknown audit fixture %q", name)
		}
		persisted, err := suite.auditEvents.FindByID(suite.scenarioContext, expected.ID())
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(persisted, expected) {
			return fmt.Errorf("audit fixture %q was modified", name)
		}
	}
	return nil
}

func (suite *testSuite) auditPageIsEmpty() error {
	page, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	if len(page.Events) != 0 || page.NextCursor != nil {
		return fmt.Errorf("expected empty audit page and no cursor; got %d events, cursor present: %t", len(page.Events), page.NextCursor != nil)
	}
	return nil
}

func (suite *testSuite) auditErrorResponseHasNoEvents() error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return fmt.Errorf("invalid audit error response JSON: %w", err)
	}
	if _, exists := raw["error"]; !exists {
		return fmt.Errorf("audit error response is missing its error field")
	}
	for field := range raw {
		if field != "error" && field != "message" {
			return fmt.Errorf("audit error response has unexpected field %q", field)
		}
	}
	return nil
}

func (suite *testSuite) auditQueryRespondsWithEmptyCollection() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	return suite.auditPageIsEmpty()
}

func (suite *testSuite) preparedAccessEventsFor(email string) ([]*audit.Event, error) {
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return nil, err
	}
	reader := suite.dependencies.Persistence.AuditEventRepository
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return nil, fmt.Errorf("capturing audit ingest watermark: %w", err)
	}
	latest, err := reader.FindPage(suite.scenarioContext, audit.LogFilter{OperatorID: &operatorID}, watermark, nil, 20)
	if err != nil {
		return nil, fmt.Errorf("listing latest audit events for operator: %w", err)
	}
	var found []*audit.Event
	for _, event := range latest {
		if event.OperatorID() == operatorID && event.CorrelationID() == suite.auditQuery.correlation &&
			event.Action() == audit.ActionAccess && event.Result() == audit.ResultPrepared &&
			event.ResourceType() == "audit_log" && event.ResourceID() == "" {
			found = append(found, event)
		}
	}
	return found, nil
}

func (suite *testSuite) onePreparedAuditAccessIsRecorded(email string) error {
	accesses, err := suite.preparedAccessEventsFor(email)
	if err != nil {
		return err
	}
	if len(accesses) != 1 {
		return fmt.Errorf("expected exactly one prepared audit access, got %d", len(accesses))
	}
	return nil
}

func (suite *testSuite) preparedAuditAccessHasCorrelationAndNoReason(correlation string) error {
	accesses, err := suite.preparedAccessEventsFor("supervisor@example.com")
	if err != nil {
		return err
	}
	if len(accesses) != 1 || accesses[0].CorrelationID() != correlation || accesses[0].Reason() != nil {
		return fmt.Errorf("prepared audit access has wrong correlation or reason")
	}
	return nil
}

func (suite *testSuite) currentAuditAccessIsNotReturned() error {
	page, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	accesses, err := suite.preparedAuditAccessEventsForCurrentOperator()
	if err != nil {
		return err
	}
	if len(accesses) != 1 {
		return fmt.Errorf("expected one prepared access event, got %d", len(accesses))
	}
	for _, event := range page.Events {
		if event.ID == accesses[0].ID() {
			return fmt.Errorf("query includes its own access event")
		}
	}
	return nil
}

func (suite *testSuite) preparedAuditAccessEventsForCurrentOperator() ([]*audit.Event, error) {
	return suite.preparedAccessEventsFor("supervisor@example.com")
}

func (suite *testSuite) schedulePostCutAuditEvent(email, action, resourceType, resourceID, result string) error {
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return err
	}
	params := audit.EventParams{
		ID: uuid.New(), OperatorID: operatorID, Action: audit.Action(action),
		ResourceType: resourceType, ResourceID: resourceID,
		OccurredOn: time.Date(2026, 9, 20, 10, 30, 0, 0, time.UTC),
		Result:     audit.Result(result), CorrelationID: "post-cut-" + uuid.NewString(),
	}
	suite.auditQuery.pendingPostCut = &params
	return nil
}

func (suite *testSuite) walkAuditPages(email, action string, limit int) error {
	if suite.auditQuery.pendingPostCut == nil {
		return fmt.Errorf("post-cut audit fixture was not scheduled")
	}
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return err
	}
	query := url.Values{"operator_id": {strconv.Itoa(operatorID)}, "action": {action}, "limit": {strconv.Itoa(limit)}}
	if err := suite.sendAuditQuery(query, ""); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("first audit page returned status %d", suite.lastStatus)
	}
	first, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	if first.NextCursor == nil || *first.NextCursor == "" {
		return fmt.Errorf("first audit page has no next cursor")
	}
	suite.auditQuery.firstPage = first
	inserted, err := suite.auditEvents.Save(suite.scenarioContext, *suite.auditQuery.pendingPostCut)
	if err != nil {
		return fmt.Errorf("saving post-cut audit fixture: %w", err)
	}
	suite.auditQuery.postCutID = inserted.ID()
	query.Set("cursor", *first.NextCursor)
	if err := suite.sendAuditQuery(query, ""); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("second audit page returned status %d", suite.lastStatus)
	}
	second, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	suite.auditQuery.secondPage = second
	return nil
}

func (suite *testSuite) auditPageMatchesFixtures(page auditPageResponse, names ...string) error {
	if len(page.Events) != len(names) {
		return fmt.Errorf("expected %d audit events, got %d", len(names), len(page.Events))
	}
	for i, name := range names {
		expected := suite.auditQuery.fixtures[name]
		if expected == nil {
			return fmt.Errorf("unknown audit fixture %q", name)
		}
		if page.Events[i].ID != expected.ID() {
			return fmt.Errorf("audit event at position %d is not fixture %q", i, name)
		}
	}
	return nil
}

func (suite *testSuite) firstAuditPageContainsEvents(first, second string) error {
	if err := suite.auditPageMatchesFixtures(suite.auditQuery.firstPage, first, second); err != nil {
		return err
	}
	if suite.auditQuery.firstPage.NextCursor == nil || *suite.auditQuery.firstPage.NextCursor == "" {
		return fmt.Errorf("first audit page has no next cursor")
	}
	return nil
}

func (suite *testSuite) secondAuditPageContainsEvents(first, second string) error {
	if err := suite.auditPageMatchesFixtures(suite.auditQuery.secondPage, first, second); err != nil {
		return err
	}
	if suite.auditQuery.secondPage.NextCursor != nil {
		return fmt.Errorf("second audit page unexpectedly has a next cursor")
	}
	return nil
}

func (suite *testSuite) auditPagesExcludeUnwantedEvents(name string) error {
	unwanted := suite.auditQuery.fixtures[name]
	if unwanted == nil || suite.auditQuery.postCutID == uuid.Nil {
		return fmt.Errorf("missing excluded audit fixtures")
	}
	seen := make(map[uuid.UUID]bool)
	for _, page := range []auditPageResponse{suite.auditQuery.firstPage, suite.auditQuery.secondPage} {
		for _, event := range page.Events {
			if seen[event.ID] || event.ID == unwanted.ID() || event.ID == suite.auditQuery.postCutID {
				return fmt.Errorf("audit pages contain a duplicate or excluded event")
			}
			seen[event.ID] = true
		}
	}
	return nil
}

func (suite *testSuite) thereAreSyntheticAuditEvents(count int, email, action, resourceType, resourceID, result string) error {
	if count < 1 || count > 101 {
		return fmt.Errorf("synthetic audit fixture count is out of bounds")
	}
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return err
	}
	base := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	for i := range count {
		_, err := suite.auditEvents.Save(suite.scenarioContext, audit.EventParams{
			ID: uuid.New(), OperatorID: operatorID, Action: audit.Action(action),
			ResourceType: resourceType, ResourceID: resourceID,
			OccurredOn: base.Add(-time.Duration(i) * time.Second),
			Result:     audit.Result(result), CorrelationID: "synthetic-" + uuid.NewString(),
		})
		if err != nil {
			return fmt.Errorf("saving synthetic audit fixture %d: %w", i, err)
		}
	}
	return nil
}

func (suite *testSuite) queryAuditWithDefaultLimit(email string) error {
	return suite.queryAuditWithLimit(email, 0)
}

func (suite *testSuite) queryAuditWithExplicitLimit(email string, limit int) error {
	return suite.queryAuditWithLimit(email, limit)
}

func (suite *testSuite) queryAuditWithLimit(email string, limit int) error {
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return err
	}
	query := url.Values{"operator_id": {strconv.Itoa(operatorID)}}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	return suite.sendAuditQuery(query, "")
}

func (suite *testSuite) auditPageHasCountAndCursor(count int) error {
	page, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	if len(page.Events) != count || page.NextCursor == nil || *page.NextCursor == "" {
		return fmt.Errorf("expected %d audit events and a next cursor, got %d events, cursor present: %t", count, len(page.Events), page.NextCursor != nil)
	}
	return nil
}

func (suite *testSuite) queryAuditWithInvalidParameter(name, value string) error {
	return suite.sendAuditQuery(url.Values{name: {value}}, "")
}

func (suite *testSuite) thereAreExistingAuditEventsForCursor(count int, email, action string) error {
	return suite.thereAreSyntheticAuditEvents(count, email, action, "category", "17", string(audit.ResultSucceeded))
}

func (suite *testSuite) obtainValidAuditCursor(email, action string, limit int) error {
	operatorID, err := suite.auditOperatorID(email)
	if err != nil {
		return err
	}
	if err := suite.sendAuditQuery(url.Values{
		"operator_id": {strconv.Itoa(operatorID)}, "action": {action}, "limit": {strconv.Itoa(limit)},
	}, ""); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("obtaining audit cursor returned status %d", suite.lastStatus)
	}
	page, _, err := suite.decodedAuditPage()
	if err != nil {
		return err
	}
	if page.NextCursor == nil || *page.NextCursor == "" {
		return fmt.Errorf("audit query did not return a valid cursor")
	}
	suite.auditQuery.validCursor = *page.NextCursor
	return nil
}

func (suite *testSuite) queryAuditWithTamperedCursor(action string) error {
	return suite.queryAuditWithCursor(action, true)
}

func (suite *testSuite) queryAuditWithUnchangedCursor(action string) error {
	return suite.queryAuditWithCursor(action, false)
}

func (suite *testSuite) queryAuditWithCursor(action string, tamper bool) error {
	operatorID, err := suite.auditOperatorID("supervisor@example.com")
	if err != nil {
		return err
	}
	cursor := suite.auditQuery.validCursor
	if cursor == "" {
		return fmt.Errorf("no valid audit cursor was obtained")
	}
	if tamper {
		index := len(cursor) / 2
		if index+1 < len(cursor) {
			index++
		}
		replacement := byte('A')
		if cursor[index] == replacement {
			replacement = 'B'
		}
		cursor = cursor[:index] + string(replacement) + cursor[index+1:]
	}
	return suite.sendAuditQuery(url.Values{
		"operator_id": {strconv.Itoa(operatorID)}, "action": {action}, "limit": {"1"}, "cursor": {cursor},
	}, "")
}
