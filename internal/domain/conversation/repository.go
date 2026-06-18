package conversation

import (
	"context"

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

type ConsumerIDFinder interface {
	FindIDByAuthID(authID string) (int, error)
	FindAuthIDByID(id int) (string, error)
}

type ProviderIDFinder interface {
	FindIDByAuthID(authID string) (int, error)
	FindAuthIDByID(id int) (string, error)
}

type ProviderRepository interface {
	ProviderIDFinder
	FindByCategoryID(categoryID int) ([]provider.Provider, error)
}

type Reader interface {
	FindSummariesByParticipantIDRoleAndType(ctx context.Context, participantID int, participantRole string, conversationType string) ([]readmodel.ConversationSummary, error)
	FindDetailByIDRoleAndType(ctx context.Context, conversationID int, participantRole string, conversationType string) (*readmodel.ConversationDetail, error)
}
