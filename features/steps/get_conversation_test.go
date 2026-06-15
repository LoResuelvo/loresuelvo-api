package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

const nonExistingConversationID = 999999999

type conversationDetailResponse struct {
	ID          int                             `json:"id"`
	Status      string                          `json:"status"`
	Counterpart conversationCounterpartResponse `json:"counterpart"`
	Messages    []conversationMessageResponse   `json:"messages"`
	UpdatedOn   time.Time                       `json:"updated_on"`
}

type conversationMessageResponse struct {
	ID         int       `json:"id"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	CreatedOn  time.Time `json:"created_on"`
}

func registerGetConversationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una conversación pendiente entre el consumidor "([^"]*)" y el prestador "([^"]*)" con el mensaje inicial:$`, suite.thereIsPendingConversationBetweenConsumerAndProviderWithInitialMessage)
	sc.Step(`^consulto la conversación pendiente con el prestador "([^"]*)"$`, suite.requestPendingConversationWithProvider)
	sc.Step(`^consulto la conversación pendiente con el consumidor "([^"]*)"$`, suite.requestPendingConversationWithConsumer)
	sc.Step(`^intento consultar la conversación pendiente con el prestador "([^"]*)"$`, suite.tryRequestPendingConversationWithProvider)
	sc.Step(`^intento consultar la conversación pendiente con el consumidor "([^"]*)"$`, suite.tryRequestPendingConversationWithConsumer)
	sc.Step(`^intento consultar una conversación inexistente$`, suite.tryRequestNonExistingConversation)
	sc.Step(`^el sistema muestra la conversación entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, suite.systemShowsConversationBetweenConsumerAndProvider)
	sc.Step(`^el detalle de la conversación incluye el mensaje inicial enviado por "([^"]*)":$`, suite.conversationDetailIncludesInitialMessageSentBy)
	sc.Step(`^el sistema me indica que no puedo acceder a esa conversación$`, suite.systemReportsConversationAccessDenied)
	sc.Step(`^el sistema me indica que la conversación no existe$`, suite.systemReportsConversationDoesNotExist)
}

func (suite *testSuite) thereIsPendingConversationBetweenConsumerAndProviderWithInitialMessage(consumerEmail, providerEmail string, message *godog.DocString) error {
	return suite.createPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail, normalizeDocString(message))
}

func (suite *testSuite) createPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail, content string) error {
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	consumerID, err := suite.consumerRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}

	conversationID, err := suite.insertPendingConversationFixture(context.Background(), consumerID, providerID, content)
	if err != nil {
		return err
	}

	suite.lastConversationID = conversationID
	return nil
}

func (suite *testSuite) requestPendingConversationWithProvider(providerEmail string) error {
	return suite.requestPendingConversationWithParticipant(providerEmail)
}

func (suite *testSuite) requestPendingConversationWithConsumer(consumerEmail string) error {
	return suite.requestPendingConversationWithParticipant(consumerEmail)
}

func (suite *testSuite) tryRequestPendingConversationWithProvider(providerEmail string) error {
	return suite.requestPendingConversationWithProvider(providerEmail)
}

func (suite *testSuite) tryRequestPendingConversationWithConsumer(consumerEmail string) error {
	return suite.requestPendingConversationWithConsumer(consumerEmail)
}

func (suite *testSuite) requestPendingConversationWithParticipant(_ string) error {
	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected a prepared conversation before requesting it")
	}

	return suite.requestConversationByID(suite.lastConversationID)
}

func (suite *testSuite) tryRequestNonExistingConversation() error {
	return suite.requestConversationByID(nonExistingConversationID)
}

func (suite *testSuite) requestConversationByID(conversationID int) error {
	httpReq, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/conversations/%d", suite.server.URL, conversationID), nil)
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

func (suite *testSuite) systemShowsConversationBetweenConsumerAndProvider(consumerEmail, providerEmail string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}

	consumerID, err := suite.consumerRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}

	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	if response.ID != suite.lastConversationID {
		return fmt.Errorf("expected conversation id %d, got %d", suite.lastConversationID, response.ID)
	}
	if suite.currentAuth0ID == auth0IDForConsumerEmail(consumerEmail) {
		if response.Counterpart.ID != providerID || response.Counterpart.Role != participantRoleProvider {
			return fmt.Errorf("expected provider counterpart id %d and role %q, got id %d and role %q", providerID, participantRoleProvider, response.Counterpart.ID, response.Counterpart.Role)
		}
	} else if suite.currentAuth0ID == auth0IDForProviderEmail(providerEmail) {
		if response.Counterpart.ID != consumerID || response.Counterpart.Role != participantRoleConsumer {
			return fmt.Errorf("expected consumer counterpart id %d and role %q, got id %d and role %q", consumerID, participantRoleConsumer, response.Counterpart.ID, response.Counterpart.Role)
		}
	} else {
		return fmt.Errorf("authenticated user is not one of the expected conversation participants")
	}
	if response.Status != conversationStatusPending {
		return fmt.Errorf("expected conversation status %q, got %q", conversationStatusPending, response.Status)
	}

	return nil
}

func (suite *testSuite) conversationDetailIncludesInitialMessageSentBy(senderEmail string, message *godog.DocString) error {
	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}

	expectedContent := normalizeDocString(message)
	expectedSenderRole, err := suite.senderRoleForEmail(senderEmail)
	if err != nil {
		return err
	}

	for _, responseMessage := range response.Messages {
		if responseMessage.Content != expectedContent {
			continue
		}

		if responseMessage.SenderRole != expectedSenderRole {
			return fmt.Errorf("expected message sender role %q, got %q", expectedSenderRole, responseMessage.SenderRole)
		}
		if responseMessage.CreatedOn.IsZero() {
			return fmt.Errorf("expected message created_on to be present")
		}
		return nil
	}

	return fmt.Errorf("expected initial message %q in conversation detail, got body %s", expectedContent, string(suite.lastBody))
}

func (suite *testSuite) systemReportsConversationAccessDenied() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusForbidden)
}

func (suite *testSuite) systemReportsConversationDoesNotExist() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusNotFound)
}

func (suite *testSuite) conversationDetailResponseFromLastBody() (conversationDetailResponse, error) {
	var response conversationDetailResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return conversationDetailResponse{}, fmt.Errorf("response is not valid JSON conversation detail response: %w", err)
	}

	return response, nil
}

func (suite *testSuite) senderRoleForEmail(email string) (string, error) {
	if _, err := suite.consumerRepository.FindIDByEmail(email); err == nil {
		return participantRoleConsumer, nil
	}

	if _, err := suite.providerIDByEmail(email); err == nil {
		return participantRoleProvider, nil
	}

	return "", fmt.Errorf("could not resolve sender role for email %q", email)
}

// TODO: no debe haber sql en steps
func (suite *testSuite) insertPendingConversationFixture(ctx context.Context, consumerID, providerID int, initialMessage string) (int, error) {
	tx, err := suite.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning pending conversation fixture transaction: %w", err)
	}

	var conversationID int
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO conversations (type, status, created_on, updated_on)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id`,
		conversationTypeWork,
		conversationStatusPending,
	).Scan(&conversationID)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("inserting pending conversation fixture: %w", err)
	}

	trimmedMessage := normalizeDocString(&godog.DocString{Content: initialMessage})
	if trimmedMessage != "" {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO messages (conversation_id, sender_role, content, created_on)
			VALUES ($1, $2, $3, NOW())`,
			conversationID,
			participantRoleConsumer,
			trimmedMessage,
		); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("inserting pending conversation fixture message: %w", err)
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO work_conversations (conversation_id, consumer_id, provider_id)
		VALUES ($1, $2, $3)`,
		conversationID,
		consumerID,
		providerID,
	); err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("inserting pending work conversation fixture: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing pending conversation fixture transaction: %w", err)
	}

	return conversationID, nil
}
