package operation_test

import (
	"context"
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestChatServiceReturnsOnlyAfterAuditSucceeds(t *testing.T) {
	for _, auditFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "prepared", true: "fail closed"}[auditFails], func(t *testing.T) {
			ctx := context.Background()
			association := &conversationAssociationReaderMock{}
			messages := &messagePageReaderMock{}
			operators := &chatOperatorIDFinderMock{}
			writer := &chatAuditWriterMock{}
			requestID := 7
			found := &readmodel.ConversationAssociation{ConversationID: 21, ConsumerID: 11, ProviderID: 12, JobRequestID: &requestID}
			page := []conversation.Message{{ID: 31, ConversationID: 21, SenderRole: conversation.SenderConsumer, Content: "Help"}}
			association.On("FindConversationAssociation", ctx, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 7}).Return(found, nil).Once()
			operators.On("FindOperatorIDByAuthID", ctx, "subject").Return(23, nil).Once()
			messages.On("FindPage", ctx, 21, (*conversation.MessagePosition)(nil), 21).Return(page, nil).Once()
			now := time.Date(2026, 9, 28, 1, 0, 0, 0, time.UTC)
			failure := errors.New("audit unavailable")
			var saveErr error
			if auditFails {
				saveErr = failure
			}
			writer.On("Save", ctx, mock.MatchedBy(func(event *audit.Event) bool {
				return event.OperatorID() == 23 && event.Action() == audit.ActionAccess && event.ResourceType() == "job_request" && event.ResourceID() == "7" && event.ConversationID() != nil && *event.ConversationID() == 21 && event.Reason().Text() == "Support case" && event.Result() == audit.ResultPrepared && event.CorrelationID() == "request-1" && event.OccurredOn().Equal(now) && event.StateChange() == nil
			})).Return(saveErr).Once()
			service := operation.NewChatService(association, messages, operators, writer, inboxFixedClock{now: now})
			result, err := service.Query(ctx, "jr-7", "subject", "request-1", " Support case ", operation.ChatQuery{})
			if auditFails {
				require.ErrorIs(t, err, failure)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, page, result.Messages)
				require.Equal(t, found.ConversationID, result.ConversationID)
				require.Equal(t, &requestID, result.JobRequestID)
			}
			association.AssertExpectations(t)
			operators.AssertExpectations(t)
			messages.AssertExpectations(t)
			writer.AssertExpectations(t)
		})
	}
}

func TestChatServiceRejectsInvalidInputBeforeReading(t *testing.T) {
	for _, tc := range []struct {
		id, reason string
		expected   error
	}{{"jr-0", "Support", operation.ErrInvalidOperationID}, {"jr-1", " ", operation.ErrInvalidChatReason}} {
		association := &conversationAssociationReaderMock{}
		messages := &messagePageReaderMock{}
		operators := &chatOperatorIDFinderMock{}
		writer := &chatAuditWriterMock{}
		service := operation.NewChatService(association, messages, operators, writer, inboxFixedClock{now: time.Now()})
		result, err := service.Query(context.Background(), tc.id, "subject", "request-1", tc.reason, operation.ChatQuery{})
		require.Nil(t, result)
		require.ErrorIs(t, err, tc.expected)
		association.AssertNotCalled(t, "FindConversationAssociation", mock.Anything, mock.Anything)
		writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	}
}

func TestChatServiceStopsBeforeAuditOnMissingAssociationOrReadFailure(t *testing.T) {
	for _, stage := range []string{"missing", "association", "operator", "messages"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			association := &conversationAssociationReaderMock{}
			messages := &messagePageReaderMock{}
			operators := &chatOperatorIDFinderMock{}
			writer := &chatAuditWriterMock{}
			found := &readmodel.ConversationAssociation{ConversationID: 21, ConsumerID: 11, ProviderID: 12}
			failure := errors.New("dependency unavailable")
			expected := failure
			var associationErr error
			if stage == "missing" {
				found = nil
				expected = operation.ErrOperationNotFound
			}
			if stage == "association" {
				associationErr = failure
			}
			association.On("FindConversationAssociation", ctx, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 7}).Return(found, associationErr).Once()
			if stage == "operator" {
				operators.On("FindOperatorIDByAuthID", ctx, "subject").Return(0, failure).Once()
			}
			if stage == "messages" {
				operators.On("FindOperatorIDByAuthID", ctx, "subject").Return(23, nil).Once()
				messages.On("FindPage", ctx, 21, (*conversation.MessagePosition)(nil), 21).Return(nil, failure).Once()
			}
			service := operation.NewChatService(association, messages, operators, writer, inboxFixedClock{now: time.Now()})
			result, err := service.Query(ctx, "jr-7", "subject", "request-1", "Support case", operation.ChatQuery{})
			require.Nil(t, result)
			require.ErrorIs(t, err, expected)
			association.AssertExpectations(t)
			operators.AssertExpectations(t)
			messages.AssertExpectations(t)
			writer.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
		})
	}
}

func TestChatServiceAuditsEachKeysetPageAndSuppressesLookahead(t *testing.T) {
	ctx := context.Background()
	association := &conversationAssociationReaderMock{}
	messages := &messagePageReaderMock{}
	operators := &chatOperatorIDFinderMock{}
	writer := &chatAuditWriterMock{}
	found := &readmodel.ConversationAssociation{ConversationID: 21, ConsumerID: 11, ProviderID: 12}
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	page := []conversation.Message{{ID: 1, CreatedOn: now}, {ID: 2, CreatedOn: now}, {ID: 3, CreatedOn: now}}
	position := &conversation.MessagePosition{ID: 2, CreatedOn: now}
	association.On("FindConversationAssociation", ctx, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 7}).Return(found, nil).Twice()
	operators.On("FindOperatorIDByAuthID", ctx, "subject").Return(23, nil).Twice()
	messages.On("FindPage", ctx, 21, (*conversation.MessagePosition)(nil), 3).Return(page, nil).Once()
	messages.On("FindPage", ctx, 21, position, 3).Return(page[2:], nil).Once()
	for _, correlation := range []string{"request-page-1", "request-page-2"} {
		writer.On("Save", ctx, mock.MatchedBy(func(event *audit.Event) bool {
			return event.CorrelationID() == correlation && event.Reason().Text() == "Support case" && *event.ConversationID() == 21
		})).Return(nil).Once()
	}
	service := operation.NewChatService(association, messages, operators, writer, inboxFixedClock{now: now})
	first, err := service.Query(ctx, "jr-7", "subject", "request-page-1", "Support case", operation.ChatQuery{Limit: 2})
	require.NoError(t, err)
	require.Equal(t, page[:2], first.Messages)
	require.Equal(t, position, first.Next)
	second, err := service.Query(ctx, "jr-7", "subject", "request-page-2", "Support case", operation.ChatQuery{Limit: 2, ConversationID: 21, After: first.Next})
	require.NoError(t, err)
	require.Equal(t, page[2:], second.Messages)
	require.Nil(t, second.Next)
	association.AssertExpectations(t)
	operators.AssertExpectations(t)
	messages.AssertExpectations(t)
	writer.AssertExpectations(t)
}

func TestChatServiceRechecksPersistedConversationOnContinuation(t *testing.T) {
	association := &conversationAssociationReaderMock{}
	messages := &messagePageReaderMock{}
	operators := &chatOperatorIDFinderMock{}
	writer := &chatAuditWriterMock{}
	association.On("FindConversationAssociation", mock.Anything, readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 7}).Return(&readmodel.ConversationAssociation{ConversationID: 22}, nil).Once()
	service := operation.NewChatService(association, messages, operators, writer, inboxFixedClock{now: time.Now()})
	result, err := service.Query(context.Background(), "jr-7", "subject", "request-1", "Support case", operation.ChatQuery{Limit: 2, ConversationID: 21, After: &conversation.MessagePosition{ID: 1, CreatedOn: time.Now()}})
	require.Nil(t, result)
	require.ErrorIs(t, err, operation.ErrInvalidChatQuery)
	association.AssertExpectations(t)
	require.Empty(t, messages.Calls)
	require.Empty(t, operators.Calls)
	require.Empty(t, writer.Calls)
}
