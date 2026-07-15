package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
)

type Service struct {
	userRepository UserRepository
	categoryFinder CategoryFinder
	fileService    FileService
}

func NewService(repository UserRepository, categoryFinder CategoryFinder, fileService FileService) *Service {
	return &Service{
		userRepository: repository,
		categoryFinder: categoryFinder,
		fileService:    fileService,
	}
}

func (s *Service) RegisterProvider(ctx context.Context, authID, email, name, surname string, categoryID int, profilePhotoFileID string) (*readmodel.ProviderSummary, error) {
	if s.userRepository.FindByEmail(email) {
		return nil, validator.ErrEmailAlreadyRegistered
	}

	category, err := s.validateCategory(categoryID)
	if err != nil {
		return nil, err
	}

	provider, err := NewProvider(authID, email, name, surname, category, profilePhotoFileID)
	if err != nil {
		return nil, err
	}

	if err := s.validateProfilePhoto(ctx, authID, profilePhotoFileID); err != nil {
		return nil, err
	}

	profilePhotoURL, err := s.fileService.ResolvePublicURL(ctx, profilePhotoFileID)
	if err != nil {
		return nil, fmt.Errorf("resolving provider profile photo url: %w", err)
	}

	savedUser, err := s.userRepository.Save(ctx, provider)
	if err != nil {
		return nil, err
	}

	return &readmodel.ProviderSummary{
		ID:              savedUser.Base().ID,
		Name:            provider.Name(),
		Surname:         provider.Surname(),
		CategoryName:    category.Name,
		ProfilePhotoURL: profilePhotoURL,
	}, nil
}

func (s *Service) FilterProvidersByCategoryID(ctx context.Context, categoryID int) ([]readmodel.ProviderSummary, error) {
	if _, err := s.validateCategory(categoryID); err != nil {
		return nil, err
	}

	providers, err := s.userRepository.FindProvidersByCategoryID(categoryID)
	if err != nil {
		return nil, err
	}

	profilePhotoFileIDs := make([]string, 0, len(providers))
	for i := range providers {
		profilePhotoFileIDs = append(profilePhotoFileIDs, providers[i].ProfilePhotoFileID)
	}

	profilePhotoURLs, err := s.fileService.ResolvePublicURLs(ctx, profilePhotoFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving provider profile photo urls: %w", err)
	}

	return SummariesWithProfilePhotoURLs(providers, profilePhotoURLs), nil
}

func (s *Service) validateCategory(categoryID int) (*category.Category, error) {
	if categoryID <= 0 {
		return nil, category.ErrIDRequired
	}

	existingCategory := s.categoryFinder.FindByID(categoryID)
	if existingCategory == nil {
		return nil, category.ErrDoesNotExist
	}

	return existingCategory, nil
}

func (s *Service) validateProfilePhoto(ctx context.Context, authID, profilePhotoFileID string) error {
	if profilePhotoFileID == "" {
		return filedomain.ErrProfilePhotoRequired
	}
	err := s.fileService.ValidateProfilePhoto(ctx, authID, profilePhotoFileID)
	if errors.Is(err, filedomain.ErrProfilePhotoRequired) || errors.Is(err, filedomain.ErrProfilePhotoNotAvailable) {
		return err
	}
	if err != nil {
		return filedomain.ErrProfilePhotoNotAvailable
	}
	return nil
}
