package consumer

import (
	"context"
	"errors"
	"fmt"

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

func (cm *Service) RegisterConsumer(ctx context.Context, auth0ID, email, name, surname, profilePhotoFileID string) (*Consumer, error) {
	if cm.userRepository.FindByEmail(email) {
		return nil, validator.ErrEmailAlreadyRegistered
	}

	var profilePhoto *filedomain.Image
	if profilePhotoFileID != "" {
		profilePhoto = &filedomain.Image{FileID: profilePhotoFileID}
	}
	consumer, err := NewConsumer(auth0ID, email, name, surname, profilePhoto)
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
		consumer.SetProfilePhotoURL(profilePhotoURL)
	}

	savedUser, err := cm.userRepository.Save(ctx, consumer)
	if err != nil {
		return nil, err
	}

	consumer.SetPersistenceID(savedUser.ID())
	return consumer, nil
}
