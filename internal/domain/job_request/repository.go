package jobrequest

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Repository interface {
	SaveWithConversation(jobRequest JobRequest, pendingConversation conversation.Conversation) (*JobRequest, error)
	ExistsBetweenWithAnyStatus(consumerID, providerID int, statuses []Status) (bool, error)
	FindByUserAuthID(userAuthID string) ([]readmodel.JobRequestSummary, error)
	FindByID(id int) (*JobRequest, error)
	SaveStatus(ctx context.Context, jobRequest JobRequest) error
}

type UserRepository interface {
	FindIDByAuthID(authID string) (int, error)
	ExistsProviderByID(id int) (bool, error)
	FindProviderByID(ctx context.Context, id int) (*provider.Provider, error)
}

type ConversationRepository interface {
	FindByID(ctx context.Context, conversationID int) (conversation.Conversation, error)
	SaveConversation(ctx context.Context, conversation conversation.Conversation) (conversation.Conversation, error)
}
