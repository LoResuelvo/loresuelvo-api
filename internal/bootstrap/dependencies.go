package bootstrap

import (
	"context"
	"database/sql"

	chatbotadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/chatbot"
	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/category_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/consumer_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/conversation_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/file_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/job_request_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/test_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/user_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/realtime"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
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
	MessageImageRepository *repositories.MessageImageRepository
	JobRequestRepository   *repositories.JobRequestRepository
	ConversationReader     *repositories.ConversationReader
	FileRepository         *repositories.FileRepository

	CategoryHandler     *category_handler.CategoryHandler
	ConsumerHandler     *consumer_handler.ConsumerHandler
	ProviderHandler     *provider_handler.ProviderHandler
	ConversationHandler *conversation_handler.ConversationHandler
	JobRequestHandler   *job_request_handler.JobRequestHandler
	UserHandler         *user_handler.UserHandler
	FileHandler         *file_handler.FileHandler
	TestHandler         *test_handler.TestHandler

	Hub              *realtime.Hub
	RealtimeHandler  *realtime.Handler
	MessagePublisher conversation.MessagePublisher

	Clock *clockadapter.SystemClock
}

func NewDependencies(database *sql.DB) *Dependencies {
	return NewDependenciesWithChatbot(database, chatbotadapter.NewChatbotFromEnv())
}

func NewDependenciesWithChatbot(database *sql.DB, chatbot conversation.Chatbot) *Dependencies {
	clockadapter := clockadapter.NewSystemClock()
	userRepository := repositories.NewUserRepository(database)
	categoryRepository := repositories.NewCategoryRepository(database)
	consumerRepository := repositories.NewConsumerRepository(database, userRepository)
	providerRepository := repositories.NewProviderRepository(database, userRepository)
	messageImageRepository := repositories.NewMessageImageRepository(database)
	messageRepository := repositories.NewMessageRepository(database, messageImageRepository)
	conversationRepository := repositories.NewConversationRepository(database, messageRepository)
	jobRequestRepository := repositories.NewJobRequestRepository(database)
	conversationReader := repositories.NewConversationReader(database, messageImageRepository)
	fileRepository := repositories.NewFileRepository(database)

	storageConfig := storage.NewConfigFromEnv()
	fileStorage := storage.NewStorageFromConfig(storageConfig)
	fileService := filedomain.NewService(fileRepository, fileStorage, storageConfig.PublicBucket, storageConfig.PrivateBucket, clockadapter)

	// Realtime infrastructure
	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	ticketStore := realtime.NewTicketStore()

	messagePublisher := realtime.NewPublisher(hub, consumerRepository, providerRepository)
	realtimeHandler := realtime.NewHandler(hub, consumerRepository, providerRepository, ticketStore)

	categoryService := category.NewService(categoryRepository)
	providerService := provider.NewService(providerRepository, categoryRepository, fileService)
	consumerService := consumer.NewService(consumerRepository)
	conversationService := conversation.NewService(
		conversationRepository,
		consumerRepository,
		providerRepository,
		conversationReader,
		messagePublisher,
		chatbot,
		categoryRepository,
		fileService,
		clockadapter,
	)
	jobRequestService := jobrequest.NewService(
		jobRequestRepository,
		consumerRepository,
		providerRepository,
		conversationRepository,
		fileService,
	)
	userService := user.NewService(userRepository)

	testHandler := test_handler.NewTestHandler(clockadapter)

	_ = cancel // TODO: wire shutdown signal to cancel context

	return &Dependencies{
		UserRepository:         userRepository,
		CategoryRepository:     categoryRepository,
		ConsumerRepository:     consumerRepository,
		ProviderRepository:     providerRepository,
		ConversationRepository: conversationRepository,
		MessageRepository:      messageRepository,
		MessageImageRepository: messageImageRepository,
		JobRequestRepository:   jobRequestRepository,
		ConversationReader:     conversationReader,
		FileRepository:         fileRepository,
		CategoryHandler:        category_handler.NewCategoryHandler(categoryService),
		ConsumerHandler:        consumer_handler.NewConsumerHandler(consumerService),
		ProviderHandler:        provider_handler.NewProviderHandler(providerService),
		ConversationHandler:    conversation_handler.NewConversationHandler(conversationService),
		JobRequestHandler:      job_request_handler.NewJobRequestHandler(jobRequestService),
		UserHandler:            user_handler.NewUserHandler(userService),
		FileHandler:            file_handler.NewFileHandler(fileService),
		TestHandler:            testHandler,
		Hub:                    hub,
		RealtimeHandler:        realtimeHandler,
		MessagePublisher:       messagePublisher,
		Clock:                  clockadapter,
	}
}
