package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/cucumber/godog"
)

type operationChatMediaCapture struct {
	resolver operation.ChatMediaResolver
	attempts int
}

func (capture *operationChatMediaCapture) decorate(resolver operation.ChatMediaResolver) operation.ChatMediaResolver {
	capture.resolver = resolver
	return capture
}
func (capture *operationChatMediaCapture) reset() { capture.attempts = 0 }
func (capture *operationChatMediaCapture) ResolveMessageImages(ctx context.Context, ids []string) (map[string]filedomain.MessageImage, error) {
	capture.attempts++
	return capture.resolver.ResolveMessageImages(ctx, ids)
}
func (capture *operationChatMediaCapture) ResolveMessageAudios(ctx context.Context, ids []string) (map[string]filedomain.MessageAudio, error) {
	capture.attempts++
	return capture.resolver.ResolveMessageAudios(ctx, ids)
}
func (capture *operationChatMediaCapture) ResolveMessageVideos(ctx context.Context, ids []string) (map[string]filedomain.MessageVideo, error) {
	capture.attempts++
	return capture.resolver.ResolveMessageVideos(ctx, ids)
}

type operationChatFailureState struct {
	conversationID int
	messageID      int
	imageID        string
	content        string
}

func registerAdminOperationChatFailureSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que "([^"]*)" tiene mensajes persistidos y un adjunto privado confirmado$`, suite.operationChatHasConfirmedPrivateAttachment)
	sc.Step(`^no se generó ningún acceso al adjunto$`, suite.operationChatAuditFailureHasNoMediaAccess)
}

func (suite *testSuite) operationChatHasConfirmedPrivateAttachment(label string) error {
	conversationID, ok := suite.operationChat.conversations[label]
	if !ok {
		return fmt.Errorf("unknown conversation %q", label)
	}
	var consumerEmail string
	for _, request := range suite.operationInbox.requests {
		if request.conversationID == conversationID {
			consumerEmail = request.consumerEmail
		}
	}
	if consumerEmail == "" {
		return fmt.Errorf("conversation is not linked to a prepared request")
	}
	originalAuth, originalConversationID := suite.currentAuth0ID, suite.lastConversationID
	originalPermissions := suite.currentPermissions
	defer func() {
		suite.currentAuth0ID = originalAuth
		suite.currentPermissions = originalPermissions
		suite.lastConversationID = originalConversationID
	}()
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	suite.currentPermissions = nil
	const name = "private-chat-audit-evidence.png"
	if err := suite.uploadAndConfirmMessageImage(name); err != nil {
		return err
	}
	suite.lastConversationID = conversationID
	const content = "Private conversation evidence must never escape a failed audit save"
	if err := suite.requestMessageWithImages(content, []string{name}); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("creating attached chat message returned %d: %s", suite.lastStatus, suite.lastBody)
	}
	var sent sentMessageResponse
	if err := json.Unmarshal(suite.lastBody, &sent); err != nil {
		return err
	}
	image := suite.messageImagesByName[name]
	file, err := suite.fileRepository.FindByID(suite.scenarioContext, image.FileID)
	if err != nil {
		return err
	}
	if file.Status != filedomain.StatusConfirmed || file.Visibility != filedomain.VisibilityPrivate || file.Purpose != filedomain.PurposeConversationMessageImage || file.UploadedByAuthID != suite.currentAuth0ID || file.Key == "" || file.Bucket == "" {
		return fmt.Errorf("chat image is not a real confirmed private message file")
	}
	persisted, err := suite.conversationRepository.FindByID(suite.scenarioContext, conversationID)
	if err != nil {
		return err
	}
	linked := false
	for _, message := range persisted.Messages() {
		if message.ID != sent.ID {
			continue
		}
		if message.ConversationID != conversationID || message.Content != content || len(message.Images) != 1 || message.Images[0].FileID != file.ID {
			return fmt.Errorf("private file is not linked to the exact persisted chat message")
		}
		linked = true
	}
	if !linked {
		return fmt.Errorf("attached message is absent from persisted conversation")
	}
	suite.operationChat.failure = operationChatFailureState{conversationID: conversationID, messageID: sent.ID, imageID: file.ID, content: content}
	if suite.operationChatMediaCapture.resolver == nil {
		return fmt.Errorf("chat media access capture is not connected to the real resolver")
	}
	suite.operationChatMediaCapture.reset()
	return nil
}

func (suite *testSuite) operationChatAuditFailureHasNoMediaAccess() error {
	fixture := suite.operationChat.failure
	if fixture.imageID == "" || fixture.messageID <= 0 || fixture.conversationID <= 0 {
		return fmt.Errorf("confirmed private attachment fixture was not established")
	}
	if !suite.operationDetailAuditCapture.failSave || suite.operationDetailAuditCapture.attempts != 1 {
		return fmt.Errorf("expected one failed synchronous audit save")
	}
	baseline := suite.operationDetailAuditSnapshot
	if baseline == nil {
		return fmt.Errorf("audit failure baseline is absent")
	}
	watermark, err := suite.dependencies.Persistence.AuditEventRepository.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	if watermark != baseline.auditMark {
		return fmt.Errorf("failed chat audit unexpectedly persisted an event")
	}
	if suite.operationChatMediaCapture.attempts != 0 {
		return fmt.Errorf("failed audit attempted %d attachment resolutions", suite.operationChatMediaCapture.attempts)
	}
	if strings.Contains(string(suite.lastBody), fixture.content) || strings.Contains(string(suite.lastBody), fixture.imageID) {
		return fmt.Errorf("failed audit exposed chat content or attachment identity")
	}
	return nil
}
