package conversation_test

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/mock"
)

type conversationReaderMock struct {
	mock.Mock
	consumerSummaries         []readmodel.ConversationSummary
	providerSummaries         []readmodel.ConversationSummary
	chatbotSummaries          []readmodel.ConversationSummary
	detail                    *readmodel.ConversationDetail
	requestedParticipantID    int
	requestedParticipantRole  string
	requestedConversationType string
	requestedConversationID   int
	err                       error
}

func (m *conversationReaderMock) FindSummariesByParticipantIDRoleAndType(ctx context.Context, participantID int, participantRole string, conversationType string) ([]readmodel.ConversationSummary, error) {
	m.requestedParticipantID = participantID
	m.requestedParticipantRole = participantRole
	m.requestedConversationType = conversationType
	if m.err != nil {
		return nil, m.err
	}
	if conversationType == conversation.TypeChatbot {
		return m.chatbotSummaries, nil
	}
	if participantRole == conversation.SenderProvider {
		return m.providerSummaries, nil
	}
	return m.consumerSummaries, nil
}

func (m *conversationReaderMock) FindSummariesByUserAndType(ctx context.Context, foundUser user.User, conversationType string) ([]readmodel.ConversationSummary, error) {
	return m.FindSummariesByParticipantIDRoleAndType(ctx, foundUser.ID(), foundUser.Role(), conversationType)
}

func (m *conversationReaderMock) FindDetailByIDRoleAndType(ctx context.Context, conversationID int, participantRole string, conversationType string) (*readmodel.ConversationDetail, error) {
	m.requestedConversationID = conversationID
	m.requestedParticipantRole = participantRole
	m.requestedConversationType = conversationType
	if m.err != nil {
		return nil, m.err
	}
	return m.detail, nil
}

func (m *conversationReaderMock) FindAccessByID(ctx context.Context, id int) (*readmodel.ConversationAccess, error) {
	args := m.Called(ctx, id)
	access, _ := args.Get(0).(*readmodel.ConversationAccess)
	return access, args.Error(1)
}

// accessReaderMock leaves unused capabilities absent so unexpected reads fail.
type accessReaderMock struct {
	conversation.Reader
	mock.Mock
}

func (m *accessReaderMock) FindAccessByID(ctx context.Context, id int) (*readmodel.ConversationAccess, error) {
	args := m.Called(ctx, id)
	access, _ := args.Get(0).(*readmodel.ConversationAccess)
	return access, args.Error(1)
}
func (m *accessReaderMock) FindDetailByIDRoleAndType(ctx context.Context, id int, role, kind string) (*readmodel.ConversationDetail, error) {
	args := m.Called(ctx, id, role, kind)
	detail, _ := args.Get(0).(*readmodel.ConversationDetail)
	return detail, args.Error(1)
}

type participantUserRepositoryMock struct {
	conversation.UserRepository
	mock.Mock
}

func (m *participantUserRepositoryMock) FindByAuthID(authID string) (user.User, error) {
	args := m.Called(authID)
	foundUser, _ := args.Get(0).(user.User)
	return foundUser, args.Error(1)
}

type detailFileServiceMock struct {
	conversation.FileService
	mock.Mock
}

func (m *detailFileServiceMock) ResolvePublicURL(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *detailFileServiceMock) ResolveMessageImages(ctx context.Context, ids []string) (map[string]filedomain.MessageImage, error) {
	args := m.Called(ctx, ids)
	files, _ := args.Get(0).(map[string]filedomain.MessageImage)
	return files, args.Error(1)
}
