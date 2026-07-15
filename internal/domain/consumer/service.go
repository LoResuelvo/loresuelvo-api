package consumer

import (
	"context"
	"errors"
	"fmt"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
)

type Service struct {
	userRepository UserRepository
	fileService    FileService
}

func NewService(userRepository UserRepository, fileService FileService) *Service {
	return &Service{
		userRepository: userRepository,
		fileService:    fileService,
	}
}

func (cm *Service) RegisterConsumer(ctx context.Context, auth0ID, email, name, surname, profilePhotoFileID string) (*readmodel.ConsumerSummary, error) {
	if cm.userRepository.FindByEmail(email) {
		return nil, validator.ErrEmailAlreadyRegistered
	}

	consumer, err := NewConsumer(auth0ID, email, name, surname, profilePhotoFileID)
	if err != nil {
		return nil, err
	}

	if profilePhotoFileID != "" {
		if err := cm.fileService.ValidateProfilePhoto(ctx, auth0ID, profilePhotoFileID); err != nil {
			if errors.Is(err, filedomain.ErrProfilePhotoNotAvailable) {
				return nil, err
			}
			return nil, filedomain.ErrProfilePhotoNotAvailable
		}
	}

	profilePhotoURL := ""
	if profilePhotoFileID != "" {
		profilePhotoURL, err = cm.fileService.ResolvePublicURL(ctx, profilePhotoFileID)
		if err != nil {
			return nil, fmt.Errorf("resolving consumer profile photo url: %w", err)
		}
	}

	savedUser, err := cm.userRepository.Save(ctx, consumer)
	if err != nil {
		return nil, err
	}

	return &readmodel.ConsumerSummary{
		ID:              savedUser.Base().ID,
		Name:            consumer.Name,
		Surname:         consumer.Surname,
		ProfilePhotoURL: profilePhotoURL,
	}, nil
}
