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
	workOrderRepository := repositories.NewWorkOrderRepository(database, serviceProposalRepository)
	notificationRepository := repositories.NewNotificationRepository(database)
	authorizationAttemptRepository := repositories.NewAuthorizationAttemptRepository(database)
	paymentAccountRepository := repositories.NewPaymentAccountRepository(database, authorizationAttemptRepository)
	paymentIntentRepository := repositories.NewPaymentIntentRepository(database)

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
		AuthorizationAttemptRepository: authorizationAttemptRepository,
	}
}
