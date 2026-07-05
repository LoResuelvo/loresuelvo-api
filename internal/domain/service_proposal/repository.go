package serviceproposal

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type ConversationRepository interface {
	FindBetween(consumerID int, providerID int) (conversation.Conversation, error)
}

type ServiceProposalRepository interface {
	Save(serviceProposal *ServiceProposal) (*ServiceProposal, error)
	FindByUserID(ctx context.Context, userID int) ([]*ServiceProposal, error)
}

type UserRepository interface {
	FindProviderByAuthID(auth0ID string) (*provider.Provider, error)
	FindConsumerByID(consumerID int) (*consumer.Consumer, error)
	FindByAuthID(auth0ID string) (user.User, error)
}

type NotificationRepository interface {
	Save(ctx context.Context, notification *notification.Notification) (*notification.Notification, error)
}

type FileURLResolver interface {
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}
