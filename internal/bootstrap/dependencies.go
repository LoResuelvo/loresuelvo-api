package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	chatbotadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/chatbot"
	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/cryptography"
	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/category_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/consumer_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/conversation_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/file_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/job_request_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_account_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/service_proposal_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/test_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/user_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/work_order_handler"
	notificationadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/notification"
	mercadopagopayment "github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment/mercadopago"
	paymentaccountadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/realtime"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/scheduler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/google/uuid"
)

type Dependencies struct {
	Persistence              *PersistenceAdapters
	WorkOrderService         *workorder.Service
	UrgentWorkOrderScheduler *scheduler.Scheduler

	CategoryHandler        *category_handler.CategoryHandler
	ConsumerHandler        *consumer_handler.ConsumerHandler
	ProviderHandler        *provider_handler.ProviderHandler
	ConversationHandler    *conversation_handler.ConversationHandler
	JobRequestHandler      *job_request_handler.JobRequestHandler
	PaymentAccountHandler  *payment_account_handler.PaymentAccountHandler
	PaymentHandler         *payment_handler.PaymentHandler
	UserHandler            *user_handler.UserHandler
	FileHandler            *file_handler.FileHandler
	ServiceProposalHandler *service_proposal_handler.ServiceProposalHandler
	WorkOrderHandler       *work_order_handler.WorkOrderHandler
	TestHandler            *test_handler.TestHandler

	Hub              *realtime.Hub
	RealtimeHandler  *realtime.Handler
	MessagePublisher conversation.MessagePublisher

	Clock *clockadapter.SystemClock
}

func (dependencies *Dependencies) RouterConfig(auth0Validator *validator.Validator) httpadapter.RouterConfig {
	return httpadapter.RouterConfig{
		CategoryHandler:        dependencies.CategoryHandler,
		ConsumerHandler:        dependencies.ConsumerHandler,
		ProviderHandler:        dependencies.ProviderHandler,
		ConversationHandler:    dependencies.ConversationHandler,
		JobRequestHandler:      dependencies.JobRequestHandler,
		PaymentAccountHandler:  dependencies.PaymentAccountHandler,
		PaymentHandler:         dependencies.PaymentHandler,
		UserHandler:            dependencies.UserHandler,
		FileHandler:            dependencies.FileHandler,
		ServiceProposalHandler: dependencies.ServiceProposalHandler,
		WorkOrderHandler:       dependencies.WorkOrderHandler,
		TestHandler:            dependencies.TestHandler,
		RealtimeHandler:        dependencies.RealtimeHandler,
		Auth0Validator:         auth0Validator,
	}
}

func NewDependencies(database *sql.DB) (*Dependencies, error) {
	return NewDependenciesWithChatbot(database, chatbotadapter.NewChatbotFromEnv())
}

func NewDependenciesWithChatbot(database *sql.DB, chatbot conversation.Chatbot) (*Dependencies, error) {
	paymentAccountOAuthConnector, err := paymentaccountadapter.NewOAuthConnectorFromEnv()
	if err != nil {
		return nil, err
	}
	credentialCipher, err := cryptography.NewCredentialCipherFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring payment account credential encryption: %w", err)
	}
	paymentAccountHandlerConfig, err := payment_account_handler.NewConfigFromEnv()
	if err != nil {
		return nil, err
	}
	paymentGateway, err := mercadopagopayment.NewCheckoutClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring Mercado Pago checkout: %w", err)
	}
	webhookVerifier, err := mercadopagopayment.NewWebhookVerifierFromEnv()
	if err != nil {
		return nil, err
	}
	return NewDependenciesWithPaymentAccountAdapters(
		database,
		chatbot,
		paymentAccountOAuthConnector,
		paymentGateway,
		webhookVerifier,
		credentialCipher,
		cryptography.NewSecureSecretGenerator(),
		paymentAccountHandlerConfig,
	), nil
}

func NewDependenciesWithPaymentAccountAdapters(
	database *sql.DB,
	chatbot conversation.Chatbot,
	paymentAccountOAuthConnector paymentaccount.OAuthConnector,
	paymentGateway payment.Gateway,
	webhookVerifier payment_handler.WebhookVerifier,
	credentialProtector paymentaccount.CredentialProtector,
	secretGenerator paymentaccount.SecretGenerator,
	paymentAccountHandlerConfig payment_account_handler.Config,
) *Dependencies {
	persistence := NewPersistenceAdapters(database)

	storageComponents := storage.NewComponentsFromEnv()
	systemClock := clockadapter.NewSystemClock()

	// Realtime infrastructure
	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	ticketStore := realtime.NewTicketStore()

	messagePublisher := realtime.NewPublisher(hub, persistence.UserRepository)
	realtimeNotificationNotificator := realtime.NewNotificationNotificator(hub, persistence.UserRepository)
	notificator := notificationadapter.NewCompositeNotificator(realtimeNotificationNotificator)
	realtimeHandler := realtime.NewHandler(hub, persistence.UserRepository, ticketStore)

	fileService := filedomain.NewService(
		persistence.FileRepository,
		storageComponents.Storage,
		storageComponents.PublicBucket,
		storageComponents.PrivateBucket,
		systemClock,
	)
	categoryService := category.NewService(persistence.CategoryRepository)
	providerService := provider.NewService(persistence.UserRepository, persistence.CategoryRepository, fileService)
	consumerService := consumer.NewService(persistence.UserRepository, fileService)
	conversationService := conversation.NewService(
		persistence.ConversationRepository,
		persistence.UserRepository,
		persistence.ConversationReader,
		messagePublisher,
		chatbot,
		persistence.CategoryRepository,
		fileService,
		systemClock,
	)
	jobRequestService := jobrequest.NewService(
		persistence.JobRequestRepository,
		persistence.UserRepository,
		persistence.ConversationRepository,
		fileService,
	)
	userService := user.NewService(persistence.UserRepository, fileService)
	paymentAccountService := paymentaccount.NewService(
		persistence.UserRepository,
		persistence.AuthorizationAttemptRepository,
		persistence.PaymentAccountRepository,
		paymentAccountOAuthConnector,
		credentialProtector,
		secretGenerator,
		systemClock,
	)
	paymentService := payment.NewService(
		persistence.PaymentIntentRepository,
		persistence.ServiceProposalRepository,
		persistence.UserRepository,
		persistence.PaymentAccountRepository,
		credentialProtector,
		paymentGateway,
		paymentGateway,
		persistence.WorkOrderRepository,
		notificator,
		uuid.NewString,
		systemClock,
	)
	servicePorposalService := serviceproposal.NewService(
		persistence.ServiceProposalRepository,
		persistence.UserRepository,
		persistence.ConversationRepository,
		persistence.NotificationRepository,
		notificator,
		fileService,
		persistence.PaymentAccountRepository,
		paymentAccountOAuthConnector.Provider(),
		serviceproposal.NewBookingPolicy(),
		systemClock)
	workOrderService := workorder.NewService(
		persistence.WorkOrderRepository,
		persistence.UserRepository,
		fileService,
		persistence.NotificationRepository,
		notificator,
		systemClock,
	)
	urgentWorkOrderScheduler := scheduler.NewScheduler(time.Hour, workOrderService)
	_ = cancel // TODO: wire shutdown signal to cancel context

	return &Dependencies{
		Persistence:              persistence,
		WorkOrderService:         workOrderService,
		UrgentWorkOrderScheduler: urgentWorkOrderScheduler,
		CategoryHandler:          category_handler.NewCategoryHandler(categoryService),
		ConsumerHandler:          consumer_handler.NewConsumerHandler(consumerService),
		ProviderHandler:          provider_handler.NewProviderHandler(providerService),
		ConversationHandler:      conversation_handler.NewConversationHandler(conversationService),
		JobRequestHandler:        job_request_handler.NewJobRequestHandler(jobRequestService),
		PaymentAccountHandler:    payment_account_handler.NewPaymentAccountHandler(paymentAccountService, paymentAccountHandlerConfig),
		PaymentHandler:           payment_handler.NewPaymentHandler(paymentService, webhookVerifier),
		UserHandler:              user_handler.NewUserHandler(userService),
		FileHandler:              file_handler.NewFileHandler(fileService),
		ServiceProposalHandler:   service_proposal_handler.NewServiceProposalHandler(servicePorposalService),
		WorkOrderHandler:         work_order_handler.NewWorkOrderHandler(workOrderService),
		TestHandler:              test_handler.NewTestHandler(systemClock),
		Hub:                      hub,
		RealtimeHandler:          realtimeHandler,
		MessagePublisher:         messagePublisher,
		Clock:                    systemClock,
	}
}
