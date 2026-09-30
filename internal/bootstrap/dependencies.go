package bootstrap

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	chatbotadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/chatbot"
	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/cryptography"
	googlecalendar "github.com/LoResuelvo/loresuelvo-api/internal/adapters/google_calendar"
	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/admin_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/audit_log_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/calendar_connection_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/category_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/consumer_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/conversation_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/coverage_zone_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/file_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/health_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/identity_verification_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/job_request_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/operation_chat_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/operation_detail_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/operation_inbox_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/operation_media_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_account_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/service_proposal_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/test_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/user_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/work_order_handler"
	didit "github.com/LoResuelvo/loresuelvo-api/internal/adapters/identityverification/didit"
	locationadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/location"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/locking"
	mediaadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/media"
	notificationadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/notification"
	mercadopagopayment "github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment/mercadopago"
	paymentaccountadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/realtime"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/scheduler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/health"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/google/uuid"
)

type Dependencies struct {
	Persistence *PersistenceAdapters
	Runtime     RuntimeDependencies
	Clock       *clockadapter.SystemClock

	routerConfig httpadapter.RouterConfig
}

type RuntimeDependencies struct {
	UrgentWorkOrderScheduler *scheduler.Scheduler
	CalendarSyncRunner       *scheduler.CalendarSyncRunner
	Hub                      *realtime.Hub
	RealtimeDispatcher       *realtime.Dispatcher
	Readiness                *health.Readiness
}

type dependencyAdapters struct {
	chatbot                             conversation.Chatbot
	paymentAccountOAuthConnector        paymentaccount.OAuthConnector
	paymentGateway                      payment.Gateway
	webhookVerifier                     payment_handler.WebhookVerifier
	credentialProtector                 paymentaccount.CredentialProtector
	secretGenerator                     paymentaccount.SecretGenerator
	paymentAccountHandlerConfig         payment_account_handler.Config
	calendarOAuthConnector              calendarconnection.OAuthConnector
	calendarCredentialProtector         calendarconnection.CredentialProtector
	calendarEventPublisher              workordercalendar.EventPublisher
	calendarHandlerConfig               calendar_connection_handler.Config
	identityVerifier                    identityverification.IdentityVerifier
	addressResolverOverride             consumer.AddressResolver
	recommendationConfig                conversation.ProviderRecommendationConfig
	identityWebhook                     identity_verification_handler.IdentityVerificationWebhook
	categoryUnitOfWorkDecorator         func(category.UnitOfWork) category.UnitOfWork
	operationDetailAuditWriterDecorator func(audit.Writer) audit.Writer
	operationChatMediaResolverDecorator func(operation.ChatMediaResolver) operation.ChatMediaResolver
	providerDiagnosticTestOptions       ProviderDiagnosticTestOptions
	auditCursorSigningKey               []byte
}

func (dependencies *Dependencies) RouterConfig(
	auth0Validator *validator.Validator,
	logger *slog.Logger,
	environment httpadapter.Environment,
) httpadapter.RouterConfig {
	config := dependencies.routerConfig
	config.Environment = environment
	config.Auth0Validator = auth0Validator
	config.Logger = logger
	return config
}

func NewDependencies(database *sql.DB) (*Dependencies, error) {
	chatbot := chatbotadapter.NewChatbotFromEnv()
	recommendationConfig, err := chatbotadapter.ProviderRecommendationConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring chatbot provider recommendation: %w", err)
	}
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
	return newDependencies(database, dependencyAdapters{
		chatbot:                      chatbot,
		paymentAccountOAuthConnector: paymentAccountOAuthConnector,
		paymentGateway:               paymentGateway,
		webhookVerifier:              webhookVerifier,
		credentialProtector:          credentialCipher,
		secretGenerator:              cryptography.NewSecureSecretGenerator(),
		paymentAccountHandlerConfig:  paymentAccountHandlerConfig,
		calendarOAuthConnector:       calendarOAuthConnector,
		calendarCredentialProtector:  calendarCredentialCipher,
		calendarEventPublisher:       calendarEventPublisher,
		calendarHandlerConfig:        calendarHandlerConfig,
		identityVerifier:             identityVerifier,
		recommendationConfig:         recommendationConfig,
		identityWebhook:              identityWebhook,
		auditCursorSigningKey:        []byte(os.Getenv("AUDIT_CURSOR_HMAC_KEY")),
	})
}

func newDependencies(database *sql.DB, adapters dependencyAdapters) (*Dependencies, error) {
	persistence := NewPersistenceAdapters(database)
	readiness := health.NewReadiness(database)

	var addressResolver consumer.AddressResolver
	var coverageZoneResolver consumer.CoverageZoneResolver
	if adapters.addressResolverOverride != nil {
		addressResolver = adapters.addressResolverOverride
		coverageZoneResolver = locationadapter.NewFakeCoverageZoneResolver(persistence.CoverageZoneRepository)
	} else {
		addressResolver = locationadapter.NewGoogleAddressResolverFromEnv()
		coverageZoneResolver = locationadapter.NewGoogleCoverageZoneResolverFromEnv(persistence.CoverageZoneRepository)
	}

	storageComponents, err := storage.NewComponentsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring storage: %w", err)
	}
	systemClock := clockadapter.NewSystemClock()

	// Realtime infrastructure
	realtimeEventBus := realtime.NewPostgresEventBus(database)
	hub := realtime.NewHub()
	dispatcher := realtime.NewDispatcher(hub, realtimeEventBus)

	ticketStore := realtime.NewPostgresTicketStore(database)

	messagePublisher := realtime.NewPublisher(dispatcher, persistence.UserRepository)
	realtimeNotificationNotificator := realtime.NewNotificationNotificator(dispatcher, persistence.UserRepository)
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
	categoryUnitOfWork := category.UnitOfWork(persistence.CategoryUnitOfWork)
	if adapters.categoryUnitOfWorkDecorator != nil {
		categoryUnitOfWork = adapters.categoryUnitOfWorkDecorator(categoryUnitOfWork)
	}
	categoryService := category.NewService(persistence.CategoryRepository, categoryUnitOfWork, persistence.UserRepository, systemClock)
	auditLogQueryService := audit.NewLogQueryService(
		persistence.AuditEventRepository,
		persistence.AuditEventRepository,
		persistence.UserRepository,
		systemClock,
	)
	auditLogHandler, err := audit_log_handler.NewHandler(auditLogQueryService, adapters.auditCursorSigningKey)
	if err != nil {
		return nil, fmt.Errorf("configuring audit log cursor: %w", err)
	}
	coverageZoneService := coveragezone.NewService(persistence.CoverageZoneRepository)
	providerService := provider.NewService(
		persistence.ProviderSearchReader,
		persistence.UserRepository,
		persistence.CategoryRepository,
		fileService,
		persistence.WorkOrderRepository,
		persistence.CoverageZoneRepository,
		persistence.IdentityVerificationRepository,
	)
	activityService := provider.NewActivityService(persistence.ProviderActivityReader, persistence.ProviderActivityActorFinder, systemClock)
	collectionService := provider.NewCollectionService(persistence.ProviderCollectionReader, persistence.ProviderActivityActorFinder, systemClock)
	collectionHandler, err := provider_handler.NewCollectionHandler(collectionService, adapters.auditCursorSigningKey)
	if err != nil {
		return nil, fmt.Errorf("configuring collection cursor: %w", err)
	}
	consumerService := consumer.NewService(persistence.UserRepository, fileService, addressResolver, coverageZoneResolver)
	conversationService := conversation.NewService(
		persistence.ConversationRepository,
		persistence.UserRepository,
		persistence.ConversationReader,
		messagePublisher,
		adapters.chatbot,
		persistence.CategoryRepository,
		fileService,
		systemClock,
		adapters.recommendationConfig,
		persistence.WorkOrderRepository,
	)
	jobRequestService := jobrequest.NewService(
		persistence.JobRequestRepository,
		persistence.UserRepository,
		persistence.ConversationRepository,
		fileService,
		systemClock,
	)
	userService := user.NewService(persistence.UserRepository, fileService)
	adminService := admin.NewService(
		persistence.AdminDirectoryReader,
		persistence.AdminDirectoryReader,
		fileService,
	)
	paymentAccountService := paymentaccount.NewService(
		persistence.UserRepository,
		persistence.AuthorizationAttemptRepository,
		persistence.PaymentAccountRepository,
		adapters.paymentAccountOAuthConnector,
		adapters.credentialProtector,
		adapters.secretGenerator,
		systemClock,
	)
	calendarConnectionService := calendarconnection.NewService(
		persistence.UserRepository,
		persistence.CalendarAuthorizationAttemptRepository,
		persistence.CalendarConnectionRepository,
		adapters.calendarOAuthConnector,
		adapters.calendarCredentialProtector,
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
		adapters.credentialProtector,
		adapters.paymentGateway,
		adapters.paymentGateway,
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
		adapters.paymentAccountOAuthConnector.Provider(),
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
		adapters.calendarEventPublisher,
		systemClock,
		notificator,
	)
	identityVerificationService := identityverification.NewService(
		persistence.UserRepository,
		persistence.IdentityVerificationRepository,
		persistence.IdentityVerificationUnitOfWork,
		adapters.identityVerifier,
		systemClock,
	)
	identityVerificationHandler := identity_verification_handler.NewIdentityVerificationHandlerWithWebhook(
		identityVerificationService,
		adapters.identityWebhook,
		systemClock,
	)
	schedulerLock := locking.NewPostgresAdvisoryLock(database)
	urgentWorkOrderScheduler := scheduler.NewScheduler(
		time.Hour,
		workOrderService,
		scheduler.WithLock(schedulerLock, scheduler.UrgentNotificationLockKey),
	)
	calendarSyncRunner := scheduler.NewCalendarSyncRunner(
		calendarSyncService,
		scheduler.WithLock(schedulerLock, scheduler.CalendarSyncLockKey),
	)
	operationDetailAuditWriter := audit.Writer(persistence.AuditEventRepository)
	if adapters.operationDetailAuditWriterDecorator != nil {
		operationDetailAuditWriter = adapters.operationDetailAuditWriterDecorator(operationDetailAuditWriter)
	}
	operationChatMediaResolver := operation.ChatMediaResolver(fileService)
	if adapters.operationChatMediaResolverDecorator != nil {
		operationChatMediaResolver = adapters.operationChatMediaResolverDecorator(operationChatMediaResolver)
	}
	operationChatAttachments := repositories.NewConversationMessageAttachmentReader(repositories.NewMessageImageRepository(database), repositories.NewMessageAudioRepository(database), repositories.NewMessageVideoRepository(database))
	operationChatHandler, err := operation_chat_handler.NewHandler(operation.NewChatService(repositories.NewOperationConversationReader(database), repositories.NewConversationMessagePageReader(database), persistence.UserRepository, operationDetailAuditWriter, systemClock, operationChatAttachments, operationChatMediaResolver), adapters.auditCursorSigningKey)
	if err != nil {
		return nil, fmt.Errorf("configuring operation chat cursor: %w", err)
	}
	diagnosticReader := admin.ProviderDiagnosticReader(persistence.ProviderDiagnosticReader)
	diagnosticWriter := audit.Writer(persistence.AuditEventRepository)
	if adapters.providerDiagnosticTestOptions.ReaderDecorator != nil {
		diagnosticReader = adapters.providerDiagnosticTestOptions.ReaderDecorator(diagnosticReader)
	}
	if adapters.providerDiagnosticTestOptions.AuditWriterDecorator != nil {
		diagnosticWriter = adapters.providerDiagnosticTestOptions.AuditWriterDecorator(diagnosticWriter)
	}
	diagnosticHandler := admin_handler.NewProviderDiagnosticHandler(admin.NewDiagnosticService(diagnosticReader, fileService, persistence.UserRepository, diagnosticWriter, systemClock))
	historyReader := admin.ConsumerHistoryReader(persistence.ConsumerHistoryReader)
	historyWriter := audit.Writer(persistence.AuditEventRepository)
	if adapters.providerDiagnosticTestOptions.ConsumerHistory.ReaderDecorator != nil {
		historyReader = adapters.providerDiagnosticTestOptions.ConsumerHistory.ReaderDecorator(historyReader)
	}
	if adapters.providerDiagnosticTestOptions.ConsumerHistory.AuditWriterDecorator != nil {
		historyWriter = adapters.providerDiagnosticTestOptions.ConsumerHistory.AuditWriterDecorator(historyWriter)
	}
	historyHandler, err := admin_handler.NewConsumerHistoryHandler(admin.NewConsumerHistoryService(historyReader, fileService, persistence.UserRepository, historyWriter, systemClock), adapters.auditCursorSigningKey)
	if err != nil {
		return nil, fmt.Errorf("configuring consumer history cursor: %w", err)
	}

	return &Dependencies{
		Persistence: persistence,
		Runtime: RuntimeDependencies{
			UrgentWorkOrderScheduler: urgentWorkOrderScheduler,
			CalendarSyncRunner:       calendarSyncRunner,
			Hub:                      hub,
			RealtimeDispatcher:       dispatcher,
			Readiness:                readiness,
		},
		Clock: systemClock,
		routerConfig: httpadapter.RouterConfig{
			AdminHandler:                admin_handler.NewAdminHandler(adminService),
			ProviderDiagnosticHandler:   diagnosticHandler,
			ConsumerHistoryHandler:      historyHandler,
			AuditLogHandler:             auditLogHandler,
			OperationInboxHandler:       operation_inbox_handler.NewHandler(operation.NewInboxService(persistence.OperationInboxReader, systemClock)),
			OperationDetailHandler:      operation_detail_handler.NewHandler(operation.NewDetailService(persistence.OperationDetailReader, persistence.UserRepository, operationDetailAuditWriter, systemClock)),
			OperationChatHandler:        operationChatHandler,
			OperationMediaHandler:       operation_media_handler.NewHandler(operation.NewMediaService(persistence.OperationMediaReader, repositories.NewOperationMediaFileFinder(persistence.FileRepository), storageComponents.Storage)),
			CategoryHandler:             category_handler.NewCategoryHandler(categoryService),
			CalendarConnectionHandler:   calendar_connection_handler.NewCalendarConnectionHandler(calendarConnectionService, adapters.calendarHandlerConfig),
			CoverageZoneHandler:         coverage_zone_handler.NewCoverageZoneHandler(coverageZoneService),
			ConsumerHandler:             consumer_handler.NewConsumerHandler(consumerService),
			ProviderHandler:             provider_handler.NewProviderHandler(providerService),
			ActivityHandler:             provider_handler.NewActivityHandler(activityService),
			CollectionHandler:           collectionHandler,
			ConversationHandler:         conversation_handler.NewConversationHandler(conversationService),
			JobRequestHandler:           job_request_handler.NewJobRequestHandler(jobRequestService),
			IdentityVerificationHandler: identityVerificationHandler,
			PaymentAccountHandler:       payment_account_handler.NewPaymentAccountHandler(paymentAccountService, adapters.paymentAccountHandlerConfig),
			PaymentHandler:              payment_handler.NewPaymentHandler(paymentService, adapters.webhookVerifier),
			UserHandler:                 user_handler.NewUserHandlerWithIdentityVerification(userService, calendarConnectionService, identityVerificationService),
			FileHandler:                 file_handler.NewFileHandler(fileService),
			HealthHandler:               health_handler.NewHealthHandler(readiness),
			ServiceProposalHandler:      service_proposal_handler.NewServiceProposalHandler(servicePorposalService),
			WorkOrderHandler:            work_order_handler.NewWorkOrderHandler(workOrderService),
			TestHandler:                 test_handler.NewTestHandler(systemClock),
			RealtimeHandler:             realtimeHandler,
		},
	}, nil
}
