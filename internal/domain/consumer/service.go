package consumer

import (
	"context"
	"errors"
	"fmt"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
)

type Service struct {
	userRepository       UserRepository
	fileService          FileService
	addressResolver      AddressResolver
	coverageZoneResolver CoverageZoneResolver
}

func NewService(
	userRepository UserRepository,
	fileService FileService,
	addressResolver AddressResolver,
	coverageZoneResolver CoverageZoneResolver,
) *Service {
	return &Service{
		userRepository:       userRepository,
		fileService:          fileService,
		addressResolver:      addressResolver,
		coverageZoneResolver: coverageZoneResolver,
	}
}

func (cm *Service) RegisterConsumer(
	ctx context.Context,
	auth0ID, email, name, surname, profilePhotoFileID string,
	address Address,
) (*Consumer, error) {
	if cm.userRepository.FindByEmail(email) {
		return nil, validator.ErrEmailAlreadyRegistered
	}
	normalizedAddress, err := NewAddress(address.Street, address.StreetNumber, address.Floor, address.Unit)
	if err != nil {
		return nil, err
	}

	location, coverageZone, err := cm.resolveRegistrationLocation(ctx, *normalizedAddress)
	if err != nil {
		return nil, err
	}

	var profilePhoto *filedomain.Image
	if profilePhotoFileID != "" {
		profilePhoto = &filedomain.Image{FileID: profilePhotoFileID}
	}
	consumer, err := NewConsumer(auth0ID, email, name, surname, profilePhoto, normalizedAddress, location, *coverageZone)
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

func (cm *Service) resolveRegistrationLocation(ctx context.Context, address Address) (GeoPoint, *coveragezone.CoverageZone, error) {
	location, err := cm.addressResolver.Resolve(ctx, address)
	if err != nil {
		return GeoPoint{}, nil, normalizeAddressResolutionError(err)
	}

	coverageZone, err := cm.coverageZoneResolver.Resolve(ctx, location)
	if err != nil {
		if errors.Is(err, coveragezone.ErrDoesNotExist) || errors.Is(err, coveragezone.ErrNotAvailable) {
			return GeoPoint{}, nil, ErrCoverageZoneNotAvailable
		}
		if errors.Is(err, ErrAddressServiceUnavailable) {
			return GeoPoint{}, nil, err
		}
		return GeoPoint{}, nil, ErrAddressServiceUnavailable
	}
	if coverageZone == nil {
		return GeoPoint{}, nil, ErrCoverageZoneNotAvailable
	}
	if err := coverageZone.ValidateSelection(); err != nil {
		return GeoPoint{}, nil, ErrCoverageZoneNotAvailable
	}

	return location, coverageZone, nil
}

func normalizeAddressResolutionError(err error) error {
	if errors.Is(err, ErrAddressNotValidated) || errors.Is(err, ErrAddressServiceUnavailable) {
		return err
	}

	return ErrAddressServiceUnavailable
}
