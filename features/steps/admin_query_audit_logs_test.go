package steps_test

import (
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
	fixtures    map[string]*audit.Event
	correlation string
	rangeStart  string
	rangeEnd    string
	headers     http.Header
	noBearer    bool
	badBearer   bool
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
	withReason := len(table.Rows) > 0 && len(table.Rows[0].Cells) == len(headers)+1
	if withReason {
		headers = append(headers, "motivo")
	}
	if err := requireTableHeaders(table, headers...); err != nil {
		return err
	}
	if len(table.Rows) < 2 {
		return fmt.Errorf("audit fixture table has no events")
	}
	suite.auditQuery.fixtures = make(map[string]*audit.Event, len(table.Rows)-1)
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
		event, err := suite.auditEvents.Save(suite.scenarioContext, audit.EventParams{
			ID: uuid.New(), OperatorID: operatorID, Action: audit.Action(cell(2)), ResourceType: cell(3),
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
	latest, err := suite.dependencies.Persistence.AuditEventRepository.FindLatest(suite.scenarioContext, audit.LogFilter{OperatorID: &operatorID}, 20)
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
