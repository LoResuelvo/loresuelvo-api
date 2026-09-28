package operation_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
)

type inboxReaderMock struct{ mock.Mock }

func (m *inboxReaderMock) FindPage(ctx context.Context, criteria operation.InboxCriteria) ([]readmodel.OperationSummary, error) {
	args := m.Called(ctx, criteria)
	if operations := args.Get(0); operations != nil {
		return operations.([]readmodel.OperationSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

type inboxFixedClock struct{ now time.Time }

func (clock inboxFixedClock) Now() time.Time { return clock.now }

type detailReaderStub struct {
	detail *readmodel.OperationDetail
	err    error
	calls  int
}

func (reader *detailReaderStub) FindByID(context.Context, readmodel.ID) (*readmodel.OperationDetail, error) {
	reader.calls++
	return reader.detail, reader.err
}

type detailOperatorStub struct {
	id    int
	err   error
	calls int
}

func (finder *detailOperatorStub) FindOperatorIDByAuthID(context.Context, string) (int, error) {
	finder.calls++
	return finder.id, finder.err
}

type detailAuditStub struct {
	err   error
	event *audit.Event
}

func (writer *detailAuditStub) Save(_ context.Context, event *audit.Event) error {
	writer.event = event
	return writer.err
}

type conversationAssociationReaderMock struct{ mock.Mock }

func (m *conversationAssociationReaderMock) FindConversationAssociation(ctx context.Context, id readmodel.ID) (*readmodel.ConversationAssociation, error) {
	args := m.Called(ctx, id)
	if found := args.Get(0); found != nil {
		return found.(*readmodel.ConversationAssociation), args.Error(1)
	}
	return nil, args.Error(1)
}

type messagePageReaderMock struct{ mock.Mock }

func (m *messagePageReaderMock) FindPage(ctx context.Context, id int, after *conversation.MessagePosition, limit int) ([]conversation.Message, error) {
	args := m.Called(ctx, id, after, limit)
	if found := args.Get(0); found != nil {
		return found.([]conversation.Message), args.Error(1)
	}
	return nil, args.Error(1)
}

type chatOperatorIDFinderMock struct{ mock.Mock }

func (m *chatOperatorIDFinderMock) FindOperatorIDByAuthID(ctx context.Context, subject string) (int, error) {
	args := m.Called(ctx, subject)
	return args.Int(0), args.Error(1)
}

type chatAuditWriterMock struct{ mock.Mock }

func (m *chatAuditWriterMock) Save(ctx context.Context, event *audit.Event) error {
	return m.Called(ctx, event).Error(0)
}

type chatAttachmentReaderMock struct{ mock.Mock }

func (m *chatAttachmentReaderMock) FindByMessagePage(ctx context.Context, scope conversation.MessageAttachmentScope) (map[int]conversation.MessageAttachmentReferences, error) {
	args := m.Called(ctx, scope)
	if value := args.Get(0); value != nil {
		return value.(map[int]conversation.MessageAttachmentReferences), args.Error(1)
	}
	return nil, args.Error(1)
}

type chatMediaResolverMock struct{ mock.Mock }

func (m *chatMediaResolverMock) ResolveMessageImages(ctx context.Context, ids []string) (map[string]filedomain.MessageImage, error) {
	args := m.Called(ctx, ids)
	if value := args.Get(0); value != nil {
		return value.(map[string]filedomain.MessageImage), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *chatMediaResolverMock) ResolveMessageAudios(ctx context.Context, ids []string) (map[string]filedomain.MessageAudio, error) {
	args := m.Called(ctx, ids)
	if value := args.Get(0); value != nil {
		return value.(map[string]filedomain.MessageAudio), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *chatMediaResolverMock) ResolveMessageVideos(ctx context.Context, ids []string) (map[string]filedomain.MessageVideo, error) {
	args := m.Called(ctx, ids)
	if value := args.Get(0); value != nil {
		return value.(map[string]filedomain.MessageVideo), args.Error(1)
	}
	return nil, args.Error(1)
}

// Text-only service scenarios share an attachment reader returning no references.
func newTextChatService(association operation.ConversationAssociationReader, messages conversation.MessagePageReader, operators audit.OperatorIDFinder, writer audit.Writer, clock inboxFixedClock) *operation.ChatService {
	attachments := &chatAttachmentReaderMock{}
	attachments.On("FindByMessagePage", mock.Anything, mock.Anything).Return(map[int]conversation.MessageAttachmentReferences{}, nil).Maybe()
	return operation.NewChatService(association, messages, operators, writer, clock, attachments, &chatMediaResolverMock{})
}
