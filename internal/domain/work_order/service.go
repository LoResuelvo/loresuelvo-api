package workorder

import (
	"context"
	"fmt"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type Service struct {
	reader          Reader
	userRepository  UserRepository
	fileURLResolver FileURLResolver
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
