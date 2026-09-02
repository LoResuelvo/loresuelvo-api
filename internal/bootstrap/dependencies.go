package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	chatbotadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/chatbot"
	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/cryptography"
	googlecalendar "github.com/LoResuelvo/loresuelvo-api/internal/adapters/google_calendar"
	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/calendar_connection_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/category_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/consumer_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/conversation_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/coverage_zone_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/file_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/identity_verification_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/job_request_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_account_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/service_proposal_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/test_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/user_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/work_order_handler"
	didit "github.com/LoResuelvo/loresuelvo-api/internal/adapters/identityverification/didit"
	identityverificationfake "github.com/LoResuelvo/loresuelvo-api/internal/adapters/identityverification/fake"
	locationadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/location"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/locking"
	mediaadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/media"
	notificationadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/notification"
	mercadopagopayment "github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment/mercadopago"
	paymentaccountadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/realtime"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/scheduler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/google/uuid"
)

type Dependencies struct {
	Persistence              *PersistenceAdapters
	WorkOrderService         *workorder.Service
	UrgentWorkOrderScheduler *scheduler.Scheduler
	CalendarSyncRunner       *scheduler.CalendarSyncRunner
	CalendarEventObserver    CalendarEventObserver

	CategoryHandler             *category_handler.CategoryHandler
	CalendarConnectionHandler   *calendar_connection_handler.CalendarConnectionHandler
	CoverageZoneHandler         *coverage_zone_handler.CoverageZoneHandler
	ConsumerHandler             *consumer_handler.ConsumerHandler
	ProviderHandler             *provider_handler.ProviderHandler
	ConversationHandler         *conversation_handler.ConversationHandler
	JobRequestHandler           *job_request_handler.JobRequestHandler
	IdentityVerificationHandler *identity_verification_handler.IdentityVerificationHandler
	PaymentAccountHandler       *payment_account_handler.PaymentAccountHandler
	PaymentHandler              *payment_handler.PaymentHandler
	UserHandler                 *user_handler.UserHandler
	FileHandler                 *file_handler.FileHandler
	ServiceProposalHandler      *service_proposal_handler.ServiceProposalHandler
	WorkOrderHandler            *work_order_handler.WorkOrderHandler
	TestHandler                 *test_handler.TestHandler

	Hub              *realtime.Hub
	RealtimeHandler  *realtime.Handler
	MessagePublisher conversation.MessagePublisher

	Clock *clockadapter.SystemClock

	ConsumerAddressResolver consumer.AddressResolver
	IdentityVerifier        identityverification.IdentityVerifier
}

type CalendarEventObserver interface {
	HasEventForUser(context.Context, int, int) (bool, error)
}

func (dependencies *Dependencies) RouterConfig(auth0Validator *validator.Validator, logger *slog.Logger) httpadapter.RouterConfig {
	return httpadapter.RouterConfig{
		CategoryHandler:             dependencies.CategoryHandler,
		CalendarConnectionHandler:   dependencies.CalendarConnectionHandler,
		CoverageZoneHandler:         dependencies.CoverageZoneHandler,
		ConsumerHandler:             dependencies.ConsumerHandler,
		ProviderHandler:             dependencies.ProviderHandler,
		ConversationHandler:         dependencies.ConversationHandler,
		JobRequestHandler:           dependencies.JobRequestHandler,
		IdentityVerificationHandler: dependencies.IdentityVerificationHandler,
		PaymentAccountHandler:       dependencies.PaymentAccountHandler,
		PaymentHandler:              dependencies.PaymentHandler,
		UserHandler:                 dependencies.UserHandler,
		FileHandler:                 dependencies.FileHandler,
		ServiceProposalHandler:      dependencies.ServiceProposalHandler,
		WorkOrderHandler:            dependencies.WorkOrderHandler,
		TestHandler:                 dependencies.TestHandler,
		RealtimeHandler:             dependencies.RealtimeHandler,
		Auth0Validator:              auth0Validator,
		Logger:                      logger,
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
	calendarOAuthConnector, err := googlecalendar.NewOAuthConnectorFromEnv()
	if err != nil {
		return nil, err
	}
	calendarCredentialCipher, err := cryptography.NewCalendarCredentialCipherFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring calendar credential encryption: %w", err)
	}
	calendarEventPublisher, err := googlecalendar.NewEventPublisherFromEnv(calendarCredentialCipher)
	if err != nil {
		return nil, err
	}
	calendarHandlerConfig := calendar_connection_handler.NewConfigFromEnv()
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
	identityVerifier, err := didit.NewClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring identity verifier: %w", err)
	}
	identityWebhook, err := didit.NewWebhookAdapterFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring identity verification webhook: %w", err)
	}
	return NewDependenciesWithPaymentAccountAndCalendarAdapters(
		database,
		chatbot,
		paymentAccountOAuthConnector,
		paymentGateway,
		webhookVerifier,
		credentialCipher,
		cryptography.NewSecureSecretGenerator(),
		paymentAccountHandlerConfig,
		calendarOAuthConnector,
		calendarCredentialCipher,
		calendarEventPublisher,
		calendarHandlerConfig,
		identityVerifier,
		identityWebhook,
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
	return newDependenciesWithPaymentAccountAndCalendarAdapters(
		database,
		chatbot,
		paymentAccountOAuthConnector,
		paymentGateway,
		webhookVerifier,
		credentialProtector,
		secretGenerator,
		paymentAccountHandlerConfig,
		googlecalendar.NewFakeOAuthClient(),
		credentialProtector,
		googlecalendar.NewFakeEventPublisher(),
		calendar_connection_handler.Config{
			ConnectionSuccessURL:   "/me",
			ConnectionCancelledURL: "/me",
		},
		identityverificationfake.NewVerifier(),
		true,
		newTestIdentityVerificationWebhook(),
	)
}

func NewDependenciesWithPaymentAccountAndCalendarAdapters(
	database *sql.DB,
	chatbot conversation.Chatbot,
	paymentAccountOAuthConnector paymentaccount.OAuthConnector,
	paymentGateway payment.Gateway,
	webhookVerifier payment_handler.WebhookVerifier,
	credentialProtector paymentaccount.CredentialProtector,
	secretGenerator paymentaccount.SecretGenerator,
	paymentAccountHandlerConfig payment_account_handler.Config,
	calendarOAuthConnector calendarconnection.OAuthConnector,
	calendarCredentialProtector calendarconnection.CredentialProtector,
	calendarEventPublisher workordercalendar.EventPublisher,
	calendarHandlerConfig calendar_connection_handler.Config,
	identityVerifier identityverification.IdentityVerifier,
	identityWebhooks ...identity_verification_handler.IdentityVerificationWebhook,
) *Dependencies {
	return newDependenciesWithPaymentAccountAndCalendarAdapters(
		database,
		chatbot,
		paymentAccountOAuthConnector,
		paymentGateway,
		webhookVerifier,
		credentialProtector,
		secretGenerator,
		paymentAccountHandlerConfig,
		calendarOAuthConnector,
		calendarCredentialProtector,
		calendarEventPublisher,
		calendarHandlerConfig,
		identityVerifier,
		false,
		identityWebhooks...,
	)
}

func newDependenciesWithPaymentAccountAndCalendarAdapters(
	database *sql.DB,
	chatbot conversation.Chatbot,
	paymentAccountOAuthConnector paymentaccount.OAuthConnector,
	paymentGateway payment.Gateway,
	webhookVerifier payment_handler.WebhookVerifier,
	credentialProtector paymentaccount.CredentialProtector,
	secretGenerator paymentaccount.SecretGenerator,
	paymentAccountHandlerConfig payment_account_handler.Config,
	calendarOAuthConnector calendarconnection.OAuthConnector,
	calendarCredentialProtector calendarconnection.CredentialProtector,
	calendarEventPublisher workordercalendar.EventPublisher,
	calendarHandlerConfig calendar_connection_handler.Config,
	identityVerifier identityverification.IdentityVerifier,
	useFakeLocationResolvers bool,
	identityWebhooks ...identity_verification_handler.IdentityVerificationWebhook,
) *Dependencies {
	persistence := NewPersistenceAdapters(database)

	var addressResolver consumer.AddressResolver
	var coverageZoneResolver consumer.CoverageZoneResolver
	if useFakeLocationResolvers {
		addressResolver = locationadapter.NewFakeAddressResolver()
		coverageZoneResolver = locationadapter.NewFakeCoverageZoneResolver(persistence.CoverageZoneRepository)
	} else {
		addressResolver = locationadapter.NewGoogleAddressResolverFromEnv()
		coverageZoneResolver = locationadapter.NewGoogleCoverageZoneResolverFromEnv(persistence.CoverageZoneRepository)
	}

	storageComponents := storage.NewComponentsFromEnv()
	systemClock := clockadapter.NewSystemClock()

	// Realtime infrastructure
	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	ticketStore := realtime.NewPostgresTicketStore(database)

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
		mediaadapter.NewWebMAudioParser(),
		mediaadapter.NewMP4VideoParser(),
	)
	categoryService := category.NewService(persistence.CategoryRepository)
	coverageZoneService := coveragezone.NewService(persistence.CoverageZoneRepository)
	providerService := provider.NewService(
		persistence.UserRepository,
		persistence.CategoryRepository,
		fileService,
		persistence.WorkOrderRepository,
		persistence.CoverageZoneRepository,
		persistence.IdentityVerificationRepository,
	)
	consumerService := consumer.NewService(persistence.UserRepository, fileService, addressResolver, coverageZoneResolver)
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
	calendarConnectionService := calendarconnection.NewService(
		persistence.UserRepository,
		persistence.CalendarAuthorizationAttemptRepository,
		persistence.CalendarConnectionRepository,
		calendarOAuthConnector,
		calendarCredentialProtector,
		cryptography.NewSecureSecretGenerator(),
		systemClock,
	)
	paymentService := payment.NewService(
		persistence.PaymentIntentRepository,
		persistence.PaymentTransactionRepository,
		persistence.ServiceProposalRepository,
		persistence.WorkOrderRepository,
		persistence.UserRepository,
		persistence.PaymentAccountRepository,
		locking.NewPostgresAdvisoryLock(database),
		persistence.PaymentUnitOfWork,
		credentialProtector,
		paymentGateway,
		paymentGateway,
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
		persistence.WorkOrderUnitOfWork,
		systemClock,
	)
	calendarSyncService := workordercalendar.NewService(
		persistence.WorkOrderRepository,
		persistence.CalendarConnectionRepository,
		persistence.WorkOrderCalendarEventRepository,
		calendarEventPublisher,
		systemClock,
		notificator,
	)
	identityVerificationService := identityverification.NewService(
		persistence.UserRepository,
		persistence.IdentityVerificationRepository,
		persistence.IdentityVerificationUnitOfWork,
		identityVerifier,
		systemClock,
	)
	identityVerificationHandler := identity_verification_handler.NewIdentityVerificationHandler(identityVerificationService)
	if len(identityWebhooks) > 0 {
		identityVerificationHandler = identity_verification_handler.NewIdentityVerificationHandlerWithWebhook(
			identityVerificationService,
			identityWebhooks[0],
			systemClock,
		)
	}
	urgentWorkOrderScheduler := scheduler.NewScheduler(time.Hour, workOrderService)
	calendarSyncRunner := scheduler.NewCalendarSyncRunner(calendarSyncService)
	var calendarEventObserver CalendarEventObserver
	if observer, ok := calendarEventPublisher.(CalendarEventObserver); ok {
		calendarEventObserver = observer
	}
	_ = cancel // TODO: wire shutdown signal to cancel context

	return &Dependencies{
		Persistence:                 persistence,
		WorkOrderService:            workOrderService,
		UrgentWorkOrderScheduler:    urgentWorkOrderScheduler,
		CalendarSyncRunner:          calendarSyncRunner,
		CalendarEventObserver:       calendarEventObserver,
		CategoryHandler:             category_handler.NewCategoryHandler(categoryService),
		CalendarConnectionHandler:   calendar_connection_handler.NewCalendarConnectionHandler(calendarConnectionService, calendarHandlerConfig),
		CoverageZoneHandler:         coverage_zone_handler.NewCoverageZoneHandler(coverageZoneService),
		ConsumerHandler:             consumer_handler.NewConsumerHandler(consumerService),
		ProviderHandler:             provider_handler.NewProviderHandler(providerService),
		ConversationHandler:         conversation_handler.NewConversationHandler(conversationService),
		JobRequestHandler:           job_request_handler.NewJobRequestHandler(jobRequestService),
		IdentityVerificationHandler: identityVerificationHandler,
		PaymentAccountHandler:       payment_account_handler.NewPaymentAccountHandler(paymentAccountService, paymentAccountHandlerConfig),
		PaymentHandler:              payment_handler.NewPaymentHandler(paymentService, webhookVerifier),
		UserHandler:                 user_handler.NewUserHandlerWithIdentityVerification(userService, calendarConnectionService, identityVerificationService),
		FileHandler:                 file_handler.NewFileHandler(fileService),
		ServiceProposalHandler:      service_proposal_handler.NewServiceProposalHandler(servicePorposalService),
		WorkOrderHandler:            work_order_handler.NewWorkOrderHandler(workOrderService),
		TestHandler:                 test_handler.NewTestHandler(systemClock),
		Hub:                         hub,
		RealtimeHandler:             realtimeHandler,
		MessagePublisher:            messagePublisher,
		Clock:                       systemClock,
		ConsumerAddressResolver:     addressResolver,
		IdentityVerifier:            identityVerifier,
	}
}

func newTestIdentityVerificationWebhook() identity_verification_handler.IdentityVerificationWebhook {
	webhook, err := didit.NewWebhookAdapter("test-didit-webhook-secret")
	if err != nil {
		panic(fmt.Errorf("configuring test identity verification webhook: %w", err))
	}
	return webhook
}
