package workorder

import (
	"context"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type Service struct {
	reader                 Reader
	userRepository         UserRepository
	fileURLResolver        FileURLResolver
	clock                  clock.Clock
	notificator            notification.Notificator
	notificationRepository NotificationRepository
}

func NewService(reader Reader, userRepository UserRepository, fileURLResolver FileURLResolver) *Service {
	return &Service{reader: reader, userRepository: userRepository, fileURLResolver: fileURLResolver}
}

func (s *Service) GetWorkOrders(ctx context.Context, auth0ID string) ([]readmodel.WorkOrderSummary, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, err
	}

	orders, err := s.reader.FindByUserID(ctx, foundUser.Base().ID, foundUser.Base().Role)
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
	actualTime := s.clock.Now()
	urgentWorkOrders, err := s.reader.FindWithLessScheduledTimeThan(ctx, actualTime)
	if err != nil {
		return err
	}

	for _, order := range urgentWorkOrders {
		consumerNotification, providerNotification := s.notificationForUsers(order)

		consumerNotification, err := s.notificationRepository.Save(ctx, consumerNotification)
		if err != nil {
			return err
		}

		providerNotification, err = s.notificationRepository.Save(ctx, providerNotification)
		if err != nil {
			return err
		}

		if err := s.notificator.Notify(ctx, consumerNotification); err != nil {
			return err
		}

		if err := s.notificator.Notify(ctx, providerNotification); err != nil {
			return err
		}
	}

	return nil
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
