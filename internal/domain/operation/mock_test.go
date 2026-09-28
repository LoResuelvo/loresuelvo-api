package operation_test

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
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

func (m *messagePageReaderMock) FindPage(ctx context.Context, id, limit int) ([]conversation.Message, error) {
	args := m.Called(ctx, id, limit)
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
