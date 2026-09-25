package steps_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

func (suite *testSuite) inboxRequestLabelFor(label string) (string, error) {
	if _, exists := suite.operationInbox.requests[label]; exists {
		return label, nil
	}
	if proposal, exists := suite.operationInbox.proposals[label]; exists {
		return proposal.requestLabel, nil
	}
	return "", fmt.Errorf("unknown operation label %q", label)
}

func (suite *testSuite) inboxOperation(label string) (inboxOperationResponse, error) {
	page, err := suite.decodedInboxPage()
	if err != nil {
		return inboxOperationResponse{}, err
	}
	if orderID, isOrder := suite.operationInbox.orders[label]; isOrder {
		for _, found := range page.Operations {
			if found.WorkOrder != nil && found.WorkOrder.ID == orderID {
				return found, nil
			}
		}
		return inboxOperationResponse{}, fmt.Errorf("the operation of work order %q is not in the inbox", label)
	}
	operationID, err := suite.inboxOperationID(label)
	if err != nil {
		return inboxOperationResponse{}, err
	}
	for _, found := range page.Operations {
		if found.ID == operationID {
			return found, nil
		}
	}
	return inboxOperationResponse{}, fmt.Errorf("operation %q (%s) is not in the inbox", label, operationID)
}

func (suite *testSuite) rawInboxOperation(label string) (map[string]json.RawMessage, error) {
	found, err := suite.inboxOperation(label)
	if err != nil {
		return nil, err
	}
	operationID := found.ID
	var page struct {
		Operations []map[string]json.RawMessage `json:"operations"`
	}
	if err := json.Unmarshal(suite.lastBody, &page); err != nil {
		return nil, fmt.Errorf("invalid inbox page: %w", err)
	}
	for _, found := range page.Operations {
		var id string
		if err := json.Unmarshal(found["id"], &id); err == nil && id == operationID {
			return found, nil
		}
	}
	return nil, fmt.Errorf("operation %q is not in the inbox", label)
}

func (suite *testSuite) sendInboxConversationMessage(requestLabel, senderAuthID string, payload sendMessageRequest) error {
	request, exists := suite.operationInbox.requests[requestLabel]
	if !exists {
		return fmt.Errorf("unknown job request label %q", requestLabel)
	}
	suite.currentAuth0ID = senderAuthID
	if err := suite.requestSendMessage(request.conversationID, payload); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("sending a message in %q returned %d: %s", requestLabel, suite.lastStatus, suite.lastBody)
	}
	return nil
}

func (suite *testSuite) inboxConversationHasImageAttachment(requestLabel string) error {
	request, exists := suite.operationInbox.requests[requestLabel]
	if !exists {
		return fmt.Errorf("unknown job request label %q", requestLabel)
	}
	return suite.withInboxFixtureClock(func() error {
		imageName := "adjunto-" + strings.ToLower(requestLabel) + ".jpg"
		if err := suite.consumerUploadedAndConfirmedMessageImage(request.consumerEmail, imageName); err != nil {
			return err
		}
		fileIDs, err := suite.messageImageFileIDs([]string{imageName})
		if err != nil {
			return err
		}
		return suite.sendInboxConversationMessage(requestLabel, auth0IDForConsumerEmail(request.consumerEmail),
			sendMessageRequest{Content: inboxPrivateMessage, ImageFileIDs: fileIDs})
	})
}

func (suite *testSuite) inboxConversationHasMessagesBetween(requestLabel, consumerEmail, providerEmail string) error {
	return suite.withInboxFixtureClock(func() error {
		if err := suite.sendInboxConversationMessage(requestLabel, auth0IDForConsumerEmail(consumerEmail), sendMessageRequest{Content: inboxPrivateMessage}); err != nil {
			return err
		}
		return suite.sendInboxConversationMessage(requestLabel, auth0IDForProviderEmail(providerEmail), sendMessageRequest{Content: inboxPrivateMessage})
	})
}

func (suite *testSuite) inboxOperationInformsParties(label, consumerName, providerName string) error {
	found, err := suite.inboxOperation(label)
	if err != nil {
		return err
	}
	requestLabel, err := suite.inboxRequestLabelFor(label)
	if err != nil {
		return err
	}
	request := suite.operationInbox.requests[requestLabel]
	for _, party := range []struct {
		role, email, expectedName string
		actual                    inboxPartyResponse
	}{
		{"consumer", request.consumerEmail, consumerName, found.Consumer},
		{"provider", request.providerEmail, providerName, found.Provider},
	} {
		expectedID, err := suite.userRepository.FindIDByEmail(party.email)
		if err != nil {
			return err
		}
		if party.actual.ID != expectedID || party.actual.Name+" "+party.actual.Surname != party.expectedName {
			return fmt.Errorf("expected %s %q with id %d, got %+v", party.role, party.expectedName, expectedID, party.actual)
		}
	}
	return nil
}

func (suite *testSuite) inboxOperationInformsCategory(label, categoryName string) error {
	found, err := suite.inboxOperation(label)
	if err != nil {
		return err
	}
	if found.Category == nil || found.Category.Name != categoryName || found.Category.ID <= 0 {
		return fmt.Errorf("expected category %q, got %+v", categoryName, found.Category)
	}
	return nil
}

func inboxStatusOf(resource *inboxResourceResponse) string {
	if resource == nil {
		return ""
	}
	return resource.Status
}

func (suite *testSuite) inboxOperationInformsDomainStatuses(label, requestStatus, proposalStatus, orderStatus string) error {
	found, err := suite.inboxOperation(label)
	if err != nil {
		return err
	}
	actual := [3]string{inboxStatusOf(found.JobRequest), inboxStatusOf(found.ServiceProposal), inboxStatusOf(found.WorkOrder)}
	if expected := [3]string{requestStatus, proposalStatus, orderStatus}; actual != expected {
		return fmt.Errorf("expected request/proposal/order statuses %v, got %v", expected, actual)
	}
	return nil
}

func (suite *testSuite) inboxOperationInformsDates(label string, table *godog.Table) error {
	found, err := suite.inboxOperation(label)
	if err != nil {
		return err
	}
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	dates := map[string]*time.Time{}
	if request := found.JobRequest; request != nil {
		dates["solicitud creada"] = request.CreatedOn
	}
	if proposal := found.ServiceProposal; proposal != nil {
		dates["propuesta creada"] = proposal.CreatedOn
		dates["fecha programada"] = proposal.ScheduledOn
	}
	if order := found.WorkOrder; order != nil {
		dates["orden aceptada"] = order.AcceptedOn
		dates["finalización informada"] = order.CompletionReportedOn
	}
	for _, row := range rows {
		expected, err := parseInboxInstant(row["valor"])
		if err != nil {
			return err
		}
		actual, exists := dates[row["fecha"]]
		if !exists {
			return fmt.Errorf("unknown or absent date %q", row["fecha"])
		}
		if actual == nil || !actual.Equal(expected) {
			return fmt.Errorf("expected %s %s, got %v", row["fecha"], expected, actual)
		}
	}
	return nil
}

func (suite *testSuite) inboxOperationHasNullBalancePayment(label string) error {
	raw, err := suite.rawInboxOperation(label)
	if err != nil {
		return err
	}
	var order map[string]json.RawMessage
	if err := json.Unmarshal(raw["work_order"], &order); err != nil || order == nil {
		return fmt.Errorf("operation %q has no work order: %s", label, raw["work_order"])
	}
	value, exists := order["balance_paid_on"]
	if !exists || string(value) != "null" {
		return fmt.Errorf("expected an explicit null balance_paid_on, got %q (present: %t)", value, exists)
	}
	return nil
}

func requireInboxAllowedFields(object map[string]json.RawMessage, kind string) error {
	allowed := map[string]bool{}
	for _, field := range inboxAllowedFields[kind] {
		allowed[field] = true
	}
	if len(object) != len(allowed) {
		fields := make([]string, 0, len(object))
		for field := range object {
			fields = append(fields, field)
		}
		return fmt.Errorf("%s exposes fields %v, allowed %v", kind, fields, inboxAllowedFields[kind])
	}
	for field, value := range object {
		if !allowed[field] {
			return fmt.Errorf("%s exposes unexpected field %q", kind, field)
		}
		if _, nested := inboxAllowedFields[field]; !nested || string(value) == "null" {
			continue
		}
		var child map[string]json.RawMessage
		if err := json.Unmarshal(value, &child); err != nil {
			return fmt.Errorf("%s.%s is not an object: %w", kind, field, err)
		}
		if err := requireInboxAllowedFields(child, field); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) inboxResponseIsMinimized() error {
	var page map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &page); err != nil {
		return fmt.Errorf("invalid inbox page: %w", err)
	}
	if err := requireExactJSONFields(page, "inbox page", "operations", "next_cursor"); err != nil {
		return err
	}
	var operations []map[string]json.RawMessage
	if err := json.Unmarshal(page["operations"], &operations); err != nil {
		return fmt.Errorf("invalid inbox operations: %w", err)
	}
	for _, found := range operations {
		if err := requireInboxAllowedFields(found, "operation"); err != nil {
			return err
		}
	}
	body := string(suite.lastBody)
	for _, private := range []string{inboxPrivateMessage, inboxCompletionDescriptionPrefix} {
		if strings.Contains(body, private) {
			return fmt.Errorf("inbox exposes private text %q", private)
		}
	}
	for name, image := range suite.messageImagesByName {
		if strings.Contains(body, image.FileID) {
			return fmt.Errorf("inbox exposes message attachment %q", name)
		}
	}
	for name, image := range suite.completionImagesByName {
		if strings.Contains(body, image.FileID) {
			return fmt.Errorf("inbox exposes completion evidence %q", name)
		}
	}
	return nil
}

// Any audit write advances the committed ingest watermark.
func (suite *testSuite) noAuditEventIsRecorded() error {
	if suite.operationInbox.auditWatermark == nil {
		return fmt.Errorf("the inbox was not queried in this scenario")
	}
	watermark, err := suite.dependencies.Persistence.AuditEventRepository.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return fmt.Errorf("capturing audit ingest watermark: %w", err)
	}
	if watermark != *suite.operationInbox.auditWatermark {
		return fmt.Errorf("the inbox query recorded %d audit events", watermark-*suite.operationInbox.auditWatermark)
	}
	return nil
}

func (suite *testSuite) thereIsInboxOperationInSituation(consumerEmail, providerEmail, situation string) error {
	suite.operationInbox.ensureMaps()
	return suite.withInboxFixtureClock(func() error {
		now := suite.clock.Now().UTC()
		hours := func(offset int) time.Time { return now.Add(time.Duration(offset) * time.Hour) }
		stamp := func(offset int) string { return hours(offset).Format(time.RFC3339) }
		accepted := string(serviceproposal.StatusAccepted)
		switch situation {
		case "una solicitud pendiente":
			return suite.createInboxJobRequest("S", consumerEmail, providerEmail, hours(-1), "pending")
		case "una solicitud aceptada sin propuestas":
			return suite.createInboxJobRequest("S", consumerEmail, providerEmail, hours(-1), "accepted")
		}
		if err := suite.createInboxJobRequest("S", consumerEmail, providerEmail, hours(-168), "accepted"); err != nil {
			return err
		}
		switch situation {
		case "una propuesta pendiente dentro del límite de pago de la seña":
			return suite.createInboxServiceProposal("P", "S", hours(-48), hours(72), 60, string(serviceproposal.StatusPending))
		case "una propuesta pendiente con el límite de pago de la seña vencido":
			return suite.createInboxServiceProposal("P", "S", hours(-48), hours(12), 60, string(serviceproposal.StatusPending))
		case "una orden programada":
			if err := suite.createInboxServiceProposal("P", "S", hours(-48), hours(72), 60, accepted); err != nil {
				return err
			}
			return suite.createInboxWorkOrder("O", "P", hours(-24), string(workorder.StatusScheduled), "", "")
		case "una orden con finalización informada y saldo pendiente", "una orden pagada":
			if err := suite.createInboxServiceProposal("P", "S", hours(-120), hours(-48), 60, accepted); err != nil {
				return err
			}
			status, paidOn := string(workorder.StatusAwaitingPayment), ""
			if situation == "una orden pagada" {
				status, paidOn = string(workorder.StatusPaid), stamp(-12)
			}
			return suite.createInboxWorkOrder("O", "P", hours(-96), status, stamp(-24), paidOn)
		default:
			return fmt.Errorf("unsupported operation situation %q", situation)
		}
	})
}

func (suite *testSuite) inboxOperationBetweenHasNextActionOwner(consumerEmail, providerEmail, owner string) error {
	expected := map[string]string{"el prestador": "provider", "el consumidor": "consumer", "ninguno": "none", "no deducible": ""}
	expectedOwner, known := expected[owner]
	if !known {
		return fmt.Errorf("unsupported next action owner %q", owner)
	}
	consumerID, err := suite.userRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}
	providerID, err := suite.userRepository.FindIDByEmail(providerEmail)
	if err != nil {
		return err
	}
	page, err := suite.decodedInboxPage()
	if err != nil {
		return err
	}
	for _, found := range page.Operations {
		if found.Consumer.ID != consumerID || found.Provider.ID != providerID {
			continue
		}
		if expectedOwner == "" {
			if found.NextActionOwner != nil {
				return fmt.Errorf("expected no deducible next action owner, got %q", *found.NextActionOwner)
			}
			return nil
		}
		if found.NextActionOwner == nil || *found.NextActionOwner != expectedOwner {
			return fmt.Errorf("expected next action owner %q, got %v", expectedOwner, found.NextActionOwner)
		}
		return nil
	}
	return fmt.Errorf("no operation between %q and %q", consumerEmail, providerEmail)
}
