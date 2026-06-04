package bootstrap

import (
	"context"
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/realtime"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
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
	JobRequestRepository   *repositories.JobRequestRepository
	ConversationReader     *repositories.ConversationReader

	CategoryHandler     *handler.CategoryHandler
	ConsumerHandler     *handler.ConsumerHandler
	ProviderHandler     *handler.ProviderHandler
	ConversationHandler *handler.ConversationHandler
	JobRequestHandler   *handler.JobRequestHandler
	UserHandler         *handler.UserHandler

	Hub              *realtime.Hub
	RealtimeHandler  *realtime.Handler
	MessagePublisher conversation.MessagePublisher
}

func NewDependencies(database *sql.DB) *Dependencies {
	userRepository := repositories.NewUserRepository(database)
	categoryRepository := repositories.NewCategoryRepository(database)
	consumerRepository := repositories.NewConsumerRepository(database, userRepository)
	providerRepository := repositories.NewProviderRepository(database, userRepository)
	messageRepository := repositories.NewMessageRepository(database)
	conversationRepository := repositories.NewConversationRepository(database, messageRepository)
	jobRequestRepository := repositories.NewJobRequestRepository(database)
	conversationReader := repositories.NewConversationReader(database)

	// Realtime infrastructure
	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	messagePublisher := realtime.NewPublisher(hub, consumerRepository, providerRepository)
	realtimeHandler := realtime.NewHandler(hub, consumerRepository, providerRepository)

	categoryService := category.NewService(categoryRepository)
	providerService := provider.NewService(providerRepository, categoryRepository)
	consumerService := consumer.NewService(consumerRepository)
	conversationService := conversation.NewService(
		conversationRepository,
		consumerRepository,
		providerRepository,
		providerRepository,
		conversationReader,
		messagePublisher,
	)
	jobRequestService := jobrequest.NewService(
		jobRequestRepository,
		consumerRepository,
		providerRepository,
		conversationRepository,
	)
	userService := user.NewService(userRepository)

	_ = cancel // TODO: wire shutdown signal to cancel context

	return &Dependencies{
		UserRepository:         userRepository,
		CategoryRepository:     categoryRepository,
		ConsumerRepository:     consumerRepository,
		ProviderRepository:     providerRepository,
		ConversationRepository: conversationRepository,
		MessageRepository:      messageRepository,
		JobRequestRepository:   jobRequestRepository,
		ConversationReader:     conversationReader,
		CategoryHandler:        handler.NewCategoryHandler(categoryService),
		ConsumerHandler:        handler.NewConsumerHandler(consumerService),
		ProviderHandler:        handler.NewProviderHandler(providerService),
		ConversationHandler:    handler.NewConversationHandler(conversationService),
		JobRequestHandler:      handler.NewJobRequestHandler(jobRequestService),
		UserHandler:            handler.NewUserHandler(userService),
		Hub:                    hub,
		RealtimeHandler:        realtimeHandler,
		MessagePublisher:       messagePublisher,
	}
}
