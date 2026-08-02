package workorder

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type Service struct {
	reader                    Reader
	userRepository            UserRepository
	fileURLResolver           FileURLResolver
	confirmationCodeDecryptor ConfirmationCodeDecryptor
	clock                     clock.Clock
	notificator               notification.Notificator
	notificationRepository    NotificationRepository
}

func NewService(
	reader Reader,
	userRepository UserRepository,
	fileURLResolver FileURLResolver,
	confirmationCodeDecryptor ConfirmationCodeDecryptor,
	notificationRepository NotificationRepository,
	notificator notification.Notificator,
	clock clock.Clock,
) *Service {
	return &Service{
		reader:                    reader,
		userRepository:            userRepository,
		fileURLResolver:           fileURLResolver,
		confirmationCodeDecryptor: confirmationCodeDecryptor,
		notificationRepository:    notificationRepository,
		notificator:               notificator,
		clock:                     clock,
	}
}

func (s *Service) GetConfirmationCode(
	ctx context.Context,
	auth0ID string,
	workOrderID int,
) (ConfirmationCode, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return ConfirmationCode{}, ErrOnlyConsumerCanViewConfirmationCode
	}
	order, err := s.reader.FindByID(ctx, workOrderID)
	if err != nil {
		return ConfirmationCode{}, err
	}
	authorization, err := order.ConfirmationAuthorizationFor(foundUser.ID())
	if err != nil {
		return ConfirmationCode{}, err
	}
	plaintext, err := s.confirmationCodeDecryptor.Decrypt(authorization.CodeCiphertext())
	if err != nil {
		return ConfirmationCode{}, fmt.Errorf("decrypting work order confirmation code: %w", err)
	}
	code, err := NewConfirmationCode(plaintext)
	if err != nil {
		return ConfirmationCode{}, fmt.Errorf("rehydrating work order confirmation code: %w", err)
	}
	return code, nil
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

	urlsByFileID, err := s.fileURLResolver.ResolvePublicURLs(ctx, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving work order counterpart profile photos: %w", err)
	}
	for index := range orders {
		orders[index].Counterpart.ProfilePhotoURL = urlsByFileID[orders[index].Counterpart.ProfilePhotoFileID]
	}

	return orders, nil
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
			savedNotification, saveErr := s.notificationRepository.Save(ctx, createdNotification)
			if saveErr != nil {
				notificationErrors = append(notificationErrors, saveErr)
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
		order.ID,
		s.clock,
	)

	providerNotification := notification.NewNotification(
		order.ProviderID(),
		notification.TypeWorkOrderCloseToScheduledTime,
		notification.ResourceWorkOrder,
		order.ID,
		s.clock,
	)

	return consumerNotification, providerNotification
}
