package bootstrap

import (
	"database/sql"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
)

type PersistenceAdapters struct {
	UserRepository                 *repositories.UserRepository
	CategoryRepository             *repositories.CategoryRepository
	ConversationRepository         *repositories.ConversationRepository
	MessageRepository              *repositories.MessageRepository
	MessageImageRepository         *repositories.MessageImageRepository
	JobRequestRepository           *repositories.JobRequestRepository
	ServiceProposalRepository      *repositories.ServiceProposalRepository
	ConversationReader             *repositories.ConversationReader
	FileRepository                 *repositories.FileRepository
	NotificationRepository         *repositories.NotificationRepository
	WorkOrderRepository            *repositories.WorkOrderRepository
	PaymentAccountRepository       *repositories.PaymentAccountRepository
	PaymentIntentRepository        *repositories.PaymentIntentRepository
	PaymentTransactionRepository   *repositories.PaymentTransactionRepository
	PaymentUnitOfWork              *repositories.PaymentUnitOfWork
	AuthorizationAttemptRepository *repositories.AuthorizationAttemptRepository
}

func NewPersistenceAdapters(database *sql.DB) *PersistenceAdapters {
	userRepository := repositories.NewUserRepository(database)
	categoryRepository := repositories.NewCategoryRepository(database)
	messageImageRepository := repositories.NewMessageImageRepository(database)
	messageRepository := repositories.NewMessageRepository(database, messageImageRepository)
	conversationRepository := repositories.NewConversationRepository(database, messageRepository)
	jobRequestRepository := repositories.NewJobRequestRepository(database)
	conversationReader := repositories.NewConversationReader(database, messageImageRepository)
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
	authorizationAttemptRepository := repositories.NewAuthorizationAttemptRepository(database)
	paymentAccountRepository := repositories.NewPaymentAccountRepository(database, authorizationAttemptRepository)
	return &PersistenceAdapters{
		UserRepository:                 userRepository,
		CategoryRepository:             categoryRepository,
		ConversationRepository:         conversationRepository,
		MessageRepository:              messageRepository,
		MessageImageRepository:         messageImageRepository,
		JobRequestRepository:           jobRequestRepository,
		ServiceProposalRepository:      serviceProposalRepository,
		ConversationReader:             conversationReader,
		FileRepository:                 fileRepository,
		NotificationRepository:         notificationRepository,
		WorkOrderRepository:            workOrderRepository,
		PaymentAccountRepository:       paymentAccountRepository,
		PaymentIntentRepository:        paymentIntentRepository,
		PaymentTransactionRepository:   paymentTransactionRepository,
		PaymentUnitOfWork:              paymentUnitOfWork,
		AuthorizationAttemptRepository: authorizationAttemptRepository,
	}
}
