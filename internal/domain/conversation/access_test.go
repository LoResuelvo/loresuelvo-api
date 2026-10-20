package conversation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetByIDRejectsIdentityAndRoleMismatchBeforeDetail(t *testing.T) {
	for _, test := range []struct {
		name, role string
		id         int
	}{
		{"consumer with provider ID", conversation.SenderConsumer, 20},
		{"provider with consumer ID", conversation.SenderProvider, 10},
		{"unsupported role with participant ID", "administrator", 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &accessReaderMock{}
			reader.On("FindAccessByID", mock.Anything, 1).Return(&readmodel.ConversationAccess{ID: 1, Type: conversation.TypeWork, ConsumerID: 10, ProviderID: 20}, nil).Once()
			users := &participantUserRepositoryMock{}
			users.On("FindByAuthID", "auth").Return(user.RehydrateBaseUser(test.id, "auth", "", "", "", test.role, nil), nil).Once()
			files := &fileURLResolverMock{}
			service := conversation.NewService(nil, users, reader, nil, nil, nil, files, fixedClock{}, conversation.ProviderRecommendationConfig{}, nil)
			detail, err := service.GetByID(context.Background(), "auth", 1)
			require.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
			require.Nil(t, detail)
			reader.AssertNotCalled(t, "FindDetailByIDRoleAndType", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			require.Empty(t, files.resolvedFileIDs)
			reader.AssertExpectations(t)
			users.AssertExpectations(t)
		})
	}
}

func TestGetByIDPreservesReadFailures(t *testing.T) {
	for _, stage := range []string{"access", "user", "detail", "files", "attachments"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("read failed")
			reader := &accessReaderMock{}
			users := &participantUserRepositoryMock{}
			files := &detailFileServiceMock{}
			access := &readmodel.ConversationAccess{ID: 1, Type: conversation.TypeWork, ConsumerID: 10, ProviderID: 20}
			if stage == "access" {
				reader.On("FindAccessByID", mock.Anything, 1).Return(nil, failure).Once()
			} else {
				reader.On("FindAccessByID", mock.Anything, 1).Return(access, nil).Once()
				if stage == "user" {
					users.On("FindByAuthID", "auth").Return(nil, failure).Once()
				} else {
					users.On("FindByAuthID", "auth").Return(user.RehydrateBaseUser(10, "auth", "", "", "", conversation.SenderConsumer, nil), nil).Once()
					if stage == "detail" {
						reader.On("FindDetailByIDRoleAndType", mock.Anything, 1, conversation.SenderConsumer, conversation.TypeWork).Return(nil, failure).Once()
					} else {
						detail := conversationDetailFixture(conversation.SenderProvider)
						if stage == "attachments" {
							detail.Work.Counterpart.ProfilePhotoFileID = ""
							detail.Messages = []readmodel.MessageDetail{{ID: 1, Images: []filedomain.MessageImage{{Image: filedomain.Image{FileID: "image"}}}}}
						}
						reader.On("FindDetailByIDRoleAndType", mock.Anything, 1, conversation.SenderConsumer, conversation.TypeWork).Return(detail, nil).Once()
						if stage == "files" {
							files.On("ResolvePublicURL", mock.Anything, mock.Anything).Return("", failure).Once()
						} else {
							files.On("ResolvePublicURL", mock.Anything, "").Return("", nil).Once()
							files.On("ResolveMessageImages", mock.Anything, []string{"image"}).Return(nil, failure).Once()
						}
					}
				}
			}
			service := conversation.NewService(nil, users, reader, nil, nil, nil, files, fixedClock{now: time.Now()}, conversation.ProviderRecommendationConfig{}, nil)
			detail, err := service.GetByID(context.Background(), "auth", 1)
			if stage == "user" {
				require.ErrorIs(t, err, conversation.ErrConversationAccessDenied)
			} else {
				require.ErrorIs(t, err, failure)
			}
			require.Nil(t, detail)
			reader.AssertExpectations(t)
			users.AssertExpectations(t)
			files.AssertExpectations(t)
		})
	}
}

func TestGetByIDPreservesUnsupportedTypeError(t *testing.T) {
	reader := &accessReaderMock{}
	reader.On("FindAccessByID", mock.Anything, 1).Return(&readmodel.ConversationAccess{ID: 1, Type: "unsupported"}, nil).Once()
	service := conversation.NewService(nil, nil, reader, nil, nil, nil, nil, nil, conversation.ProviderRecommendationConfig{}, nil)
	detail, err := service.GetByID(context.Background(), "auth", 1)
	require.ErrorContains(t, err, `unsupported conversation type "unsupported"`)
	require.NotErrorIs(t, err, conversation.ErrConversationAccessDenied)
	require.Nil(t, detail)
	reader.AssertExpectations(t)
}

func TestGetByIDPreservesChatbotAggregateReadFailure(t *testing.T) {
	failure := errors.New("chatbot aggregate read failed")
	repo := &conversationRepositoryMock{err: failure}
	reader := &accessReaderMock{}
	reader.On("FindAccessByID", mock.Anything, 7).Return(&readmodel.ConversationAccess{ID: 7, Type: conversation.TypeChatbot, ConsumerID: 10}, nil).Once()
	service := conversation.NewService(repo, nil, reader, nil, nil, nil, nil, nil, conversation.ProviderRecommendationConfig{}, nil)
	detail, err := service.GetByID(context.Background(), "auth", 7)
	require.ErrorIs(t, err, failure)
	require.Nil(t, detail)
	require.True(t, repo.findByIDCalled)
	reader.AssertExpectations(t)
	reader.AssertNotCalled(t, "FindDetailByIDRoleAndType", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
