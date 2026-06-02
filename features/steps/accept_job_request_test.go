package steps_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

const conversationStatusActive = "active"

func registerAcceptJobRequestSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^acepto la solicitud de trabajo pendiente$`, suite.acceptPendingJobRequest)
	sc.Step(`^intento aceptar la solicitud de trabajo pendiente$`, suite.tryAcceptPendingJobRequest)
	sc.Step(`^la solicitud de trabajo queda aceptada$`, suite.systemAcceptsJobRequest)
	sc.Step(`^la conversación vinculada queda activa$`, suite.linkedConversationIsActive)
	sc.Step(`^el prestador puede enviar un mensaje en el chat vinculado$`, suite.providerCanSendMessageInLinkedChat)
	sc.Step(`^el sistema deniega la aceptación de la solicitud$`, suite.systemDeniesJobRequestAcceptance)
	sc.Step(`^que existe una solicitud de trabajo pendiente aceptada$`, suite.thereIsAcceptedPendingJobRequest)
	sc.Step(`^envío seis mensajes en el chat vinculado$`, suite.sendSixMessagesInLinkedChat)
	sc.Step(`^el sistema registra los seis mensajes$`, suite.systemRegistersSixMessages)
}

func (suite *testSuite) acceptPendingJobRequest() error {
	return suite.requestAcceptPendingJobRequest()
}

func (suite *testSuite) tryAcceptPendingJobRequest() error {
	return suite.requestAcceptPendingJobRequest()
}

func (suite *testSuite) requestAcceptPendingJobRequest() error {
	if suite.lastJobRequestID == 0 {
		return fmt.Errorf("expected a prepared job request before accepting it")
	}

	httpReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/job-requests/%d/accept", suite.server.URL, suite.lastJobRequestID), bytes.NewReader(nil))
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

func (suite *testSuite) systemAcceptsJobRequest() error {
	return suite.lastResponseShouldHaveStatusCode(http.StatusOK)
}

func (suite *testSuite) linkedConversationIsActive() error {
	if err := suite.requestConversationByID(suite.lastConversationID); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.Status != conversationStatusActive {
		return fmt.Errorf("expected linked conversation status %q, got %q", conversationStatusActive, response.Status)
	}

	return nil
}

func (suite *testSuite) providerCanSendMessageInLinkedChat() error {
	if err := suite.requestSendMessageToPreparedConversation(sendMessageRequest{
		Content: "Solicitud aceptada, coordinemos el trabajo.",
	}); err != nil {
		return err
	}

	return suite.systemRegistersMessageInConversation()
}

func (suite *testSuite) systemDeniesJobRequestAcceptance() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusForbidden)
}

func (suite *testSuite) thereIsAcceptedPendingJobRequest() error {
	consumerEmail := "consumidor@example.com"
	providerEmail := "prestador@example.com"

	if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(consumerEmail, "Consumidor", "Ejemplo"); err != nil {
		return err
	}
	if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(providerEmail, "Prestador", "Ejemplo", "Plomería"); err != nil {
		return err
	}
	if err := suite.createPendingJobRequest(consumerEmail, providerEmail); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForProviderEmail(providerEmail)
	if err := suite.requestAcceptPendingJobRequest(); err != nil {
		return err
	}

	return suite.systemAcceptsJobRequest()
}

func (suite *testSuite) sendSixMessagesInLinkedChat() error {
	for i := 1; i <= 6; i++ {
		if err := suite.requestSendMessageToPreparedConversation(sendMessageRequest{
			Content: fmt.Sprintf("Mensaje aceptado %d", i),
		}); err != nil {
			return err
		}
		if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
			return err
		}
	}

	return nil
}

func (suite *testSuite) systemRegistersSixMessages() error {
	if err := suite.requestConversationByID(suite.lastConversationID); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	if len(response.Messages) < 6 {
		return fmt.Errorf("expected at least 6 messages in linked chat, got %d with body %s", len(response.Messages), string(suite.lastBody))
	}

	return nil
}
