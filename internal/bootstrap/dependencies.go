package bootstrap

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type Dependencies struct {
	UserRepository         *repositories.UserRepository
	CategoryRepository     *repositories.CategoryRepository
	ConsumerRepository     *repositories.ConsumerRepository
	ProviderRepository     *repositories.ProviderRepository
	ConversationRepository *repositories.ConversationRepository
	MessageRepository      *repositories.MessageRepository

	CategoryHandler     *handler.CategoryHandler
	ConsumerHandler     *handler.ConsumerHandler
	ProviderHandler     *handler.ProviderHandler
	ConversationHandler *handler.ConversationHandler
	UserHandler         *handler.UserHandler
}

func NewDependencies(database *sql.DB) *Dependencies {
	userRepository := repositories.NewUserRepository(database)
	categoryRepository := repositories.NewCategoryRepository(database)
	consumerRepository := repositories.NewConsumerRepository(database, userRepository)
	providerRepository := repositories.NewProviderRepository(database, userRepository)
	messageRepository := repositories.NewMessageRepository(database)
	conversationRepository := repositories.NewConversationRepository(database, messageRepository)

	categoryService := category.NewService(categoryRepository)
	providerService := provider.NewService(providerRepository, categoryRepository)
	consumerService := consumer.NewService(consumerRepository)
	conversationService := conversation.NewService(conversationRepository, consumerRepository, providerRepository)
	userService := user.NewService(userRepository)

	return &Dependencies{
		UserRepository:         userRepository,
		CategoryRepository:     categoryRepository,
		ConsumerRepository:     consumerRepository,
		ProviderRepository:     providerRepository,
		ConversationRepository: conversationRepository,
		MessageRepository:      messageRepository,
		CategoryHandler:        handler.NewCategoryHandler(categoryService),
		ConsumerHandler:        handler.NewConsumerHandler(consumerService),
		ProviderHandler:        handler.NewProviderHandler(providerService),
		ConversationHandler:    handler.NewConversationHandler(conversationService),
		UserHandler:            handler.NewUserHandler(userService),
	}
}
