package conversation

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type Repository interface {
	SaveConversation(ctx context.Context, conversation Conversation) (Conversation, error)
	FindByID(ctx context.Context, conversationID int) (Conversation, error)
	CountMessagesBySenderRole(ctx context.Context, conversationID int, senderRole string) (int, error)
}

type UserRepository interface {
	FindByAuthID(authID string) (user.User, error)
	FindConsumerByAuthID(ctx context.Context, authID string) (*consumer.Consumer, error)
	FindProviderByID(ctx context.Context, providerID int) (*provider.Provider, error)
	FindProvidersByCategoryAndCoverageZoneID(ctx context.Context, categoryID, coverageZoneID int) ([]provider.Provider, error)
}

type Reader interface {
	FindSummariesByUserAndType(ctx context.Context, user user.User, conversationType string) ([]readmodel.ConversationSummary, error)
	FindDetailByIDRoleAndType(ctx context.Context, conversationID int, participantRole string, conversationType string) (*readmodel.ConversationDetail, error)
}
