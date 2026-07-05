package steps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/cucumber/godog"
)

type aiJobRequestCreationRequest struct {
	ProviderID int `json:"provider_id"`
}

func registerAIJobRequestSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que tengo una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "([^"]*)" con el título "([^"]*)" y la descripción:$`, suite.iHaveChatbotConversationWithProfessionalAssessment)
	sc.Step(`^que tengo una conversación con el chatbot cuya evaluación vigente clasifica el problema en el rubro "([^"]*)" y determina que puede resolverse sin un profesional$`, suite.iHaveChatbotConversationWithSelfServiceAssessment)
	sc.Step(`^que tengo una conversación con el chatbot cuya evaluación vigente todavía requiere más información$`, suite.iHaveChatbotConversationCollectingInformation)
	sc.Step(`^que tengo una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "([^"]*)"$`, suite.iHaveChatbotConversationWithProfessionalAssessmentForCategory)
	sc.Step(`^que el consumidor "([^"]*)" tiene una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "([^"]*)"$`, suite.consumerHasChatbotConversationWithProfessionalAssessment)
	sc.Step(`^que la última respuesta del chatbot en esa conversación fue por una pregunta fuera del alcance de los problemas del hogar$`, suite.lastChatbotResponseWasOutOfScope)
	sc.Step(`^que mi conversación con el chatbot tenía una evaluación que permitía resolver el problema sin un profesional$`, suite.myChatbotConversationHadSelfServiceAssessment)
	sc.Step(`^que después de aportar nueva información la evaluación vigente requiere un profesional del rubro "([^"]*)" con el título "([^"]*)" y la descripción:$`, suite.newInformationProducesProfessionalAssessment)
	sc.Step(`^que envié al prestador "([^"]*)" una solicitud desde una evaluación con el título "([^"]*)" y la descripción:$`, suite.iSentProviderRequestFromAssessment)
	sc.Step(`^que después de enviar la solicitud la conversación con el chatbot produjo una nueva revisión de la evaluación con el título "([^"]*)" y la descripción:$`, suite.chatbotConversationProducedNewAssessmentRevision)
	sc.Step(`^que ya existe una solicitud de trabajo abierta entre "([^"]*)" y "([^"]*)"$`, suite.openJobRequestAlreadyExistsBetween)

	sc.Step(`^elijo contactar al prestador recomendado "([^"]*)" desde esa conversación con el chatbot$`, suite.chooseRecommendedProviderFromChatbotConversation)
	sc.Step(`^intento contactar al prestador "([^"]*)" desde esa conversación con el chatbot$`, suite.tryContactProviderFromChatbotConversation)
	sc.Step(`^elijo contactar a los prestadores recomendados "([^"]*)" y "([^"]*)" desde esa conversación con el chatbot$`, suite.chooseRecommendedProvidersFromChatbotConversation)
	sc.Step(`^consulto la solicitud de trabajo enviada al prestador "([^"]*)"$`, suite.getJobRequestSentToProvider)

	sc.Step(`^el sistema registra una solicitud de trabajo pendiente para el prestador "([^"]*)"$`, suite.systemRegistersPendingJobRequestForProvider)
	sc.Step(`^la solicitud tiene el título "([^"]*)"$`, suite.jobRequestHasTitle)
	sc.Step(`^la solicitud tiene la descripción obtenida de la evaluación vigente:$`, suite.jobRequestHasCurrentAssessmentDescription)
	sc.Step(`^la solicitud queda vinculada con la evaluación vigente de la conversación con el chatbot$`, suite.jobRequestIsLinkedToCurrentAssessment)
	sc.Step(`^el sistema crea una conversación de trabajo pendiente entre "([^"]*)" y "([^"]*)"$`, suite.systemCreatesPendingWorkConversationBetween)
	sc.Step(`^el sistema indica que la evaluación vigente no requiere contactar a un profesional$`, suite.systemReportsAssessmentDoesNotRequireProfessional)
	sc.Step(`^el sistema indica que todavía no existe información suficiente para contactar a un profesional$`, suite.systemReportsAssessmentNeedsMoreInformation)
	sc.Step(`^el sistema no registra una solicitud de trabajo$`, suite.systemDoesNotRegisterAIJobRequest)
	sc.Step(`^el sistema no crea una conversación de trabajo pendiente$`, suite.systemDoesNotCreatePendingWorkConversation)
	sc.Step(`^la solicitud conserva el título y la descripción de la evaluación profesional vigente$`, suite.jobRequestPreservesProfessionalAssessmentContent)
	sc.Step(`^el sistema indica que el prestador no corresponde al rubro requerido por la evaluación vigente$`, suite.systemReportsProviderCategoryDoesNotMatchAssessment)
	sc.Step(`^el sistema no registra una solicitud de trabajo para "([^"]*)"$`, suite.systemDoesNotRegisterAIJobRequestForProvider)
	sc.Step(`^el sistema no crea una conversación de trabajo pendiente con "([^"]*)"$`, suite.systemDoesNotCreatePendingWorkConversationWithProvider)
	sc.Step(`^ambas solicitudes quedan vinculadas con la misma evaluación vigente$`, suite.bothJobRequestsAreLinkedToSameAssessment)
	sc.Step(`^el sistema crea una conversación de trabajo pendiente con cada prestador contactado$`, suite.systemCreatesPendingWorkConversationWithEachProvider)
	sc.Step(`^el sistema registra la solicitud usando la revisión vigente de la evaluación$`, suite.systemUsesCurrentAssessmentRevision)
	sc.Step(`^la solicitud conserva el título "([^"]*)"$`, suite.jobRequestKeepsTitle)
	sc.Step(`^la solicitud conserva la descripción que fue enviada originalmente:$`, suite.jobRequestKeepsOriginalDescription)
	sc.Step(`^la solicitud continúa vinculada con la revisión de la evaluación que la originó$`, suite.jobRequestRemainsLinkedToSourceAssessment)
	sc.Step(`^el sistema indica que no puedo acceder a esa conversación con el chatbot$`, suite.systemReportsCannotAccessSourceChatbotConversation)
	sc.Step(`^el sistema indica que ya existe una solicitud de trabajo abierta con ese prestador$`, suite.systemReportsOpenAIJobRequestAlreadyExists)
	sc.Step(`^el sistema no registra otra solicitud de trabajo para "([^"]*)"$`, suite.systemDoesNotRegisterAnotherAIJobRequestForProvider)
	sc.Step(`^el sistema no crea otra conversación de trabajo pendiente con "([^"]*)"$`, suite.systemDoesNotCreateAnotherPendingWorkConversationWithProvider)
}

func (suite *testSuite) iHaveChatbotConversationWithProfessionalAssessment(categoryName, title string, description *godog.DocString) error {
	return suite.prepareProfessionalAssessment(categoryName, title, normalizeDocString(description))
}

func (suite *testSuite) iHaveChatbotConversationWithSelfServiceAssessment(categoryName string) error {
	if _, err := suite.categoryIDFor(categoryName); err != nil {
		return err
	}

	suite.aiExpectedJobRequestTitle = "Problema simple de " + categoryName
	suite.aiExpectedJobRequestDescription = "La evaluación determina que el consumidor puede resolver el problema sin un profesional."
	suite.chatbot.SetSelfServiceResponse(suite.aiExpectedJobRequestTitle, suite.aiExpectedJobRequestDescription, categoryName)

	if err := suite.createAndRememberChatbotConversation("Necesito orientación para resolver un problema simple de mi hogar."); err != nil {
		return err
	}

	return suite.rememberCurrentAssessmentIDIfAvailable()
}

func (suite *testSuite) iHaveChatbotConversationCollectingInformation() error {
	suite.aiExpectedJobRequestTitle = ""
	suite.aiExpectedJobRequestDescription = ""
	suite.chatbot.SetResponse("Consulta sobre un problema del hogar", "Necesito más información para evaluar el problema con seguridad.")

	if err := suite.createAndRememberChatbotConversation("Tengo un problema en mi hogar, pero todavía no puedo describirlo con precisión."); err != nil {
		return err
	}

	return suite.rememberCurrentAssessmentIDIfAvailable()
}

func (suite *testSuite) iHaveChatbotConversationWithProfessionalAssessmentForCategory(categoryName string) error {
	return suite.prepareProfessionalAssessment(
		categoryName,
		"Problema que requiere un profesional",
		"La evaluación vigente determina que el problema requiere la intervención de un profesional.",
	)
}

func (suite *testSuite) consumerHasChatbotConversationWithProfessionalAssessment(consumerEmail, categoryName string) error {
	previousAuth0ID := suite.currentAuth0ID
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	err := suite.iHaveChatbotConversationWithProfessionalAssessmentForCategory(categoryName)
	suite.currentAuth0ID = previousAuth0ID
	return err
}

func (suite *testSuite) lastChatbotResponseWasOutOfScope() error {
	suite.chatbot.SetOutOfScopeResponse("Consulta fuera de alcance", "Solo puedo responder consultas relacionadas con problemas del hogar.")
	if err := suite.requestContinueChatbotConversation(suite.aiSourceChatbotConversationID, chatbotConversationRequest{
		Content: "¿Podés recomendarme una receta de cocina?",
	}); err != nil {
		return err
	}

	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return fmt.Errorf("could not prepare out-of-scope chatbot turn: %w", err)
	}

	return suite.rememberCurrentAssessmentIDIfAvailable()
}

func (suite *testSuite) myChatbotConversationHadSelfServiceAssessment() error {
	return suite.iHaveChatbotConversationWithSelfServiceAssessment("Plomería")
}

func (suite *testSuite) newInformationProducesProfessionalAssessment(categoryName, title string, description *godog.DocString) error {
	if _, err := suite.categoryIDFor(categoryName); err != nil {
		return err
	}

	suite.aiExpectedJobRequestTitle = strings.TrimSpace(title)
	suite.aiExpectedJobRequestDescription = normalizeDocString(description)
	suite.chatbot.SetConcludedDiagnosisResponse(
		suite.aiExpectedJobRequestTitle,
		suite.aiExpectedJobRequestDescription,
		categoryName,
	)
	if err := suite.requestContinueChatbotConversation(suite.aiSourceChatbotConversationID, chatbotConversationRequest{
		Content: "El problema continúa después de seguir las indicaciones y ahora presenta estos nuevos síntomas.",
	}); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return fmt.Errorf("could not prepare updated chatbot assessment: %w", err)
	}

	return suite.rememberCurrentAssessmentIDIfAvailable()
}

func (suite *testSuite) iSentProviderRequestFromAssessment(providerFullName, title string, description *godog.DocString) error {
	if err := suite.prepareProfessionalAssessment("Plomería", title, normalizeDocString(description)); err != nil {
		return err
	}
	if err := suite.contactProviderFromChatbotConversation(providerFullName); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return fmt.Errorf("could not prepare AI-generated job request: %w", err)
	}

	return nil
}

func (suite *testSuite) chatbotConversationProducedNewAssessmentRevision(title string, description *godog.DocString) error {
	suite.chatbot.SetConcludedDiagnosisResponse(strings.TrimSpace(title), normalizeDocString(description), "Plomería")
	if err := suite.requestContinueChatbotConversation(suite.aiSourceChatbotConversationID, chatbotConversationRequest{
		Content: "La situación cambió y necesito actualizar la evaluación del problema.",
	}); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return fmt.Errorf("could not prepare newer chatbot assessment revision: %w", err)
	}

	return nil
}

func (suite *testSuite) openJobRequestAlreadyExistsBetween(_, providerFullName string) error {
	return suite.requestJobRequestToProviderFullName(providerFullName, jobRequestPayload{
		title:       "Solicitud abierta existente",
		description: "Solicitud preparada para comprobar que no se generen duplicados.",
	})
}

func (suite *testSuite) chooseRecommendedProviderFromChatbotConversation(providerFullName string) error {
	return suite.contactProviderFromChatbotConversation(providerFullName)
}

func (suite *testSuite) tryContactProviderFromChatbotConversation(providerFullName string) error {
	return suite.contactProviderFromChatbotConversation(providerFullName)
}

func (suite *testSuite) chooseRecommendedProvidersFromChatbotConversation(firstProvider, secondProvider string) error {
	if err := suite.prepareAIContactBaseline([]string{firstProvider, secondProvider}); err != nil {
		return err
	}
	for _, providerFullName := range []string{firstProvider, secondProvider} {
		if err := suite.requestAIJobRequest(providerFullName); err != nil {
			return err
		}
		if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
			return err
		}
	}

	return nil
}

func (suite *testSuite) getJobRequestSentToProvider(providerFullName string) error {
	response, exists := suite.aiJobRequestsByProvider[providerFullName]
	if !exists || response.ID == 0 {
		return fmt.Errorf("expected an AI-generated job request for provider %q", providerFullName)
	}

	suite.lastJobRequestID = response.ID
	return suite.requestMyPendingJobRequests()
}

func (suite *testSuite) systemRegistersPendingJobRequestForProvider(providerFullName string) error {
	response, exists := suite.aiJobRequestsByProvider[providerFullName]
	if !exists {
		return fmt.Errorf("expected a recorded AI-generated job request for provider %q", providerFullName)
	}
	if response.ID == 0 || response.ConversationID == 0 || response.Status != string(jobrequest.StatusPending) {
		return fmt.Errorf("expected pending job request with ids for provider %q, got %+v", providerFullName, response)
	}

	providerAuthID, err := suite.authIDForProviderFullName(providerFullName)
	if err != nil {
		return err
	}
	requests, err := suite.jobRequestRepository.FindByUserAuthID(providerAuthID)
	if err != nil {
		return err
	}
	for _, request := range requests {
		if request.ID == response.ID {
			return nil
		}
	}

	return fmt.Errorf("expected provider %q to receive job request %d", providerFullName, response.ID)
}

func (suite *testSuite) jobRequestHasTitle(expectedTitle string) error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	if request.Title != strings.TrimSpace(expectedTitle) {
		return fmt.Errorf("expected job request title %q, got %q", strings.TrimSpace(expectedTitle), request.Title)
	}

	return nil
}

func (suite *testSuite) jobRequestHasCurrentAssessmentDescription(description *godog.DocString) error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	expectedDescription := normalizeDocString(description)
	if request.Description != expectedDescription {
		return fmt.Errorf("expected job request description %q, got %q", expectedDescription, request.Description)
	}

	return nil
}

func (suite *testSuite) jobRequestIsLinkedToCurrentAssessment() error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	sourceAssessmentID, err := exportedIntField(request, "SourceAssessmentID")
	if err != nil {
		return err
	}
	if sourceAssessmentID == 0 || sourceAssessmentID != suite.aiExpectedAssessmentID {
		return fmt.Errorf("expected job request source assessment id %d, got %d", suite.aiExpectedAssessmentID, sourceAssessmentID)
	}

	return nil
}

func (suite *testSuite) systemCreatesPendingWorkConversationBetween(_, providerFullName string) error {
	providerID, err := suite.providerIDByFullName(providerFullName)
	if err != nil {
		return err
	}
	return suite.assertPendingWorkConversationCreated(providerID)
}

func (suite *testSuite) systemReportsAssessmentDoesNotRequireProfessional() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemReportsAssessmentNeedsMoreInformation() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemDoesNotRegisterAIJobRequest() error {
	currentCount, err := suite.currentConsumerPendingJobRequestCount()
	if err != nil {
		return err
	}
	if currentCount != suite.aiJobRequestCountBeforeContact {
		return fmt.Errorf("expected pending job request count to remain %d, got %d", suite.aiJobRequestCountBeforeContact, currentCount)
	}

	return nil
}

func (suite *testSuite) systemDoesNotCreatePendingWorkConversation() error {
	for _, providerID := range suite.aiAttemptedProviderIDs {
		if err := suite.assertWorkConversationUnchanged(providerID); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) jobRequestPreservesProfessionalAssessmentContent() error {
	if err := suite.jobRequestHasTitle(suite.aiExpectedJobRequestTitle); err != nil {
		return err
	}
	return suite.jobRequestHasDescription(suite.aiExpectedJobRequestDescription)
}

func (suite *testSuite) systemReportsProviderCategoryDoesNotMatchAssessment() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemDoesNotRegisterAIJobRequestForProvider(_ string) error {
	return suite.systemDoesNotRegisterAIJobRequest()
}

func (suite *testSuite) systemDoesNotCreatePendingWorkConversationWithProvider(providerFullName string) error {
	providerID, err := suite.providerIDByFullName(providerFullName)
	if err != nil {
		return err
	}
	return suite.assertWorkConversationUnchanged(providerID)
}

func (suite *testSuite) bothJobRequestsAreLinkedToSameAssessment() error {
	var sourceAssessmentID int
	for providerFullName := range suite.aiJobRequestsByProvider {
		request, err := suite.aiJobRequestForProvider(providerFullName)
		if err != nil {
			return err
		}
		requestAssessmentID, err := exportedIntField(request, "SourceAssessmentID")
		if err != nil {
			return err
		}
		if sourceAssessmentID == 0 {
			sourceAssessmentID = requestAssessmentID
			continue
		}
		if requestAssessmentID != sourceAssessmentID {
			return fmt.Errorf("expected all job requests to reference assessment %d, got %d", sourceAssessmentID, requestAssessmentID)
		}
	}
	if sourceAssessmentID == 0 || sourceAssessmentID != suite.aiExpectedAssessmentID {
		return fmt.Errorf("expected both job requests to reference current assessment %d, got %d", suite.aiExpectedAssessmentID, sourceAssessmentID)
	}

	return nil
}

func (suite *testSuite) systemCreatesPendingWorkConversationWithEachProvider() error {
	for _, providerID := range suite.aiAttemptedProviderIDs {
		if err := suite.assertPendingWorkConversationCreated(providerID); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) systemUsesCurrentAssessmentRevision() error {
	return suite.jobRequestIsLinkedToCurrentAssessment()
}

func (suite *testSuite) jobRequestKeepsTitle(expectedTitle string) error {
	return suite.jobRequestHasTitle(expectedTitle)
}

func (suite *testSuite) jobRequestKeepsOriginalDescription(description *godog.DocString) error {
	return suite.jobRequestHasDescription(normalizeDocString(description))
}

func (suite *testSuite) jobRequestRemainsLinkedToSourceAssessment() error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	sourceAssessmentID, err := exportedIntField(request, "SourceAssessmentID")
	if err != nil {
		return err
	}
	if sourceAssessmentID == 0 || sourceAssessmentID != suite.aiExpectedAssessmentID {
		return fmt.Errorf("expected job request to remain linked to source assessment %d, got %d", suite.aiExpectedAssessmentID, sourceAssessmentID)
	}

	return nil
}

func (suite *testSuite) systemReportsCannotAccessSourceChatbotConversation() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusForbidden)
}

func (suite *testSuite) systemReportsOpenAIJobRequestAlreadyExists() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemDoesNotRegisterAnotherAIJobRequestForProvider(_ string) error {
	return suite.systemDoesNotRegisterAIJobRequest()
}

func (suite *testSuite) systemDoesNotCreateAnotherPendingWorkConversationWithProvider(providerFullName string) error {
	return suite.systemDoesNotCreatePendingWorkConversationWithProvider(providerFullName)
}

func (suite *testSuite) prepareProfessionalAssessment(categoryName, title, description string) error {
	if _, err := suite.categoryIDFor(categoryName); err != nil {
		return err
	}

	suite.aiExpectedJobRequestTitle = strings.TrimSpace(title)
	suite.aiExpectedJobRequestDescription = strings.TrimSpace(description)
	suite.chatbot.SetConcludedDiagnosisResponse(
		suite.aiExpectedJobRequestTitle,
		suite.aiExpectedJobRequestDescription,
		categoryName,
	)
	if err := suite.createAndRememberChatbotConversation("Necesito ayuda para evaluar este problema de mi hogar."); err != nil {
		return err
	}

	return suite.rememberCurrentAssessmentIDIfAvailable()
}

func (suite *testSuite) createAndRememberChatbotConversation(content string) error {
	if err := suite.requestCreateChatbotConversation(chatbotConversationRequest{Content: content}); err != nil {
		return err
	}
	if err := suite.rememberCreatedChatbotConversation(); err != nil {
		return fmt.Errorf("could not prepare chatbot conversation: %w", err)
	}
	suite.aiSourceChatbotConversationID = suite.lastConversationID

	return nil
}

func (suite *testSuite) contactProviderFromChatbotConversation(providerFullName string) error {
	if err := suite.prepareAIContactBaseline([]string{providerFullName}); err != nil {
		return err
	}
	return suite.requestAIJobRequest(providerFullName)
}

func (suite *testSuite) prepareAIContactBaseline(providerFullNames []string) error {
	count, err := suite.currentConsumerPendingJobRequestCount()
	if err != nil {
		return err
	}
	suite.aiJobRequestCountBeforeContact = count
	suite.aiAttemptedProviderIDs = nil
	suite.aiWorkConversationIDsBeforeContact = map[int]int{}

	for _, providerFullName := range providerFullNames {
		providerID, err := suite.providerIDByFullName(providerFullName)
		if err != nil {
			return err
		}
		suite.aiAttemptedProviderIDs = append(suite.aiAttemptedProviderIDs, providerID)
		conversationID, err := suite.workConversationIDWithCurrentConsumer(providerID)
		if err != nil {
			return err
		}
		suite.aiWorkConversationIDsBeforeContact[providerID] = conversationID
	}

	return nil
}

func (suite *testSuite) requestAIJobRequest(providerFullName string) error {
	providerID, err := suite.providerIDByFullName(providerFullName)
	if err != nil {
		return err
	}
	body, err := json.Marshal(aiJobRequestCreationRequest{ProviderID: providerID})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/chatbot/conversations/%d/job-requests", suite.server.URL, suite.aiSourceChatbotConversationID)
	httpRequest, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if suite.currentAuth0ID != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer response.Body.Close()

	suite.lastBody, err = io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	suite.lastStatus = response.StatusCode

	if response.StatusCode == http.StatusCreated {
		createdJobRequest, err := suite.jobRequestCreationResponseFromLastBody()
		if err != nil {
			return err
		}
		suite.lastJobRequestID = createdJobRequest.ID
		suite.lastConversationID = createdJobRequest.ConversationID
		suite.aiJobRequestsByProvider[providerFullName] = createdJobRequest
	}

	return nil
}

func (suite *testSuite) currentConsumerPendingJobRequestCount() (int, error) {
	if strings.TrimSpace(suite.currentAuth0ID) == "" {
		return 0, nil
	}
	requests, err := suite.jobRequestRepository.FindByUserAuthID(suite.currentAuth0ID)
	if err != nil {
		return 0, err
	}
	return len(requests), nil
}

func (suite *testSuite) workConversationIDWithCurrentConsumer(providerID int) (int, error) {
	if strings.TrimSpace(suite.currentAuth0ID) == "" {
		return 0, nil
	}
	consumerID, err := suite.userRepository.FindIDByAuthID(suite.currentAuth0ID)
	if err != nil {
		return 0, nil
	}
	exists, err := suite.conversationRepository.ExistsBetween(consumerID, providerID)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	foundConversation, err := suite.conversationRepository.FindBetween(consumerID, providerID)
	if err != nil {
		return 0, err
	}
	return foundConversation.Base().ID, nil
}

func (suite *testSuite) assertPendingWorkConversationCreated(providerID int) error {
	consumerID, err := suite.userRepository.FindIDByAuthID(suite.currentAuth0ID)
	if err != nil {
		return err
	}
	foundConversation, err := suite.conversationRepository.FindBetween(consumerID, providerID)
	if err != nil {
		return err
	}
	if foundConversation.ConversationType() != conversation.TypeWork {
		return fmt.Errorf("expected work conversation, got %q", foundConversation.ConversationType())
	}
	if foundConversation.Base().Status != conversation.StatusPending {
		return fmt.Errorf("expected pending work conversation, got status %q", foundConversation.Base().Status)
	}
	if foundConversation.Base().ID == suite.aiWorkConversationIDsBeforeContact[providerID] {
		return fmt.Errorf("expected a new pending work conversation for provider id %d", providerID)
	}

	return nil
}

func (suite *testSuite) assertWorkConversationUnchanged(providerID int) error {
	currentConversationID, err := suite.workConversationIDWithCurrentConsumer(providerID)
	if err != nil {
		return err
	}
	expectedConversationID := suite.aiWorkConversationIDsBeforeContact[providerID]
	if currentConversationID != expectedConversationID {
		return fmt.Errorf("expected work conversation id to remain %d for provider id %d, got %d", expectedConversationID, providerID, currentConversationID)
	}

	return nil
}

func (suite *testSuite) lastAIJobRequest() (*jobrequest.JobRequest, error) {
	if suite.lastJobRequestID == 0 {
		return nil, fmt.Errorf("expected an AI-generated job request id")
	}
	return suite.jobRequestRepository.FindByID(suite.lastJobRequestID)
}

func (suite *testSuite) aiJobRequestForProvider(providerFullName string) (*jobrequest.JobRequest, error) {
	response, exists := suite.aiJobRequestsByProvider[providerFullName]
	if !exists || response.ID == 0 {
		return nil, fmt.Errorf("expected an AI-generated job request for provider %q", providerFullName)
	}
	return suite.jobRequestRepository.FindByID(response.ID)
}

func (suite *testSuite) jobRequestHasDescription(expectedDescription string) error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	if request.Description != strings.TrimSpace(expectedDescription) {
		return fmt.Errorf("expected job request description %q, got %q", strings.TrimSpace(expectedDescription), request.Description)
	}

	return nil
}

func (suite *testSuite) rememberCurrentAssessmentIDIfAvailable() error {
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.aiSourceChatbotConversationID)
	if err != nil {
		return err
	}
	assessmentID, err := nestedExportedIntField(foundConversation, "CurrentAssessment", "ID")
	if err != nil {
		// The assessment model is introduced by US-51. Keeping zero lets these
		// @wip steps compile before the production model exists.
		suite.aiExpectedAssessmentID = 0
		return nil
	}
	suite.aiExpectedAssessmentID = assessmentID
	return nil
}

func exportedIntField(value any, fieldName string) (int, error) {
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return 0, fmt.Errorf("expected non-nil value with field %s", fieldName)
		}
		reflected = reflected.Elem()
	}
	if reflected.Kind() != reflect.Struct {
		return 0, fmt.Errorf("expected struct with field %s", fieldName)
	}
	field := reflected.FieldByName(fieldName)
	if field.IsValid() && field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return 0, fmt.Errorf("expected non-nil integer field %s", fieldName)
		}
		field = field.Elem()
	}
	if !field.IsValid() || !field.CanInt() {
		return 0, fmt.Errorf("expected integer field %s", fieldName)
	}
	return int(field.Int()), nil
}

func nestedExportedIntField(value any, parentFieldName, childFieldName string) (int, error) {
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return 0, fmt.Errorf("expected non-nil value with field %s", parentFieldName)
		}
		reflected = reflected.Elem()
	}
	if reflected.Kind() != reflect.Struct {
		return 0, fmt.Errorf("expected struct with field %s", parentFieldName)
	}
	parent := reflected.FieldByName(parentFieldName)
	if !parent.IsValid() {
		return 0, fmt.Errorf("expected field %s", parentFieldName)
	}
	if parent.Kind() == reflect.Pointer {
		if parent.IsNil() {
			return 0, fmt.Errorf("expected non-nil field %s", parentFieldName)
		}
		parent = parent.Elem()
	}
	if parent.Kind() != reflect.Struct {
		return 0, fmt.Errorf("expected struct field %s", parentFieldName)
	}
	child := parent.FieldByName(childFieldName)
	if !child.IsValid() || !child.CanInt() {
		return 0, fmt.Errorf("expected integer field %s.%s", parentFieldName, childFieldName)
	}
	return int(child.Int()), nil
}
