package steps_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

// quotedLabelList matches `"A"`, `"A" y "B"` and `"A", "B" y "C"`.
const quotedLabelList = `"[^"]*"(?:, "[^"]*")*(?: y "[^"]*")?`

var quotedLabelPattern = regexp.MustCompile(`"([^"]*)"`)

func registerAdminOperationsInboxAlertSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existen las siguientes órdenes de trabajo con su fecha programada:$`, suite.thereAreInboxScheduledWorkOrders)
	sc.Step(`^que la conversación de "([^"]*)" tiene un mensaje enviado el "([^"]*)"$`, suite.inboxConversationHasMessageSentOn)
	sc.Step(`^filtro la bandeja por la alerta "([^"]*)"$`, suite.filterInboxByAlert)
	sc.Step(`^la bandeja contiene solamente (?:la operación|las operaciones) (`+quotedLabelList+`)$`, suite.inboxContainsOnlyOperations)
	sc.Step(`^la bandeja contiene solamente (?:la operación de la orden|las operaciones de las órdenes) (`+quotedLabelList+`)$`, suite.inboxContainsOnlyOperationsOfOrders)
	sc.Step(`^la operación (?:de la orden )?"([^"]*)" informa la alerta "([^"]*)"$`, suite.inboxOperationHasAlert)
	sc.Step(`^la operación (?:de la orden )?"([^"]*)" no informa la alerta "([^"]*)"$`, suite.inboxOperationLacksAlert)
	sc.Step(`^la operación (?:de la orden )?"([^"]*)" informa el estado de dominio "([^"]*)" de la (solicitud|propuesta|orden)$`, suite.inboxOperationHasDomainStatus)
	sc.Step(`^la operación "([^"]*)" informa como último avance de negocio "([^"]*)"$`, suite.inboxOperationHasLastBusinessAdvance)
	sc.Step(`^la operación "([^"]*)" informa explícitamente como nulo su último avance de negocio$`, suite.inboxOperationHasNullLastBusinessAdvance)
	sc.Step(`^la operación "([^"]*)" informa la limitación "([^"]*)"$`, suite.inboxOperationHasLimitation)
}

// thereAreInboxScheduledWorkOrders builds each order's request and proposal
// ahead of its schedule and reports completion at its expected end.
func (suite *testSuite) thereAreInboxScheduledWorkOrders(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	suite.operationInbox.ensureMaps()
	now := suite.clock.Now()
	return suite.withInboxFixtureClock(func() error {
		for _, row := range rows {
			label := row["orden"]
			scheduledOn, err := parseInboxInstant(row["fecha programada"])
			if err != nil {
				return err
			}
			duration, err := strconv.Atoi(row["duración"])
			if err != nil {
				return fmt.Errorf("parsing duration of %q: %w", label, err)
			}
			requestLabel, proposalLabel := "S-"+label, "P-"+label
			if err := suite.createInboxJobRequest(requestLabel, row["consumidor"], row["prestador"], scheduledOn.Add(-120*time.Hour), "accepted"); err != nil {
				return err
			}
			if err := suite.createInboxServiceProposal(proposalLabel, requestLabel, scheduledOn.Add(-96*time.Hour), scheduledOn, duration, string(serviceproposal.StatusAccepted)); err != nil {
				return err
			}
			reportedOn := scheduledOn.Add(time.Duration(duration) * time.Minute)
			if row["estado"] != string(workorder.StatusScheduled) && reportedOn.After(now) {
				return fmt.Errorf("work order %q would report completion after the current time", label)
			}
			var reported, paid string
			switch row["estado"] {
			case string(workorder.StatusScheduled):
			case string(workorder.StatusAwaitingPayment):
				reported = reportedOn.Format(time.RFC3339)
			case string(workorder.StatusPaid):
				paidOn := reportedOn.Add(time.Hour)
				if paidOn.After(now) {
					return fmt.Errorf("work order %q would pay its balance after the current time", label)
				}
				reported, paid = reportedOn.Format(time.RFC3339), paidOn.Format(time.RFC3339)
			default:
				return fmt.Errorf("unsupported work order status %q", row["estado"])
			}
			if err := suite.createInboxWorkOrder(label, proposalLabel, scheduledOn.Add(-72*time.Hour), row["estado"], reported, paid); err != nil {
				return err
			}
		}
		return nil
	})
}

func (suite *testSuite) inboxConversationHasMessageSentOn(requestLabel, sentOn string) error {
	request, exists := suite.operationInbox.requests[requestLabel]
	if !exists {
		return fmt.Errorf("unknown job request label %q", requestLabel)
	}
	instant, err := parseInboxInstant(sentOn)
	if err != nil {
		return err
	}
	return suite.withInboxFixtureClock(func() error {
		if err := suite.setInboxFixtureClock(instant); err != nil {
			return err
		}
		return suite.sendInboxConversationMessage(requestLabel, auth0IDForConsumerEmail(request.consumerEmail), sendMessageRequest{Content: inboxPrivateMessage})
	})
}

func (suite *testSuite) filterInboxByAlert(alert string) error {
	return suite.queryOperationsInboxWith(url.Values{"alert": {alert}})
}

func quotedLabels(list string) []string {
	matches := quotedLabelPattern.FindAllStringSubmatch(list, -1)
	labels := make([]string, 0, len(matches))
	for _, match := range matches {
		labels = append(labels, match[1])
	}
	return labels
}

func (suite *testSuite) inboxContainsOnlyOperations(list string) error {
	expected := []string{}
	for _, label := range quotedLabels(list) {
		operationID, err := suite.inboxOperationID(label)
		if err != nil {
			return err
		}
		expected = append(expected, operationID)
	}
	return suite.inboxHasExactlyOperationIDs(expected, func(found inboxOperationResponse) string { return found.ID })
}

func (suite *testSuite) inboxContainsOnlyOperationsOfOrders(list string) error {
	expected := []string{}
	for _, label := range quotedLabels(list) {
		orderID, exists := suite.operationInbox.orders[label]
		if !exists {
			return fmt.Errorf("unknown work order label %q", label)
		}
		expected = append(expected, strconv.Itoa(orderID))
	}
	return suite.inboxHasExactlyOperationIDs(expected, func(found inboxOperationResponse) string {
		if found.WorkOrder == nil {
			return "operation " + found.ID + " without work order"
		}
		return strconv.Itoa(found.WorkOrder.ID)
	})
}

func (suite *testSuite) inboxHasExactlyOperationIDs(expected []string, key func(inboxOperationResponse) string) error {
	page, err := suite.decodedInboxPage()
	if err != nil {
		return err
	}
	actual := make([]string, 0, len(page.Operations))
	for _, found := range page.Operations {
		actual = append(actual, key(found))
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if strings.Join(expected, ",") != strings.Join(actual, ",") {
		return fmt.Errorf("expected only %v, got %v", expected, actual)
	}
	return nil
}

func (suite *testSuite) inboxOperationAlerts(label string) ([]string, error) {
	found, err := suite.inboxOperation(label)
	if err != nil {
		return nil, err
	}
	if found.Alerts == nil {
		return nil, fmt.Errorf("operation %q omits its alerts", label)
	}
	return found.Alerts, nil
}

func (suite *testSuite) inboxOperationHasAlert(label, alert string) error {
	alerts, err := suite.inboxOperationAlerts(label)
	if err != nil {
		return err
	}
	for _, found := range alerts {
		if found == alert {
			return nil
		}
	}
	return fmt.Errorf("operation %q has alerts %v, missing %q", label, alerts, alert)
}

func (suite *testSuite) inboxOperationLacksAlert(label, alert string) error {
	alerts, err := suite.inboxOperationAlerts(label)
	if err != nil {
		return err
	}
	for _, found := range alerts {
		if found == alert {
			return fmt.Errorf("operation %q unexpectedly has alert %q", label, alert)
		}
	}
	return nil
}

func (suite *testSuite) inboxOperationHasDomainStatus(label, status, resource string) error {
	found, err := suite.inboxOperation(label)
	if err != nil {
		return err
	}
	actual := map[string]*inboxResourceResponse{
		"solicitud": found.JobRequest, "propuesta": found.ServiceProposal, "orden": found.WorkOrder,
	}[resource]
	if actual == nil || actual.Status != status {
		return fmt.Errorf("expected %s status %q, got %+v", resource, status, actual)
	}
	return nil
}

func (suite *testSuite) inboxOperationHasLastBusinessAdvance(label, expectedValue string) error {
	expected, err := parseInboxInstant(expectedValue)
	if err != nil {
		return err
	}
	found, err := suite.inboxOperation(label)
	if err != nil {
		return err
	}
	if found.LastBusinessAdvanceOn == nil || !found.LastBusinessAdvanceOn.Equal(expected) {
		return fmt.Errorf("expected last business advance %s, got %v", expected, found.LastBusinessAdvanceOn)
	}
	return nil
}

func (suite *testSuite) inboxOperationHasNullLastBusinessAdvance(label string) error {
	raw, err := suite.rawInboxOperation(label)
	if err != nil {
		return err
	}
	value, exists := raw["last_business_advance_on"]
	if !exists || string(value) != "null" {
		return fmt.Errorf("expected an explicit null last business advance, got %q (present: %t)", value, exists)
	}
	return nil
}

func (suite *testSuite) inboxOperationHasLimitation(label, limitation string) error {
	raw, err := suite.rawInboxOperation(label)
	if err != nil {
		return err
	}
	var limitations []string
	if err := json.Unmarshal(raw["limitations"], &limitations); err != nil {
		return fmt.Errorf("invalid limitations of %q: %w", label, err)
	}
	for _, found := range limitations {
		if found == limitation {
			return nil
		}
	}
	return fmt.Errorf("operation %q has limitations %v, missing %q", label, limitations, limitation)
}
