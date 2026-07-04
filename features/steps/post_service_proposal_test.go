package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

type serviceProposalCreationRequest struct {
	ConsumerID  int    `json:"consumer_id"`
	Amount      string `json:"amount,omitempty"`
	ScheduledOn string `json:"scheduled_on,omitempty"`
	Description string `json:"description,omitempty"`
}

type serviceProposalCreationResponse struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	ConsumerID     int       `json:"consumer_id"`
	ProviderID     int       `json:"provider_id"`
	Amount         string    `json:"amount"`
	AmountCents    int       `json:"amount_cents"`
	ScheduledOn    time.Time `json:"scheduled_on"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
}

type realtimeNotificationEvent struct {
	Type         string                   `json:"type"`
	Notification realtimeNotificationData `json:"notification"`
}

type realtimeNotificationData struct {
	ID        int            `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Metadata  map[string]any `json:"metadata"`
	CreatedOn time.Time      `json:"created_on"`
}

func registerPostServiceProposalSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que la fecha y hora actual del sistema es "([^"]*)"$`, suite.systemDateTimeIs)
	sc.Step(`^envío una propuesta de servicio al consumidor "([^"]*)" por "([^"]*)" para la fecha y hora "([^"]*)" con la descripción:$`, suite.sendServiceProposalToConsumerForDateTimeWithDescription)
	sc.Step(`^intento enviar una propuesta de servicio al consumidor "([^"]*)" por "([^"]*)" para la fecha y hora "([^"]*)" con la descripción:$`, suite.trySendServiceProposalToConsumerForDateTimeWithDescription)
	sc.Step(`^intento enviar una propuesta de servicio al consumidor "([^"]*)" con falta de parámetros$`, suite.trySendServiceProposalToConsumerWithMissingParameters)
	sc.Step(`^el sistema registra la propuesta de servicio$`, suite.systemRegistersServiceProposal)
	sc.Step(`^el consumidor "([^"]*)" recibe en tiempo real la notificación de propuesta de servicio$`, suite.consumerReceivesRealtimeServiceProposalNotification)
	sc.Step(`^el sistema rechaza la propuesta de servicio porque el monto es inválido$`, suite.systemRejectsServiceProposalBecauseAmountIsInvalid)
	sc.Step(`^el sistema rechaza la propuesta de servicio porque no existe un chat activo con ese consumidor$`, suite.systemRejectsServiceProposalBecauseActiveChatIsRequired)
	sc.Step(`^el sistema rechaza la propuesta de servicio porque la fecha y hora debe ser futura$`, suite.systemRejectsServiceProposalBecauseScheduledOnMustBeFuture)
	sc.Step(`^el sistema rechaza la propuesta de servicio porque faltan parámetros obligatorios$`, suite.systemRejectsServiceProposalBecauseRequiredParametersAreMissing)
}

func (suite *testSuite) systemDateTimeIs(currentDateTime string) error {
	return suite.requestTestClockMock(currentDateTime)
}

func (suite *testSuite) sendServiceProposalToConsumerForDateTimeWithDescription(consumerEmail, amount, scheduledOn string, description *godog.DocString) error {
	return suite.requestServiceProposalToConsumer(consumerEmail, serviceProposalPayload{
		amount:      amount,
		scheduledOn: scheduledOn,
		description: normalizeDocString(description),
	})
}

func (suite *testSuite) trySendServiceProposalToConsumerForDateTimeWithDescription(consumerEmail, amount, scheduledOn string, description *godog.DocString) error {
	return suite.sendServiceProposalToConsumerForDateTimeWithDescription(consumerEmail, amount, scheduledOn, description)
}

func (suite *testSuite) trySendServiceProposalToConsumerWithMissingParameters(consumerEmail string) error {
	consumerID, err := suite.consumerRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}

	return suite.requestServiceProposal(serviceProposalCreationRequest{ConsumerID: consumerID})
}

func (suite *testSuite) systemRegistersServiceProposal() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	response, err := suite.serviceProposalCreationResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.ID == 0 {
		return fmt.Errorf("expected created service proposal id, got body %s", string(suite.lastBody))
	}
	if response.ConversationID == 0 {
		return fmt.Errorf("expected service proposal conversation_id, got body %s", string(suite.lastBody))
	}
	if response.ConsumerID == 0 {
		return fmt.Errorf("expected service proposal consumer_id, got body %s", string(suite.lastBody))
	}
	if response.ProviderID == 0 {
		return fmt.Errorf("expected service proposal provider_id, got body %s", string(suite.lastBody))
	}
	if strings.TrimSpace(response.Amount) == "" && response.AmountCents == 0 {
		return fmt.Errorf("expected service proposal amount or amount_cents, got body %s", string(suite.lastBody))
	}
	if response.ScheduledOn.IsZero() {
		return fmt.Errorf("expected service proposal scheduled_on, got body %s", string(suite.lastBody))
	}
	if strings.TrimSpace(response.Description) == "" {
		return fmt.Errorf("expected service proposal description, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) consumerReceivesRealtimeServiceProposalNotification(email string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}

	event, err := connection.readNotificationEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}

	if event.Type != "notification.created" {
		return fmt.Errorf("expected realtime event type %q, got %q", "notification.created", event.Type)
	}
	notification := event.Notification
	if notification.ID == 0 {
		return fmt.Errorf("expected realtime notification id to be present")
	}
	if notification.Type != "service_proposal.created" {
		return fmt.Errorf("expected notification type %q, got %q", "service_proposal.created", notification.Type)
	}
	if strings.TrimSpace(notification.Title) == "" {
		return fmt.Errorf("expected realtime notification title to be present")
	}
	if strings.TrimSpace(notification.Body) == "" {
		return fmt.Errorf("expected realtime notification body to be present")
	}
	if notification.CreatedOn.IsZero() {
		return fmt.Errorf("expected realtime notification created_on to be present")
	}
	if err := assertServiceProposalNotificationMetadata(notification.Metadata); err != nil {
		return err
	}

	return nil
}

func (suite *testSuite) systemRejectsServiceProposalBecauseAmountIsInvalid() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusBadRequest)
}

func (suite *testSuite) systemRejectsServiceProposalBecauseActiveChatIsRequired() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemRejectsServiceProposalBecauseScheduledOnMustBeFuture() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusBadRequest)
}

func (suite *testSuite) systemRejectsServiceProposalBecauseRequiredParametersAreMissing() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusBadRequest)
}

type serviceProposalPayload struct {
	amount      string
	scheduledOn string
	description string
}

func (suite *testSuite) requestServiceProposalToConsumer(consumerEmail string, payload serviceProposalPayload) error {
	consumerID, err := suite.consumerRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}

	return suite.requestServiceProposal(serviceProposalCreationRequest{
		ConsumerID:  consumerID,
		Amount:      payload.amount,
		ScheduledOn: payload.scheduledOn,
		Description: payload.description,
	})
}

func (suite *testSuite) requestServiceProposal(payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/service-proposals", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if suite.currentAuth0ID != "" {
		httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = responseBody

	return nil
}

func (suite *testSuite) requestTestClockMock(currentDateTime string) error {
	body, err := json.Marshal(map[string]string{"now": currentDateTime})
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/test/clock", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = responseBody
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not mock system date and time: status %d, body %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

func (suite *testSuite) serviceProposalCreationResponseFromLastBody() (serviceProposalCreationResponse, error) {
	var response serviceProposalCreationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return serviceProposalCreationResponse{}, fmt.Errorf("response is not valid JSON service proposal creation response: %w", err)
	}

	return response, nil
}

func (connection *realtimeTestConnection) readNotificationEvent(timeout time.Duration) (realtimeNotificationEvent, error) {
	if err := connection.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return realtimeNotificationEvent{}, err
	}

	payload, err := connection.readTextFrame()
	if err != nil {
		return realtimeNotificationEvent{}, err
	}

	var event realtimeNotificationEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return realtimeNotificationEvent{}, fmt.Errorf("realtime notification is not valid JSON: %w", err)
	}

	_ = connection.conn.SetReadDeadline(time.Time{})
	return event, nil
}

func assertServiceProposalNotificationMetadata(metadata map[string]any) error {
	if len(metadata) == 0 {
		return fmt.Errorf("expected realtime notification metadata to be present")
	}

	requiredKeys := []string{"service_proposal_id", "consumer_id", "provider_id", "conversation_id", "amount_cents", "scheduled_on"}
	for _, key := range requiredKeys {
		value, ok := metadata[key]
		if !ok || value == nil {
			return fmt.Errorf("expected realtime notification metadata %q to be present", key)
		}
	}

	return nil
}
