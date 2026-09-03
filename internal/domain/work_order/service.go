package workorder

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type Service struct {
	reader                 Reader
	userRepository         UserRepository
	fileService            FileService
	clock                  clock.Clock
	notificator            notification.Notificator
	notificationRepository NotificationRepository
	unitOfWork             UnitOfWork
}

func NewService(
	reader Reader,
	userRepository UserRepository,
	fileService FileService,
	notificationRepository NotificationRepository,
	notificator notification.Notificator,
	unitOfWork UnitOfWork,
	clock clock.Clock,
) *Service {
	return &Service{
		reader:                 reader,
		userRepository:         userRepository,
		fileService:            fileService,
		notificationRepository: notificationRepository,
		notificator:            notificator,
		unitOfWork:             unitOfWork,
		clock:                  clock,
	}
}

func (s *Service) GetWorkOrders(ctx context.Context, auth0ID string) ([]readmodel.WorkOrderSummary, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, err
	}

	orders, err := s.reader.FindByUserID(ctx, foundUser.ID(), foundUser.Role())
	if err != nil {
		return nil, err
	}

	fileIDs := make([]string, 0, len(orders))
	for _, order := range orders {
		if order.Counterpart.ProfilePhotoFileID != "" {
			fileIDs = append(fileIDs, order.Counterpart.ProfilePhotoFileID)
		}
	}

	urlsByFileID, err := s.fileService.ResolvePublicURLs(ctx, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving work order counterpart profile photos: %w", err)
	}
	for index := range orders {
		orders[index].Counterpart.ProfilePhotoURL = urlsByFileID[orders[index].Counterpart.ProfilePhotoFileID]
	}

	return orders, nil
}

func (s *Service) GetWorkOrder(ctx context.Context, auth0ID string, workOrderID int) (*readmodel.WorkOrderDetail, error) {
	order, err := s.reader.FindByID(ctx, workOrderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrDoesNotExist
	}

	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil || !isWorkOrderParticipant(foundUser, order) {
		return nil, ErrOnlyWorkOrderParticipantCanView
	}

	detail := &readmodel.WorkOrderDetail{
		ID:                order.ID(),
		ServiceProposalID: order.ServiceProposalID(),
		ConsumerID:        order.ConsumerID(),
		ProviderID:        order.ProviderID(),
		Amount:            order.Amount(),
		ScheduledOn:       order.ScheduledOn(),
		Description:       order.Description(),
		Status:            string(order.Status()),
		AcceptedOn:        order.AcceptedOn(),
		PaidOn:            order.PaidOn(),
		Review:            reviewToReadModel(order.Review()),
	}

	report := order.CompletionReport()
	if report == nil {
		return detail, nil
	}
	if s.fileService == nil {
		return nil, fmt.Errorf("resolving work order completion images: file service is required")
	}

	images := make([]filedomain.Image, 0, len(report.ImageFileIDs()))
	for _, fileID := range report.ImageFileIDs() {
		images = append(images, filedomain.Image{FileID: fileID})
	}
	resolvedImages, err := s.fileService.ResolveWorkOrderCompletionImages(ctx, images)
	if err != nil {
		return nil, fmt.Errorf("resolving work order completion images: %w", err)
	}
	detail.CompletionReport = &readmodel.CompletionReport{
		ID:          report.ID(),
		Description: report.Description(),
		ReportedOn:  report.ReportedOn(),
		Images:      resolvedImages,
	}

	return detail, nil
}

func (s *Service) CreateReview(
	ctx context.Context,
	auth0ID string,
	workOrderID int,
	rating int,
	description string,
) (*readmodel.Review, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, err
	}

	order, err := s.reader.FindByID(ctx, workOrderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrDoesNotExist
	}
	reviewer, ok := foundUser.(*consumer.Consumer)
	if !ok || reviewer == nil || reviewer.Role() != consumer.Role {
		return nil, ErrOnlyWorkOrderConsumerCanReview
	}
	review, err := NewReview(rating, description)
	if err != nil {
		return nil, err
	}
	if err := order.AddReview(reviewer, review); err != nil {
		return nil, err
	}
	if s.unitOfWork == nil {
		return nil, ErrWorkOrderUnitOfWorkRequired
	}
	if err := s.unitOfWork.Execute(ctx, func(store TransactionalStore) error {
		return store.SaveWorkOrder(ctx, order)
	}); err != nil {
		return nil, err
	}

	return reviewToReadModel(review), nil
}

func reviewToReadModel(review *Review) *readmodel.Review {
	if review == nil {
		return nil
	}
	return &readmodel.Review{
		Rating:      review.Rating(),
		Description: review.Description(),
	}
}

func isWorkOrderParticipant(foundUser user.User, order *WorkOrder) bool {
	if foundUser == nil || order == nil {
		return false
	}
	if foundUser.Role() == consumer.Role {
		return foundUser.ID() == order.ConsumerID()
	}
	if foundUser.Role() == provider.Role {
		return foundUser.ID() == order.ProviderID()
	}
	return false
}

func (s *Service) ReportCompletion(
	ctx context.Context,
	auth0ID string,
	workOrderID int,
	description string,
	imageFileIDs []string,
) (*readmodel.CompletionReport, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, err
	}

	order, err := s.reader.FindByID(ctx, workOrderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrDoesNotExist
	}
	if foundUser.Role() != provider.Role || foundUser.ID() != order.ProviderID() {
		return nil, ErrOnlyAssignedProviderCanReport
	}

	preparedImages, err := s.fileService.PrepareWorkOrderCompletionImages(ctx, auth0ID, imageFileIDs)
	if errors.Is(err, filedomain.ErrWorkOrderCompletionImageNotAvailable) {
		return nil, ErrWorkOrderCompletionImageNotAvailable
	}
	if err != nil {
		return nil, fmt.Errorf("validating work order completion images: %w", err)
	}

	preparedImageFileIDs := make([]string, 0, len(preparedImages))
	for _, image := range preparedImages {
		preparedImageFileIDs = append(preparedImageFileIDs, image.FileID)
	}
	report, err := NewCompletionReport(description, preparedImageFileIDs, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := order.ReportCompletion(foundUser.ID(), report); err != nil {
		return nil, err
	}
	if s.unitOfWork == nil {
		return nil, ErrWorkOrderUnitOfWorkRequired
	}

	createdNotification := notification.NewNotification(
		order.ConsumerID(),
		notification.TypeWorkOrderCompletionReported,
		notification.ResourceWorkOrder,
		order.ID(),
		s.clock,
	)
	if err := s.unitOfWork.Execute(ctx, func(store TransactionalStore) error {
		if err := store.SaveWorkOrder(ctx, order); err != nil {
			return err
		}
		return store.SaveNotification(ctx, createdNotification)
	}); err != nil {
		return nil, err
	}

	if s.notificator != nil {
		_ = s.notificator.Notify(ctx, createdNotification)
	}

	return &readmodel.CompletionReport{
		ID:          report.ID(),
		Description: report.Description(),
		ReportedOn:  report.ReportedOn(),
		Images:      append([]filedomain.Image(nil), preparedImages...),
	}, nil
}

func (s *Service) UrgentNotification(ctx context.Context) error {
	now := s.clock.Now()
	urgentWorkOrders, err := s.reader.FindScheduledBetween(ctx, now, now.Add(24*time.Hour))
	if err != nil {
		return err
	}

	var notificationErrors []error
	for _, order := range urgentWorkOrders {
		consumerNotification, providerNotification := s.notificationForUsers(order)
		for _, createdNotification := range []*notification.Notification{consumerNotification, providerNotification} {
			savedNotification, created, saveErr := s.notificationRepository.SaveIfAbsent(ctx, createdNotification)
			if saveErr != nil {
				notificationErrors = append(notificationErrors, saveErr)
				continue
			}
			if !created {
				continue
			}

			if notifyErr := s.notificator.Notify(ctx, savedNotification); notifyErr != nil {
				notificationErrors = append(notificationErrors, notifyErr)
			}
		}
	}

	return errors.Join(notificationErrors...)
}

func (s *Service) notificationForUsers(order *WorkOrder) (*notification.Notification, *notification.Notification) {
	consumerNotification := notification.NewNotification(
		order.ConsumerID(),
		notification.TypeWorkOrderCloseToScheduledTime,
		notification.ResourceWorkOrder,
		order.ID(),
		s.clock,
	)

	providerNotification := notification.NewNotification(
		order.ProviderID(),
		notification.TypeWorkOrderCloseToScheduledTime,
		notification.ResourceWorkOrder,
		order.ID(),
		s.clock,
	)

	return consumerNotification, providerNotification
}
