package operation

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/google/uuid"
)

var ErrInvalidChatReason = errors.New("invalid conversation access reason")

const DefaultChatPageSize = 20

type ConversationAssociationReader interface {
	FindConversationAssociation(ctx context.Context, id readmodel.ID) (*readmodel.ConversationAssociation, error)
}

type ChatService struct {
	association ConversationAssociationReader
	messages    conversation.MessagePageReader
	operatorIDs audit.OperatorIDFinder
	auditWriter audit.Writer
	clock       clock.Clock
}

func NewChatService(association ConversationAssociationReader, messages conversation.MessagePageReader, operatorIDs audit.OperatorIDFinder, auditWriter audit.Writer, clock clock.Clock) *ChatService {
	return &ChatService{association: association, messages: messages, operatorIDs: operatorIDs, auditWriter: auditWriter, clock: clock}
}

func (service *ChatService) Query(ctx context.Context, rawID, authSubject, correlationID, reasonText string) (*readmodel.OperationChat, error) {
	id, err := ParseOperationID(rawID)
	if err != nil {
		return nil, err
	}
	reason, err := audit.NewReason(reasonText)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidChatReason, err)
	}
	association, err := service.association.FindConversationAssociation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding operation conversation association: %w", err)
	}
	if association == nil {
		return nil, ErrOperationNotFound
	}
	operatorID, err := service.operatorIDs.FindOperatorIDByAuthID(ctx, authSubject)
	if err != nil {
		return nil, fmt.Errorf("finding operation conversation reader: %w", err)
	}
	messages, err := service.messages.FindPage(ctx, association.ConversationID, DefaultChatPageSize)
	if err != nil {
		return nil, fmt.Errorf("reading operation conversation messages: %w", err)
	}
	resourceType := "job_request"
	if id.Kind == readmodel.KindServiceProposal {
		resourceType = "service_proposal"
	}
	event, err := audit.NewEvent(audit.EventParams{
		ID: uuid.New(), OperatorID: operatorID, Action: audit.ActionAccess,
		ResourceType: resourceType, ResourceID: strconv.Itoa(id.ResourceID),
		ConversationID: &association.ConversationID, Reason: reason,
		OccurredOn: service.clock.Now(), Result: audit.ResultPrepared, CorrelationID: correlationID,
	})
	if err != nil {
		return nil, fmt.Errorf("creating operation conversation access event: %w", err)
	}
	if err := service.auditWriter.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("saving operation conversation access event: %w", err)
	}
	return &readmodel.OperationChat{OperationID: id, ConversationAssociation: *association, Messages: messages}, nil
}
