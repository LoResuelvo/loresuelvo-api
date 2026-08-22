package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/cucumber/godog"
)

const (
	defaultServiceProposalAmount      = int64(1500050)
	defaultServiceProposalDescription = "Servicio acordado entre consumidor y prestador."
)

var defaultServiceProposalScheduledOn = time.Date(2026, time.July, 5, 12, 30, 0, 0, time.UTC)

type serviceProposalSummaryResponse struct {
	ID                       int                                `json:"id"`
	ConversationID           int                                `json:"conversation_id"`
	AmountCents              int64                              `json:"amount_cents"`
	ScheduledOn              time.Time                          `json:"scheduled_on"`
	Description              string                             `json:"description"`
	Status                   string                             `json:"status"`
	EstimatedDurationMinutes int                                `json:"estimated_duration_minutes"`
	CreatedOn                time.Time                          `json:"created_on"`
	Counterpart              serviceProposalCounterpartResponse `json:"counterpart"`
	BookingTerms             bookingTermsResponse               `json:"booking_terms"`
}

type serviceProposalCounterpartResponse struct {
	ID              int    `json:"id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name,omitempty"`
	ProfilePhotoURL string `json:"profile_photo_url"`
}

type serviceProposalFixtureParticipants struct {
	provider     *provider.Provider
	consumer     *consumer.Consumer
	conversation conversation.Conversation
}

type serviceProposalFixture struct {
	providerID  int
	consumerID  int
	amountCents int64
	scheduledOn time.Time
	description string
}

func registerGetServiceProposalsSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una propuesta de servicio pendiente de "([^"]*)" para "([^"]*)" por "([^"]*)" para la fecha y hora "([^"]*)" con la descripción:$`, suite.thereIsPendingServiceProposalWithDetails)
	sc.Step(`^que existe una propuesta de servicio pendiente de "([^"]*)" para "([^"]*)"$`, suite.thereIsPendingServiceProposal)
	sc.Step(`^que existen propuestas de servicio de "([^"]*)" para "([^"]*)" con los estados:$`, suite.thereAreServiceProposalsWithStatuses)
	sc.Step(`^que existen varias propuestas de servicio para "([^"]*)" creadas en distintos momentos$`, suite.thereAreServiceProposalsCreatedAtDifferentTimes)
	sc.Step(`^consulto mis propuestas de servicio$`, suite.requestMyServiceProposals)
	sc.Step(`^intento consultar mis propuestas de servicio$`, suite.requestMyServiceProposals)
	sc.Step(`^el sistema muestra un listado de propuestas de servicio vacío$`, suite.systemShowsEmptyServiceProposalList)
	sc.Step(`^el sistema muestra la propuesta de servicio pendiente por "([^"]*)" para la fecha y hora "([^"]*)"$`, suite.systemShowsPendingServiceProposal)
	sc.Step(`^la propuesta incluye la descripción:$`, suite.serviceProposalIncludesDescription)
	sc.Step(`^la contraparte de la propuesta es el prestador "([^"]*)" con rubro "([^"]*)" y su foto de perfil$`, suite.serviceProposalCounterpartIsProvider)
	sc.Step(`^la contraparte de la propuesta es la consumidora "([^"]*)"$`, suite.serviceProposalCounterpartIsConsumer)
	sc.Step(`^la contraparte no incluye un rubro$`, suite.serviceProposalCounterpartDoesNotIncludeCategory)
	sc.Step(`^la propuesta incluye el identificador de la conversación con el prestador$`, suite.serviceProposalIncludesConversationID)
	sc.Step(`^la propuesta incluye el identificador de la conversación con la consumidora$`, suite.serviceProposalIncludesConversationID)
	sc.Step(`^el sistema muestra las (\d+) propuestas de servicio$`, suite.systemShowsServiceProposalCount)
	sc.Step(`^cada propuesta incluye su estado actual$`, suite.everyServiceProposalIncludesItsStatus)
	sc.Step(`^el sistema muestra solamente la propuesta entre "([^"]*)" y "([^"]*)"$`, suite.systemShowsOnlyServiceProposalBetween)
	sc.Step(`^el sistema muestra las propuestas de servicio ordenadas desde la más reciente$`, suite.systemShowsNewestServiceProposalsFirst)
}

func (suite *testSuite) thereIsPendingServiceProposalWithDetails(providerEmail, consumerEmail, amount, scheduledOn string, description *godog.DocString) error {
	amountCents, err := httphandler.ParseAmountToCents(amount)
	if err != nil {
		return err
	}

	scheduledAt, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return fmt.Errorf("parsing service proposal scheduled_on: %w", err)
	}

	return suite.createServiceProposalFixture(
		providerEmail,
		consumerEmail,
		serviceproposal.StatusPending,
		amountCents,
		scheduledAt.UTC(),
		normalizeDocString(description),
	)
}

func (suite *testSuite) thereIsPendingServiceProposal(providerEmail, consumerEmail string) error {
	return suite.createServiceProposalFixture(
		providerEmail,
		consumerEmail,
		serviceproposal.StatusPending,
		defaultServiceProposalAmount,
		defaultServiceProposalScheduledOn,
		defaultServiceProposalDescription,
	)
}

func (suite *testSuite) thereAreServiceProposalsWithStatuses(providerEmail, consumerEmail string, table *godog.Table) error {
	if table == nil || len(table.Rows) < 2 {
		return fmt.Errorf("expected a table with at least one service proposal status")
	}

	participants, err := suite.prepareServiceProposalFixtureParticipants(providerEmail, consumerEmail)
	if err != nil {
		return err
	}

	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 1 {
			return fmt.Errorf("expected exactly one status per table row")
		}

		status := serviceproposal.Status(strings.TrimSpace(row.Cells[0].Value))
		if !isKnownServiceProposalStatus(status) {
			return fmt.Errorf("unsupported service proposal status %q", status)
		}

		if err := suite.saveServiceProposalFixture(
			participants,
			status,
			defaultServiceProposalAmount,
			defaultServiceProposalScheduledOn,
			defaultServiceProposalDescription,
		); err != nil {
			return err
		}
	}

	return nil
}

func (suite *testSuite) thereAreServiceProposalsCreatedAtDifferentTimes(consumerEmail string) error {
	providerEmails := []string{"juan.plomero@example.com", "pedro.plomero@example.com"}
	for index, providerEmail := range providerEmails {
		if err := suite.thereIsPendingServiceProposal(providerEmail, consumerEmail); err != nil {
			return err
		}
		if index < len(providerEmails)-1 {
			time.Sleep(time.Millisecond)
		}
	}

	return nil
}

func (suite *testSuite) createServiceProposalFixture(
	providerEmail string,
	consumerEmail string,
	status serviceproposal.Status,
	amountCents int64,
	scheduledOn time.Time,
	description string,
) error {
	participants, err := suite.prepareServiceProposalFixtureParticipants(providerEmail, consumerEmail)
	if err != nil {
		return err
	}

	return suite.saveServiceProposalFixture(participants, status, amountCents, scheduledOn, description)
}

func (suite *testSuite) prepareServiceProposalFixtureParticipants(providerEmail, consumerEmail string) (serviceProposalFixtureParticipants, error) {
	conversationKey := serviceProposalParticipantsKey(consumerEmail, providerEmail)
	conversationID := suite.serviceProposalConversationIDs[conversationKey]
	if conversationID == 0 {
		if err := suite.thereIsActiveChatBetweenConsumerAndProviderWithInitialMessage(
			consumerEmail,
			providerEmail,
			&godog.DocString{Content: "Conversación preparada para una propuesta de servicio."},
		); err != nil {
			return serviceProposalFixtureParticipants{}, err
		}
		conversationID = suite.lastConversationID
		suite.serviceProposalConversationIDs[conversationKey] = conversationID
	}

	foundProvider, err := suite.userRepository.FindByAuthID(auth0IDForProviderEmail(providerEmail))
	if err != nil {
		return serviceProposalFixtureParticipants{}, fmt.Errorf("finding service proposal provider fixture: %w", err)
	}
	proposalProvider, ok := foundProvider.(*provider.Provider)
	if !ok {
		return serviceProposalFixtureParticipants{}, fmt.Errorf("expected provider fixture for %q, got %T", providerEmail, foundProvider)
	}

	foundConsumer, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(consumerEmail))
	if err != nil {
		return serviceProposalFixtureParticipants{}, fmt.Errorf("finding service proposal consumer fixture: %w", err)
	}
	proposalConsumer, ok := foundConsumer.(*consumer.Consumer)
	if !ok {
		return serviceProposalFixtureParticipants{}, fmt.Errorf("expected consumer fixture for %q, got %T", consumerEmail, foundConsumer)
	}

	proposalConversation, err := suite.conversationRepository.FindByID(context.Background(), conversationID)
	if err != nil {
		return serviceProposalFixtureParticipants{}, fmt.Errorf("finding service proposal conversation fixture: %w", err)
	}

	return serviceProposalFixtureParticipants{
		provider:     proposalProvider,
		consumer:     proposalConsumer,
		conversation: proposalConversation,
	}, nil
}

func (suite *testSuite) saveServiceProposalFixture(
	participants serviceProposalFixtureParticipants,
	status serviceproposal.Status,
	amountCents int64,
	scheduledOn time.Time,
	description string,
) error {
	repository := repositories.NewServiceProposalRepository(suite.database)
	bookingTerms, err := serviceproposal.NewBookingPolicy().Calculate(amountCents, scheduledOn)
	if err != nil {
		return fmt.Errorf("calculating service proposal fixture booking terms: %w", err)
	}
	savedProposal, err := repository.Save(&serviceproposal.ServiceProposal{
		Provider:                 participants.provider,
		Consumer:                 participants.consumer,
		Conversation:             participants.conversation,
		BookingTerms:             bookingTerms,
		ScheduledOn:              scheduledOn,
		Description:              description,
		EstimatedDurationMinutes: 60,
		Status:                   status,
	})
	if err != nil {
		return fmt.Errorf("saving service proposal fixture: %w", err)
	}

	suite.lastServiceProposalID = savedProposal.ID
	suite.serviceProposalIDs = append(suite.serviceProposalIDs, savedProposal.ID)
	suite.serviceProposalFixtures[savedProposal.ID] = serviceProposalFixture{
		providerID:  participants.provider.ID(),
		consumerID:  participants.consumer.ID(),
		amountCents: amountCents,
		scheduledOn: scheduledOn,
		description: description,
	}

	return nil
}

func (suite *testSuite) requestMyServiceProposals() error {
	httpReq, err := http.NewRequest(http.MethodGet, suite.server.URL+"/service-proposals", nil)
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
	return nil
}

func (suite *testSuite) systemShowsEmptyServiceProposalList() error {
	proposals, err := suite.serviceProposalSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(proposals) != 0 {
		return fmt.Errorf("expected empty service proposal list, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemShowsPendingServiceProposal(amount, scheduledOn string) error {
	proposals, err := suite.serviceProposalSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(proposals) != 1 {
		return fmt.Errorf("expected exactly one service proposal, got %d with body %s", len(proposals), string(suite.lastBody))
	}

	expectedAmount, err := httphandler.ParseAmountToCents(amount)
	if err != nil {
		return err
	}
	expectedScheduledOn, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return err
	}

	proposal := proposals[0]
	if proposal.Status != string(serviceproposal.StatusPending) {
		return fmt.Errorf("expected service proposal status %q, got %q", serviceproposal.StatusPending, proposal.Status)
	}
	if proposal.AmountCents != expectedAmount {
		return fmt.Errorf("expected amount_cents %d, got %d", expectedAmount, proposal.AmountCents)
	}
	if !proposal.ScheduledOn.Equal(expectedScheduledOn.UTC()) {
		return fmt.Errorf("expected scheduled_on %s, got %s", expectedScheduledOn.UTC(), proposal.ScheduledOn)
	}

	return suite.assertValidServiceProposalSummary(proposal)
}

func (suite *testSuite) serviceProposalIncludesDescription(description *godog.DocString) error {
	proposal, err := suite.onlyServiceProposalSummary()
	if err != nil {
		return err
	}
	if proposal.Description != normalizeDocString(description) {
		return fmt.Errorf("expected service proposal description %q, got %q", normalizeDocString(description), proposal.Description)
	}
	return nil
}

func (suite *testSuite) serviceProposalCounterpartIsProvider(fullName, categoryName string) error {
	proposal, err := suite.onlyServiceProposalSummary()
	if err != nil {
		return err
	}
	if !serviceProposalCounterpartMatches(proposal.Counterpart, participantRoleProvider, fullName) {
		return fmt.Errorf("expected provider counterpart %q, got body %s", fullName, string(suite.lastBody))
	}
	if proposal.Counterpart.CategoryName != categoryName {
		return fmt.Errorf("expected counterpart category %q, got %q", categoryName, proposal.Counterpart.CategoryName)
	}
	if strings.TrimSpace(proposal.Counterpart.ProfilePhotoURL) == "" {
		return fmt.Errorf("expected provider counterpart to include profile_photo_url, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) serviceProposalCounterpartIsConsumer(fullName string) error {
	proposal, err := suite.onlyServiceProposalSummary()
	if err != nil {
		return err
	}
	if !serviceProposalCounterpartMatches(proposal.Counterpart, participantRoleConsumer, fullName) {
		return fmt.Errorf("expected consumer counterpart %q, got body %s", fullName, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) serviceProposalCounterpartDoesNotIncludeCategory() error {
	proposal, err := suite.onlyServiceProposalSummary()
	if err != nil {
		return err
	}
	if proposal.Counterpart.CategoryName != "" {
		return fmt.Errorf("expected consumer counterpart without category_name, got %q", proposal.Counterpart.CategoryName)
	}
	return nil
}

func (suite *testSuite) serviceProposalIncludesConversationID() error {
	proposal, err := suite.onlyServiceProposalSummary()
	if err != nil {
		return err
	}
	if proposal.ConversationID == 0 {
		return fmt.Errorf("expected service proposal to include conversation_id, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) systemShowsServiceProposalCount(expectedCount int) error {
	proposals, err := suite.serviceProposalSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(proposals) != expectedCount {
		return fmt.Errorf("expected %d service proposals, got %d with body %s", expectedCount, len(proposals), string(suite.lastBody))
	}
	for _, proposal := range proposals {
		if err := suite.assertValidServiceProposalSummary(proposal); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) everyServiceProposalIncludesItsStatus() error {
	proposals, err := suite.serviceProposalSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	foundStatuses := make(map[string]bool, len(proposals))
	for _, proposal := range proposals {
		if !isKnownServiceProposalStatus(serviceproposal.Status(proposal.Status)) {
			return fmt.Errorf("unexpected or missing service proposal status %q", proposal.Status)
		}
		foundStatuses[proposal.Status] = true
	}

	for _, expected := range []serviceproposal.Status{
		serviceproposal.StatusPending,
		serviceproposal.StatusAccepted,
		serviceproposal.StatusRejected,
	} {
		if !foundStatuses[string(expected)] {
			return fmt.Errorf("expected service proposal status %q, got body %s", expected, string(suite.lastBody))
		}
	}
	return nil
}

func (suite *testSuite) systemShowsOnlyServiceProposalBetween(consumerEmail, providerEmail string) error {
	proposal, err := suite.onlyServiceProposalSummary()
	if err != nil {
		return err
	}

	expectedConversationID := suite.serviceProposalConversationIDs[serviceProposalParticipantsKey(consumerEmail, providerEmail)]
	if expectedConversationID == 0 {
		return fmt.Errorf("expected a prepared service proposal between %q and %q", consumerEmail, providerEmail)
	}
	if proposal.ConversationID != expectedConversationID {
		return fmt.Errorf("expected conversation_id %d, got %d", expectedConversationID, proposal.ConversationID)
	}
	return suite.assertValidServiceProposalSummary(proposal)
}

func (suite *testSuite) systemShowsNewestServiceProposalsFirst() error {
	proposals, err := suite.serviceProposalSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(proposals) < 2 {
		return fmt.Errorf("expected at least two service proposals, got %d", len(proposals))
	}

	for index, proposal := range proposals {
		if err := suite.assertValidServiceProposalSummary(proposal); err != nil {
			return err
		}
		if index > 0 && proposals[index-1].CreatedOn.Before(proposal.CreatedOn) {
			return fmt.Errorf("expected service proposals ordered by created_on descending, got body %s", string(suite.lastBody))
		}
	}
	return nil
}

func (suite *testSuite) onlyServiceProposalSummary() (serviceProposalSummaryResponse, error) {
	proposals, err := suite.serviceProposalSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return serviceProposalSummaryResponse{}, err
	}
	if len(proposals) != 1 {
		return serviceProposalSummaryResponse{}, fmt.Errorf("expected exactly one service proposal, got %d with body %s", len(proposals), string(suite.lastBody))
	}
	return proposals[0], nil
}

func (suite *testSuite) serviceProposalSummaryResponsesShouldHaveStatusCode(statusCode int) ([]serviceProposalSummaryResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(statusCode); err != nil {
		return nil, err
	}

	var proposals []serviceProposalSummaryResponse
	if err := json.Unmarshal(suite.lastBody, &proposals); err != nil {
		return nil, fmt.Errorf("response is not a valid JSON service proposal summary list: %w", err)
	}
	return proposals, nil
}

func (suite *testSuite) assertValidServiceProposalSummary(proposal serviceProposalSummaryResponse) error {
	if proposal.ID == 0 {
		return fmt.Errorf("expected service proposal to include id, got body %s", string(suite.lastBody))
	}
	if proposal.ConversationID == 0 {
		return fmt.Errorf("expected service proposal to include conversation_id, got body %s", string(suite.lastBody))
	}
	if proposal.AmountCents <= 0 {
		return fmt.Errorf("expected service proposal to include positive amount_cents, got body %s", string(suite.lastBody))
	}
	if proposal.ScheduledOn.IsZero() {
		return fmt.Errorf("expected service proposal to include scheduled_on, got body %s", string(suite.lastBody))
	}
	if strings.TrimSpace(proposal.Description) == "" {
		return fmt.Errorf("expected service proposal to include description, got body %s", string(suite.lastBody))
	}
	if !isKnownServiceProposalStatus(serviceproposal.Status(proposal.Status)) {
		return fmt.Errorf("expected service proposal to include a valid status, got body %s", string(suite.lastBody))
	}
	if proposal.CreatedOn.IsZero() {
		return fmt.Errorf("expected service proposal to include created_on, got body %s", string(suite.lastBody))
	}
	if proposal.Counterpart.ID == 0 ||
		strings.TrimSpace(proposal.Counterpart.Role) == "" ||
		strings.TrimSpace(proposal.Counterpart.Name) == "" ||
		strings.TrimSpace(proposal.Counterpart.Surname) == "" {
		return fmt.Errorf("expected service proposal to include a complete counterpart, got body %s", string(suite.lastBody))
	}
	return nil
}

func serviceProposalCounterpartMatches(counterpart serviceProposalCounterpartResponse, role, fullName string) bool {
	return counterpart.Role == role && strings.TrimSpace(counterpart.Name+" "+counterpart.Surname) == fullName
}

func serviceProposalParticipantsKey(consumerEmail, providerEmail string) string {
	return consumerEmail + "|" + providerEmail
}

func isKnownServiceProposalStatus(status serviceproposal.Status) bool {
	return status == serviceproposal.StatusPending ||
		status == serviceproposal.StatusAccepted ||
		status == serviceproposal.StatusRejected
}
