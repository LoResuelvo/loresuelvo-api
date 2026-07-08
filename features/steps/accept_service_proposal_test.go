package steps_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/cucumber/godog"
)

const workOrderStatusScheduled = "scheduled"

type workOrderResponse struct {
	ID                int       `json:"id"`
	ServiceProposalID int       `json:"service_proposal_id"`
	ConsumerID        int       `json:"consumer_id"`
	ProviderID        int       `json:"provider_id"`
	AmountCents       int64     `json:"amount_cents"`
	ScheduledOn       time.Time `json:"scheduled_on"`
	Description       string    `json:"description"`
	Status            string    `json:"status"`
	AcceptedOn        time.Time `json:"accepted_on"`
}

func registerAcceptServiceProposalSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^confirmo la propuesta de servicio pendiente$`, suite.confirmPendingServiceProposal)
	sc.Step(`^intento confirmar la propuesta de servicio pendiente$`, suite.tryConfirmPendingServiceProposal)
	sc.Step(`^intento confirmar la propuesta de servicio pendiente de "([^"]*)"$`, suite.tryConfirmPendingServiceProposalForConsumer)
	sc.Step(`^intento confirmar nuevamente la propuesta de servicio aceptada$`, suite.tryConfirmPendingServiceProposal)
	sc.Step(`^intento confirmar la propuesta de servicio rechazada$`, suite.tryConfirmPendingServiceProposal)
	sc.Step(`^confirmo una de las propuestas de servicio pendientes$`, suite.confirmOnePendingServiceProposal)
	sc.Step(`^que existe una propuesta de servicio aceptada de "([^"]*)" para "([^"]*)"$`, suite.thereIsAcceptedServiceProposal)
	sc.Step(`^que existe una propuesta de servicio rechazada de "([^"]*)" para "([^"]*)"$`, suite.thereIsRejectedServiceProposal)
	sc.Step(`^que existen dos propuestas de servicio pendientes de "([^"]*)" para "([^"]*)"$`, suite.thereAreTwoPendingServiceProposals)
	sc.Step(`^la propuesta de servicio queda aceptada$`, suite.serviceProposalIsAccepted)
	sc.Step(`^la propuesta de servicio confirmada queda aceptada$`, suite.serviceProposalIsAccepted)
	sc.Step(`^la propuesta de servicio permanece pendiente$`, suite.serviceProposalRemainsPending)
	sc.Step(`^la otra propuesta de servicio permanece pendiente$`, suite.otherServiceProposalRemainsPending)
	sc.Step(`^el sistema registra una única orden de trabajo programada$`, suite.systemRegistersOneScheduledWorkOrder)
	sc.Step(`^la orden de trabajo queda vinculada a la propuesta aceptada$`, suite.workOrderIsLinkedToAcceptedServiceProposal)
	sc.Step(`^la orden de trabajo conserva el consumidor, el prestador, el monto, la fecha y hora y la descripción acordados$`, suite.workOrderKeepsAgreedTerms)
	sc.Step(`^el prestador "([^"]*)" recibe en tiempo real la notificación de propuesta de servicio aceptada$`, suite.providerReceivesAcceptedServiceProposalNotification)
	sc.Step(`^el sistema deniega la confirmación de la propuesta de servicio$`, suite.systemDeniesServiceProposalConfirmation)
	sc.Step(`^el sistema rechaza confirmar una propuesta de servicio ya aceptada$`, suite.systemRejectsAlreadyAcceptedServiceProposal)
	sc.Step(`^el sistema rechaza confirmar una propuesta de servicio rechazada$`, suite.systemRejectsRejectedServiceProposal)
	sc.Step(`^el sistema rechaza la confirmación porque la propuesta de servicio está vencida$`, suite.systemRejectsExpiredServiceProposal)
	sc.Step(`^el sistema conserva una única orden de trabajo para la propuesta$`, suite.systemKeepsOneWorkOrderForServiceProposal)
	sc.Step(`^el sistema registra una única orden de trabajo para la propuesta aceptada$`, suite.systemRegistersOneWorkOrderForAcceptedProposal)
	sc.Step(`^el sistema no registra una orden de trabajo para la propuesta$`, suite.systemDoesNotRegisterWorkOrderForServiceProposal)
}

func (suite *testSuite) confirmPendingServiceProposal() error {
	return suite.requestServiceProposalAcceptance(suite.lastServiceProposalID)
}

func (suite *testSuite) tryConfirmPendingServiceProposal() error {
	return suite.confirmPendingServiceProposal()
}

func (suite *testSuite) tryConfirmPendingServiceProposalForConsumer(_ string) error {
	return suite.confirmPendingServiceProposal()
}

func (suite *testSuite) confirmOnePendingServiceProposal() error {
	return suite.confirmPendingServiceProposal()
}

func (suite *testSuite) requestServiceProposalAcceptance(proposalID int) error {
	if proposalID == 0 {
		return fmt.Errorf("expected a prepared service proposal before confirming it")
	}

	httpReq, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/service-proposals/%d/accept", suite.server.URL, proposalID),
		nil,
	)
	if err != nil {
		return err
	}
	if suite.currentAuth0ID != "" {
		httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = body
	if resp.StatusCode == http.StatusCreated {
		workOrder, err := suite.workOrderResponseFromLastBody()
		if err != nil {
			return err
		}
		suite.workOrdersByServiceProposalID[proposalID] = append(
			suite.workOrdersByServiceProposalID[proposalID],
			workOrder,
		)
	}
	return nil
}

func (suite *testSuite) thereIsAcceptedServiceProposal(providerEmail, consumerEmail string) error {
	if err := suite.thereIsPendingServiceProposal(providerEmail, consumerEmail); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	if err := suite.requestServiceProposalAcceptance(suite.lastServiceProposalID); err != nil {
		return err
	}
	return suite.systemRegistersOneScheduledWorkOrder()
}

func (suite *testSuite) thereIsRejectedServiceProposal(providerEmail, consumerEmail string) error {
	return suite.createServiceProposalFixture(
		providerEmail,
		consumerEmail,
		serviceproposal.StatusRejected,
		defaultServiceProposalAmount,
		defaultServiceProposalScheduledOn,
		defaultServiceProposalDescription,
	)
}

func (suite *testSuite) thereAreTwoPendingServiceProposals(providerEmail, consumerEmail string) error {
	participants, err := suite.prepareServiceProposalFixtureParticipants(providerEmail, consumerEmail)
	if err != nil {
		return err
	}
	for range 2 {
		if err := suite.saveServiceProposalFixture(
			participants,
			serviceproposal.StatusPending,
			defaultServiceProposalAmount,
			defaultServiceProposalScheduledOn,
			defaultServiceProposalDescription,
		); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) serviceProposalIsAccepted() error {
	return suite.serviceProposalHasStatus(suite.lastServiceProposalID, serviceproposal.StatusAccepted)
}

func (suite *testSuite) serviceProposalRemainsPending() error {
	return suite.serviceProposalHasStatus(suite.lastServiceProposalID, serviceproposal.StatusPending)
}

func (suite *testSuite) otherServiceProposalRemainsPending() error {
	for _, proposalID := range suite.serviceProposalIDs {
		if proposalID != suite.lastServiceProposalID {
			return suite.serviceProposalHasStatus(proposalID, serviceproposal.StatusPending)
		}
	}
	return fmt.Errorf("expected another prepared service proposal")
}

func (suite *testSuite) serviceProposalHasStatus(proposalID int, expected serviceproposal.Status) error {
	fixture, exists := suite.serviceProposalFixtures[proposalID]
	if !exists {
		return fmt.Errorf("expected fixture for service proposal id %d", proposalID)
	}
	proposals, err := repositories.NewServiceProposalRepository(
		suite.database,
	).FindByUserID(context.Background(), fixture.consumerID)
	if err != nil {
		return fmt.Errorf("finding service proposal fixture: %w", err)
	}
	for _, proposal := range proposals {
		if proposal.ID == proposalID {
			if proposal.Status != expected {
				return fmt.Errorf("expected service proposal %d status %q, got %q", proposalID, expected, proposal.Status)
			}
			return nil
		}
	}
	return fmt.Errorf("expected service proposal id %d", proposalID)
}

func (suite *testSuite) systemRegistersOneScheduledWorkOrder() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	workOrder, err := suite.workOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if workOrder.ID == 0 {
		return fmt.Errorf("expected created work order id, got body %s", string(suite.lastBody))
	}
	if workOrder.Status != workOrderStatusScheduled {
		return fmt.Errorf("expected work order status %q, got %q", workOrderStatusScheduled, workOrder.Status)
	}
	if workOrder.AcceptedOn.IsZero() {
		return fmt.Errorf("expected work order accepted_on, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) workOrderIsLinkedToAcceptedServiceProposal() error {
	workOrder, err := suite.workOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if workOrder.ServiceProposalID != suite.lastServiceProposalID {
		return fmt.Errorf("expected work order service_proposal_id %d, got %d", suite.lastServiceProposalID, workOrder.ServiceProposalID)
	}
	return nil
}

func (suite *testSuite) workOrderKeepsAgreedTerms() error {
	workOrder, err := suite.workOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	fixture := suite.serviceProposalFixtures[suite.lastServiceProposalID]
	if workOrder.ConsumerID != fixture.consumerID {
		return fmt.Errorf("expected work order consumer_id %d, got %d", fixture.consumerID, workOrder.ConsumerID)
	}
	if workOrder.ProviderID != fixture.providerID {
		return fmt.Errorf("expected work order provider_id %d, got %d", fixture.providerID, workOrder.ProviderID)
	}
	if workOrder.AmountCents != fixture.amountCents {
		return fmt.Errorf("expected work order amount_cents %d, got %d", fixture.amountCents, workOrder.AmountCents)
	}
	if !workOrder.ScheduledOn.Equal(fixture.scheduledOn.UTC()) {
		return fmt.Errorf("expected work order scheduled_on %s, got %s", fixture.scheduledOn.UTC(), workOrder.ScheduledOn)
	}
	if workOrder.Description != fixture.description {
		return fmt.Errorf("expected work order description %q, got %q", fixture.description, workOrder.Description)
	}
	return nil
}

func (suite *testSuite) providerReceivesAcceptedServiceProposalNotification(email string) error {
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
	if notification.Type != "service_proposal_accepted" {
		return fmt.Errorf("expected notification type %q, got %q", "service_proposal_accepted", notification.Type)
	}
	if notification.ResourceType != "service_proposal" {
		return fmt.Errorf("expected notification resource_type %q, got %q", "service_proposal", notification.ResourceType)
	}
	providerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("finding expected notification provider: %w", err)
	}
	if notification.UserID != providerID {
		return fmt.Errorf("expected notification user_id %d, got %d", providerID, notification.UserID)
	}
	if notification.ResourceID != suite.lastServiceProposalID {
		return fmt.Errorf("expected notification resource_id %d, got %d", suite.lastServiceProposalID, notification.ResourceID)
	}
	if notification.ReadAt != nil {
		return fmt.Errorf("expected unread realtime notification, got read_at %s", notification.ReadAt.Format(time.RFC3339))
	}
	if notification.CreatedAt.IsZero() {
		return fmt.Errorf("expected realtime notification created_at to be present")
	}
	return nil
}

func (suite *testSuite) systemDeniesServiceProposalConfirmation() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusForbidden)
}

func (suite *testSuite) systemRejectsAlreadyAcceptedServiceProposal() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemRejectsRejectedServiceProposal() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemRejectsExpiredServiceProposal() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemKeepsOneWorkOrderForServiceProposal() error {
	return suite.assertPersistedWorkOrder(suite.lastServiceProposalID)
}

func (suite *testSuite) systemRegistersOneWorkOrderForAcceptedProposal() error {
	return suite.assertPersistedWorkOrder(suite.lastServiceProposalID)
}

func (suite *testSuite) systemDoesNotRegisterWorkOrderForServiceProposal() error {
	_, err := suite.workOrderRepository.FindByServiceProposalID(context.Background(), suite.lastServiceProposalID)
	if errors.Is(err, workorder.ErrDoesNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("expected no work order for service proposal %d", suite.lastServiceProposalID)
}

func (suite *testSuite) assertPersistedWorkOrder(proposalID int) error {
	order, err := suite.workOrderRepository.FindByServiceProposalID(context.Background(), proposalID)
	if err != nil {
		return err
	}
	if order.ServiceProposal.ServiceProposalID() != proposalID {
		return fmt.Errorf("expected work order for service proposal %d, got %d", proposalID, order.ServiceProposal.ServiceProposalID())
	}
	return nil
}

func (suite *testSuite) workOrderForLastServiceProposal() (workOrderResponse, error) {
	workOrders := suite.workOrdersByServiceProposalID[suite.lastServiceProposalID]
	if len(workOrders) != 1 {
		return workOrderResponse{}, fmt.Errorf(
			"expected exactly one work order for service proposal %d, got %d",
			suite.lastServiceProposalID,
			len(workOrders),
		)
	}
	return workOrders[0], nil
}

func (suite *testSuite) workOrderResponseFromLastBody() (workOrderResponse, error) {
	var response workOrderResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return workOrderResponse{}, fmt.Errorf("response is not a valid JSON work order: %w", err)
	}
	return response, nil
}
