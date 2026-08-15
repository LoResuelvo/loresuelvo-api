package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

type conversationSummaryResponse struct {
	ID          int                                     `json:"id"`
	Status      string                                  `json:"status"`
	Counterpart conversationCounterpartResponse         `json:"counterpart"`
	LastMessage *conversationLastMessageSummaryResponse `json:"last_message"`
	UpdatedOn   string                                  `json:"updated_on"`
}

type conversationLastMessageSummaryResponse struct {
	ID         int                   `json:"id"`
	SenderRole string                `json:"sender_role"`
	Content    string                `json:"content"`
	Audio      *messageAudioResponse `json:"audio,omitempty"`
	Video      *messageVideoResponse `json:"video,omitempty"`
	CreatedOn  string                `json:"created_on"`
}

func registerGetConversationsSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^consulto mis conversaciones$`, suite.requestMyConversations)
	sc.Step(`^intento consultar mis conversaciones$`, suite.tryRequestMyConversations)
	sc.Step(`^el sistema muestra un listado de conversaciones vacío$`, suite.systemShowsEmptyConversationList)
	sc.Step(`^el sistema muestra solamente la conversación con el prestador "([^"]*)"$`, suite.systemShowsOnlyConversationWithProvider)
	sc.Step(`^el sistema muestra una conversación pendiente con el consumidor "([^"]*)"$`, suite.systemShowsPendingConversationWithConsumer)
	sc.Step(`^el último mensaje de la conversación es$`, suite.conversationLastMessageIs)
	sc.Step(`^el último mensaje de esa conversación se identifica como un mensaje de audio$`, suite.lastConversationMessageIsAudio)
	sc.Step(`^el último mensaje de esa conversación se identifica como un mensaje con video$`, suite.lastConversationMessageIsVideo)
	sc.Step(`^el último mensaje informa una duración de ([0-9]+) segundos$`, suite.lastConversationMessageDurationIs)
}

func (suite *testSuite) requestMyConversations() error {
	return suite.requestConversations()
}

func (suite *testSuite) tryRequestMyConversations() error {
	return suite.requestConversations()
}

func (suite *testSuite) requestConversations() error {
	httpReq, err := http.NewRequest(http.MethodGet, suite.server.URL+"/conversations", nil)
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

func (suite *testSuite) systemShowsEmptyConversationList() error {
	summaries, err := suite.conversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(summaries) != 0 {
		return fmt.Errorf("expected empty conversation list, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemShowsOnlyConversationWithProvider(fullName string) error {
	summaries, err := suite.conversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(summaries) != 1 {
		return fmt.Errorf("expected exactly one conversation, got %d with body %s", len(summaries), string(suite.lastBody))
	}

	if !conversationCounterpartMatches(summaries[0].Counterpart, participantRoleProvider, fullName) {
		return fmt.Errorf("expected only conversation with provider %q, got body %s", fullName, string(suite.lastBody))
	}

	return suite.assertValidConversationSummary(summaries[0])
}

func (suite *testSuite) systemShowsPendingConversationWithConsumer(fullName string) error {
	summaries, err := suite.conversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	for _, summary := range summaries {
		if !conversationCounterpartMatches(summary.Counterpart, participantRoleConsumer, fullName) {
			continue
		}

		if summary.Status != conversationStatusPending {
			return fmt.Errorf("expected conversation status %q, got %q", conversationStatusPending, summary.Status)
		}

		return suite.assertValidConversationSummary(summary)
	}

	return fmt.Errorf("expected pending conversation with consumer %q, got body %s", fullName, string(suite.lastBody))
}

func (suite *testSuite) conversationLastMessageIs(message *godog.DocString) error {
	summaries, err := suite.conversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(summaries) != 1 {
		return fmt.Errorf("expected exactly one conversation before checking its last message, got %d with body %s", len(summaries), string(suite.lastBody))
	}

	if summaries[0].LastMessage == nil {
		return fmt.Errorf("expected conversation to include last_message, got body %s", string(suite.lastBody))
	}

	expectedContent := normalizeDocString(message)
	if summaries[0].LastMessage.Content != expectedContent {
		return fmt.Errorf("expected last message %q, got %q", expectedContent, summaries[0].LastMessage.Content)
	}

	return nil
}

func (suite *testSuite) lastConversationMessageIsAudio() error {
	summaries, err := suite.conversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(summaries) != 1 {
		return fmt.Errorf("expected exactly one conversation before checking its audio last message, got %d with body %s", len(summaries), string(suite.lastBody))
	}
	if summaries[0].LastMessage == nil || summaries[0].LastMessage.Audio == nil {
		return fmt.Errorf("expected last_message to identify an audio message, got body %s", string(suite.lastBody))
	}
	if summaries[0].LastMessage.ID == 0 || summaries[0].LastMessage.Audio.ID == "" {
		return fmt.Errorf("expected audio last_message to include message and file identifiers, got body %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) lastConversationMessageIsVideo() error {
	summaries, err := suite.conversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(summaries) != 1 {
		return fmt.Errorf("expected exactly one conversation before checking its video last message, got %d with body %s", len(summaries), string(suite.lastBody))
	}
	lastMessage := summaries[0].LastMessage
	if lastMessage == nil || lastMessage.Video == nil {
		return fmt.Errorf("expected last_message to identify a video message, got body %s", string(suite.lastBody))
	}
	if lastMessage.ID == 0 || lastMessage.Video.ID == "" {
		return fmt.Errorf("expected video last_message to include message and file identifiers, got body %s", string(suite.lastBody))
	}
	if suite.lastAttemptedMessageVideoName != "" && lastMessage.Video.OriginalName != suite.lastAttemptedMessageVideoName {
		return fmt.Errorf("expected last video %q, got %q", suite.lastAttemptedMessageVideoName, lastMessage.Video.OriginalName)
	}
	return nil
}

func (suite *testSuite) lastConversationMessageDurationIs(durationText string) error {
	expectedDuration, err := strconv.Atoi(durationText)
	if err != nil || expectedDuration <= 0 {
		return fmt.Errorf("expected a positive audio duration, got %q", durationText)
	}
	summaries, err := suite.conversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(summaries) != 1 || summaries[0].LastMessage == nil {
		return fmt.Errorf("expected one conversation with an audio or video last message, got body %s", string(suite.lastBody))
	}
	if summaries[0].LastMessage.Audio != nil {
		if summaries[0].LastMessage.Audio.DurationSeconds != expectedDuration {
			return fmt.Errorf("expected last audio duration %d seconds, got %d", expectedDuration, summaries[0].LastMessage.Audio.DurationSeconds)
		}
		return nil
	}
	if summaries[0].LastMessage.Video != nil {
		if summaries[0].LastMessage.Video.DurationSeconds != expectedDuration {
			return fmt.Errorf("expected last video duration %d seconds, got %d", expectedDuration, summaries[0].LastMessage.Video.DurationSeconds)
		}
		return nil
	}
	return fmt.Errorf("expected last_message to include audio or video metadata, got body %s", string(suite.lastBody))
}

func (suite *testSuite) conversationSummaryResponsesShouldHaveStatusCode(statusCode int) ([]conversationSummaryResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(statusCode); err != nil {
		return nil, err
	}

	var summaries []conversationSummaryResponse
	if err := json.Unmarshal(suite.lastBody, &summaries); err != nil {
		return nil, fmt.Errorf("response is not valid JSON conversation summary list: %w", err)
	}

	return summaries, nil
}

func (suite *testSuite) assertValidConversationSummary(summary conversationSummaryResponse) error {
	if summary.ID == 0 {
		return fmt.Errorf("expected conversation summary to include id, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.Status) == "" {
		return fmt.Errorf("expected conversation summary to include status, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.UpdatedOn) == "" {
		return fmt.Errorf("expected conversation summary to include updated_on, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.Counterpart.Name) == "" || strings.TrimSpace(summary.Counterpart.Surname) == "" {
		return fmt.Errorf("expected conversation summary to include counterpart name and surname, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.Counterpart.Role) == "" {
		return fmt.Errorf("expected conversation summary to include counterpart role, got body %s", string(suite.lastBody))
	}

	if summary.LastMessage == nil {
		return fmt.Errorf("expected conversation summary to include last_message, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.LastMessage.Content) == "" {
		return fmt.Errorf("expected last_message to include content, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.LastMessage.SenderRole) == "" {
		return fmt.Errorf("expected last_message to include sender_role, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.LastMessage.CreatedOn) == "" {
		return fmt.Errorf("expected last_message to include created_on, got body %s", string(suite.lastBody))
	}

	return nil
}

func conversationCounterpartMatches(counterpart conversationCounterpartResponse, role, fullName string) bool {
	return counterpart.Role == role && strings.TrimSpace(counterpart.Name+" "+counterpart.Surname) == fullName
}
