package operation_test

import (
	"context"
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestChatServiceResolvesOnlyDeliveredPageMediaAfterSuccessfulAudit(t *testing.T) {
	for _, auditFails := range []bool{false, true} {
		t.Run(map[bool]string{true: "audit failure", false: "prepared"}[auditFails], func(t *testing.T) {
			ctx := context.Background()
			association := &conversationAssociationReaderMock{}
			messages := &messagePageReaderMock{}
			operators := &chatOperatorIDFinderMock{}
			writer := &chatAuditWriterMock{}
			attachments := &chatAttachmentReaderMock{}
			media := &chatMediaResolverMock{}
			now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
			page := []conversation.Message{{ID: 1, ConversationID: 21, CreatedOn: now}, {ID: 2, ConversationID: 21, CreatedOn: now}, {ID: 3, ConversationID: 21, CreatedOn: now}}
			association.On("FindConversationAssociation", ctx, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 7}).Return(&readmodel.ConversationAssociation{ConversationID: 21, ConsumerID: 11, ProviderID: 12}, nil).Once()
			operators.On("FindOperatorIDByAuthID", ctx, "subject").Return(23, nil).Once()
			messages.On("FindPage", ctx, 21, (*conversation.MessagePosition)(nil), 3).Return(page, nil).Once()
			var auditErr error
			if auditFails {
				auditErr = errors.New("audit unavailable")
			}
			saved := writer.On("Save", ctx, mock.Anything).Return(auditErr).Once()
			if !auditFails {
				refs := map[int]conversation.MessageAttachmentReferences{
					1: {Images: []conversation.MessageImageReference{{FileID: "image", Description: "Evidence"}}, AudioFileID: "audio"},
					2: {VideoFileID: "video"},
					// Even erroneous extra rows cannot broaden which messages get signed.
					3:  {Images: []conversation.MessageImageReference{{FileID: "outside-page"}}},
					99: {VideoFileID: "foreign-video"},
				}
				loaded := attachments.On("FindByMessagePage", ctx, conversation.MessageAttachmentScope{ConversationID: 21, ConsumerID: 11, ProviderID: 12, MessageIDs: []int{1, 2}}).Return(refs, nil).Once()
				image := media.On("ResolveMessageImages", ctx, []string{"image"}).Return(map[string]filedomain.MessageImage{"image": {Image: filedomain.Image{FileID: "image", URL: "signed-image"}}}, nil).Once()
				audio := media.On("ResolveMessageAudios", ctx, []string{"audio"}).Return(map[string]filedomain.MessageAudio{"audio": {FileID: "audio", URL: "signed-audio"}}, nil).Once()
				video := media.On("ResolveMessageVideos", ctx, []string{"video"}).Return(map[string]filedomain.MessageVideo{"video": {FileID: "video", URL: "signed-video"}}, nil).Once()
				mock.InOrder(saved, loaded, image, audio, video)
			}
			service := operation.NewChatService(association, messages, operators, writer, inboxFixedClock{now: now}, attachments, media)
			result, err := service.Query(ctx, "jr-7", "subject", "request-1", "Support case", operation.ChatQuery{Limit: 2})
			if auditFails {
				require.ErrorIs(t, err, auditErr)
				require.Nil(t, result)
				require.Empty(t, attachments.Calls)
				require.Empty(t, media.Calls)
			} else {
				require.NoError(t, err)
				require.Len(t, result.Messages, 2)
				require.Equal(t, "signed-image", result.Messages[0].Images[0].URL)
				require.Equal(t, "Evidence", result.Messages[0].Images[0].Description)
				require.Equal(t, "signed-audio", result.Messages[0].Audio.URL)
				require.Equal(t, "signed-video", result.Messages[1].Video.URL)
			}
			association.AssertExpectations(t)
			operators.AssertExpectations(t)
			messages.AssertExpectations(t)
			writer.AssertExpectations(t)
			attachments.AssertExpectations(t)
			media.AssertExpectations(t)
		})
	}
}

func TestChatServiceOmitsUnavailableMediaWithoutUsingLegacyURLs(t *testing.T) {
	ctx := context.Background()
	association := &conversationAssociationReaderMock{}
	messages := &messagePageReaderMock{}
	operators := &chatOperatorIDFinderMock{}
	writer := &chatAuditWriterMock{}
	attachments := &chatAttachmentReaderMock{}
	media := &chatMediaResolverMock{}
	association.On("FindConversationAssociation", ctx, mock.Anything).Return(&readmodel.ConversationAssociation{ConversationID: 21, ConsumerID: 11, ProviderID: 12}, nil).Once()
	operators.On("FindOperatorIDByAuthID", ctx, "subject").Return(23, nil).Once()
	messages.On("FindPage", ctx, 21, (*conversation.MessagePosition)(nil), 21).Return([]conversation.Message{{ID: 1, Images: []filedomain.MessageImage{{Image: filedomain.Image{FileID: "legacy", URL: "legacy-private-url"}}}}}, nil).Once()
	saved := writer.On("Save", ctx, mock.Anything).Return(nil).Once()
	loaded := attachments.On("FindByMessagePage", ctx, mock.Anything).Return(map[int]conversation.MessageAttachmentReferences{1: {Images: []conversation.MessageImageReference{{FileID: "unavailable"}}}}, nil).Once()
	resolved := media.On("ResolveMessageImages", ctx, []string{"unavailable"}).Return(map[string]filedomain.MessageImage{}, nil).Once()
	mock.InOrder(saved, loaded, resolved)
	service := operation.NewChatService(association, messages, operators, writer, inboxFixedClock{now: time.Now()}, attachments, media)
	result, err := service.Query(ctx, "jr-7", "subject", "request-1", "Support case", operation.ChatQuery{})
	require.NoError(t, err)
	require.Empty(t, result.Messages[0].Images)
	attachments.AssertExpectations(t)
	media.AssertExpectations(t)
}
