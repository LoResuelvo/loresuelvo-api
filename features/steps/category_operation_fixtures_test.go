package steps_test

import (
	"fmt"

	"github.com/cucumber/godog"
)

func registerCategoryOperationFixtureSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que existe la solicitud de trabajo aceptada "([^"]*)" de "([^"]*)" para "([^"]*)"$`, s.categoryAcceptedRequestFixture)
	sc.Step(`^que existe una solicitud de trabajo aceptada entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, func(consumer, provider string) error {
		return s.categoryAcceptedRequestFixture("S1", consumer, provider)
	})
	sc.Step(`^que la conversación de trabajo vinculada a la solicitud "([^"]*)" está activa$`, s.categoryActiveRequestConversation)
	sc.Step(`^que existe una orden de trabajo programada para la propuesta aceptada en la conversación de la solicitud "([^"]*)" de "([^"]*)" para "([^"]*)" por "([^"]*)" para la fecha y hora "([^"]*)" con la descripción:$`, s.categoryScheduledRequestOrder)
	sc.Step(`^que tengo una sesión válida para una identidad que aún no está registrada$`, func() error {
		s.currentAuth0ID = "auth0|provider-test"
		s.currentPermissions = nil
		s.invalidSession = false
		return nil
	})
}
func (s *testSuite) categoryAcceptedRequestFixture(label, consumer, provider string) error {
	auth, permissions := s.currentAuth0ID, s.currentPermissions
	defer func() { s.currentAuth0ID, s.currentPermissions = auth, permissions }()
	if err := s.createPendingJobRequest(consumer, provider); err != nil {
		return err
	}
	s.currentAuth0ID = auth0IDForProviderEmail(provider)
	if err := s.requestAcceptPendingJobRequest(); err != nil {
		return err
	}
	if err := s.lastResponseShouldHaveStatusCode(200); err != nil {
		return err
	}
	s.operationInbox.ensureMaps()
	s.operationInbox.requests[label] = inboxJobRequestFixture{id: s.lastJobRequestID, conversationID: s.lastConversationID, consumerEmail: consumer, providerEmail: provider, createdOn: s.clock.Now()}
	s.serviceProposalConversationIDs[serviceProposalParticipantsKey(consumer, provider)] = s.lastConversationID
	return nil
}
func (s *testSuite) categoryActiveRequestConversation(label string) error {
	request, ok := s.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("missing accepted request %s", label)
	}
	thread, err := s.conversationRepository.FindByID(s.scenarioContext, request.conversationID)
	if err != nil {
		return err
	}
	if thread.Status() != "active" {
		return fmt.Errorf("request conversation not active")
	}
	s.lastConversationID = request.conversationID
	return nil
}
func (s *testSuite) categoryScheduledRequestOrder(label, provider, consumer, amount, scheduled string, description *godog.DocString) error {
	request, ok := s.operationInbox.requests[label]
	if !ok || request.consumerEmail != consumer || request.providerEmail != provider {
		return fmt.Errorf("request participants mismatch")
	}
	s.serviceProposalConversationIDs[serviceProposalParticipantsKey(consumer, provider)] = request.conversationID
	return s.thereIsScheduledWorkOrderForAcceptedProposalWithDetails(provider, consumer, amount, scheduled, description)
}
