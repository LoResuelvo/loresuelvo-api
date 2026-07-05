package conversation

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Repository interface {
	SaveConversation(ctx context.Context, conversation Conversation) (Conversation, error)
	FindByID(ctx context.Context, conversationID int) (Conversation, error)
	UpdateConversation(ctx context.Context, conversation Conversation) (Conversation, error)
	AddMessage(ctx context.Context, conversationID int, message Message) (*Message, error)
	CountMessagesBySenderRole(ctx context.Context, conversationID int, senderRole string) (int, error)
}

type UserRepository interface {
	FindByAuthID(authID string) (user.User, error)
	FindProvidersByCategoryID(categoryID int) ([]provider.Provider, error)
}

type Reader interface {
	FindSummariesByUserAndType(ctx context.Context, user user.User, conversationType string) ([]readmodel.ConversationSummary, error)
	FindDetailByIDRoleAndType(ctx context.Context, conversationID int, participantRole string, conversationType string) (*readmodel.ConversationDetail, error)
}
