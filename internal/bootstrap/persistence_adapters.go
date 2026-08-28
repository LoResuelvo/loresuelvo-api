package bootstrap

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
)

type PersistenceAdapters struct {
	UserRepository                         *repositories.UserRepository
	CategoryRepository                     *repositories.CategoryRepository
	CoverageZoneRepository                 *repositories.CoverageZoneRepository
	ConversationRepository                 *repositories.ConversationRepository
	MessageRepository                      *repositories.MessageRepository
	MessageImageRepository                 *repositories.MessageImageRepository
	MessageAudioRepository                 *repositories.MessageAudioRepository
	MessageVideoRepository                 *repositories.MessageVideoRepository
	JobRequestRepository                   *repositories.JobRequestRepository
	ServiceProposalRepository              *repositories.ServiceProposalRepository
	ConversationReader                     *repositories.ConversationReader
	FileRepository                         *repositories.FileRepository
	NotificationRepository                 *repositories.NotificationRepository
	WorkOrderRepository                    *repositories.WorkOrderRepository
	PaymentAccountRepository               *repositories.PaymentAccountRepository
	PaymentIntentRepository                *repositories.PaymentIntentRepository
	PaymentTransactionRepository           *repositories.PaymentTransactionRepository
	PaymentUnitOfWork                      *repositories.PaymentUnitOfWork
	WorkOrderUnitOfWork                    *repositories.WorkOrderUnitOfWork
	AuthorizationAttemptRepository         *repositories.AuthorizationAttemptRepository
	CalendarAuthorizationAttemptRepository *repositories.GoogleCalendarAuthorizationAttemptRepository
	CalendarConnectionRepository           *repositories.GoogleCalendarConnectionRepository
	WorkOrderCalendarEventRepository       *repositories.WorkOrderCalendarEventRepository
	IdentityVerificationRepository         *repositories.IdentityVerificationRepository
}

func NewPersistenceAdapters(database *sql.DB) *PersistenceAdapters {
	userRepository := repositories.NewUserRepository(database)
	categoryRepository := repositories.NewCategoryRepository(database)
	coverageZoneRepository := repositories.NewCoverageZoneRepository(database)
	messageImageRepository := repositories.NewMessageImageRepository(database)
	messageAudioRepository := repositories.NewMessageAudioRepository(database)
	messageVideoRepository := repositories.NewMessageVideoRepository(database)
	messageRepository := repositories.NewMessageRepository(database, messageImageRepository, messageAudioRepository, messageVideoRepository)
	conversationRepository := repositories.NewConversationRepository(database, messageRepository)
	jobRequestRepository := repositories.NewJobRequestRepository(database)
	conversationReader := repositories.NewConversationReader(database, messageImageRepository, messageAudioRepository, messageVideoRepository)
	fileRepository := repositories.NewFileRepository(database)
	serviceProposalRepository := repositories.NewServiceProposalRepository(database)
	paymentIntentRepository := repositories.NewPaymentIntentRepository(database)
	paymentTransactionRepository := repositories.NewPaymentTransactionRepository(database)
	notificationRepository := repositories.NewNotificationRepository(database)
	workOrderRepository := repositories.NewWorkOrderRepository(
		database,
		serviceProposalRepository,
	)
	paymentUnitOfWork := repositories.NewPaymentUnitOfWork(
		database,
		paymentIntentRepository,
		paymentTransactionRepository,
		serviceProposalRepository,
		workOrderRepository,
		notificationRepository,
	)
	workOrderUnitOfWork := repositories.NewWorkOrderUnitOfWork(
		database,
		workOrderRepository,
		notificationRepository,
	)
	authorizationAttemptRepository := repositories.NewAuthorizationAttemptRepository(database)
	paymentAccountRepository := repositories.NewPaymentAccountRepository(database, authorizationAttemptRepository)
	calendarAuthorizationAttemptRepository := repositories.NewGoogleCalendarAuthorizationAttemptRepository(database)
	calendarConnectionRepository := repositories.NewGoogleCalendarConnectionRepository(database, calendarAuthorizationAttemptRepository)
	workOrderCalendarEventRepository := repositories.NewWorkOrderCalendarEventRepository(database)
	identityVerificationRepository := repositories.NewIdentityVerificationRepository(database)
	return &PersistenceAdapters{
		UserRepository:                         userRepository,
		CategoryRepository:                     categoryRepository,
		CoverageZoneRepository:                 coverageZoneRepository,
		ConversationRepository:                 conversationRepository,
		MessageRepository:                      messageRepository,
		MessageImageRepository:                 messageImageRepository,
		MessageAudioRepository:                 messageAudioRepository,
		MessageVideoRepository:                 messageVideoRepository,
		JobRequestRepository:                   jobRequestRepository,
		ServiceProposalRepository:              serviceProposalRepository,
		ConversationReader:                     conversationReader,
		FileRepository:                         fileRepository,
		NotificationRepository:                 notificationRepository,
		WorkOrderRepository:                    workOrderRepository,
		PaymentAccountRepository:               paymentAccountRepository,
		PaymentIntentRepository:                paymentIntentRepository,
		PaymentTransactionRepository:           paymentTransactionRepository,
		PaymentUnitOfWork:                      paymentUnitOfWork,
		WorkOrderUnitOfWork:                    workOrderUnitOfWork,
		AuthorizationAttemptRepository:         authorizationAttemptRepository,
		CalendarAuthorizationAttemptRepository: calendarAuthorizationAttemptRepository,
		CalendarConnectionRepository:           calendarConnectionRepository,
		WorkOrderCalendarEventRepository:       workOrderCalendarEventRepository,
		IdentityVerificationRepository:         identityVerificationRepository,
	}
}
