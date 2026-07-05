package steps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
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
	AmountCents    int64     `json:"amount_cents"`
	ScheduledOn    time.Time `json:"scheduled_on"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
}

type realtimeNotificationEvent struct {
	Type         string                   `json:"type"`
	Notification realtimeNotificationData `json:"notification"`
}

type realtimeNotificationData struct {
	ID           int        `json:"id"`
	UserID       int        `json:"user_id"`
	Type         string     `json:"type"`
	ResourceType string     `json:"resource_type"`
	ResourceID   int        `json:"resource_id"`
	ReadAt       *time.Time `json:"read_at"`
	CreatedAt    time.Time  `json:"created_at"`
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
	consumerID, err := suite.userRepository.FindIDByEmail(consumerEmail)
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

	expectedConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return fmt.Errorf("could not find expected service proposal conversation: %w", err)
	}
	expectedWorkConversation, ok := expectedConversation.(*conversation.WorkConversation)
	if !ok {
		return fmt.Errorf("expected work conversation fixture, got %T", expectedConversation)
	}
	if response.ConversationID != suite.lastConversationID {
		return fmt.Errorf("expected service proposal conversation_id %d, got %d", suite.lastConversationID, response.ConversationID)
	}
	if response.ConsumerID != expectedWorkConversation.ConsumerID {
		return fmt.Errorf("expected service proposal consumer_id %d, got %d", expectedWorkConversation.ConsumerID, response.ConsumerID)
	}
	if response.ProviderID != expectedWorkConversation.ProviderID {
		return fmt.Errorf("expected service proposal provider_id %d, got %d", expectedWorkConversation.ProviderID, response.ProviderID)
	}

	expectedAmountCents, err := httphandler.ParseAmountToCents(suite.lastServiceProposalRequest.Amount)
	if err != nil {
		return fmt.Errorf("could not parse expected service proposal amount: %w", err)
	}
	if response.AmountCents != expectedAmountCents {
		return fmt.Errorf("expected service proposal amount_cents %d, got %d", expectedAmountCents, response.AmountCents)
	}

	expectedScheduledOn, err := time.Parse(time.RFC3339, suite.lastServiceProposalRequest.ScheduledOn)
	if err != nil {
		return fmt.Errorf("could not parse expected service proposal scheduled_on: %w", err)
	}
	if !response.ScheduledOn.Equal(expectedScheduledOn.UTC()) {
		return fmt.Errorf("expected service proposal scheduled_on %s, got %s", expectedScheduledOn.UTC().Format(time.RFC3339), response.ScheduledOn.Format(time.RFC3339))
	}
	if response.Description != suite.lastServiceProposalRequest.Description {
		return fmt.Errorf("expected service proposal description %q, got %q", suite.lastServiceProposalRequest.Description, response.Description)
	}
	if response.Status != string(serviceproposal.StatusPending) {
		return fmt.Errorf("expected service proposal status %q, got %q", serviceproposal.StatusPending, response.Status)
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
	if notification.Type != "service_proposal_received" {
		return fmt.Errorf("expected notification type %q, got %q", "service_proposal_received", notification.Type)
	}
	if notification.ResourceType != "service_proposal" {
		return fmt.Errorf("expected notification resource_type %q, got %q", "service_proposal", notification.ResourceType)
	}
	consumerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("finding expected notification user: %w", err)
	}
	if notification.UserID != consumerID {
		return fmt.Errorf("expected notification user_id %d, got %d", consumerID, notification.UserID)
	}
	response, err := suite.serviceProposalCreationResponseFromLastBody()
	if err != nil {
		return err
	}
	if notification.ResourceID != response.ID {
		return fmt.Errorf("expected notification resource_id %d, got %d", response.ID, notification.ResourceID)
	}
	if notification.ReadAt != nil {
		return fmt.Errorf("expected unread realtime notification, got read_at %s", notification.ReadAt.Format(time.RFC3339))
	}
	if notification.CreatedAt.IsZero() {
		return fmt.Errorf("expected realtime notification created_at to be present")
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
	consumerID, err := suite.userRepository.FindIDByEmail(consumerEmail)
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
	if request, ok := payload.(serviceProposalCreationRequest); ok {
		suite.lastServiceProposalRequest = request
	}

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
