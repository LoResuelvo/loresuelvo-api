package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
)

type Service struct {
	userRepository         UserRepository
	categoryFinder         CategoryFinder
	coverageZoneFinder     CoverageZoneFinder
	fileService            FileService
	profileReader          ProviderProfileReader
	identityApprovalReader IdentityApprovalReader
}

func NewService(
	repository UserRepository,
	categoryFinder CategoryFinder,
	fileService FileService,
	profileReader ProviderProfileReader,
	coverageZoneFinder CoverageZoneFinder,
	identityApprovalReaders ...IdentityApprovalReader,
) *Service {
	service := &Service{
		userRepository:     repository,
		categoryFinder:     categoryFinder,
		coverageZoneFinder: coverageZoneFinder,
		fileService:        fileService,
		profileReader:      profileReader,
	}
	if len(identityApprovalReaders) > 0 {
		service.identityApprovalReader = identityApprovalReaders[0]
	}
	return service
}

func (s *Service) RegisterProvider(ctx context.Context, authID, email, name, surname string, categoryID int, profilePhotoFileID string, coverageZoneIDs []int) (*Provider, error) {
	if s.userRepository.FindByEmail(email) {
		return nil, validator.ErrEmailAlreadyRegistered
	}

	category, err := s.validateCategory(categoryID)
	if err != nil {
		return nil, err
	}
	coverageZones, err := s.resolveCoverageZones(ctx, coverageZoneIDs)
	if err != nil {
		return nil, err
	}
	provider, err := NewProvider(
		authID,
		email,
		name,
		surname,
		category,
		&filedomain.Image{FileID: profilePhotoFileID},
		coverageZones,
	)
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
	provider.SetProfilePhotoURL(profilePhotoURL)

	savedUser, err := s.userRepository.Save(ctx, provider)
	if err != nil {
		return nil, err
	}

	provider.SetPersistenceID(savedUser.ID())
	return provider, nil
}

func (s *Service) resolveCoverageZones(ctx context.Context, ids []int) ([]coveragezone.CoverageZone, error) {
	zones := make([]coveragezone.CoverageZone, 0, len(ids))
	if err := coveragezone.ValidateUniqueIDs(ids); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return zones, nil
	}
	if s.coverageZoneFinder == nil {
		return nil, ErrCoverageZoneFinderNotConfigured
	}

	for _, id := range ids {
		zone, err := s.coverageZoneFinder.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, coveragezone.ErrDoesNotExist) {
				return nil, coveragezone.ErrDoesNotExist
			}
			return nil, fmt.Errorf("finding coverage zone %d: %w", id, err)
		}
		if zone == nil {
			return nil, coveragezone.ErrDoesNotExist
		}
		if err := zone.ValidateSelection(); err != nil {
			return nil, err
		}
		zones = append(zones, *zone)
	}

	return zones, nil
}

func (s *Service) FilterProvidersByCategoryID(ctx context.Context, categoryID int) ([]Provider, error) {
	if _, err := s.validateCategory(categoryID); err != nil {
		return nil, err
	}

	providers, err := s.userRepository.FindProvidersByCategoryID(categoryID)
	if err != nil {
		return nil, err
	}

	profilePhotoFileIDs := make([]string, 0, len(providers))
	for i := range providers {
		profilePhotoFileIDs = append(profilePhotoFileIDs, providers[i].ProfilePhoto().FileID)
	}

	profilePhotoURLs, err := s.fileService.ResolvePublicURLs(ctx, profilePhotoFileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving provider profile photo urls: %w", err)
	}

	return WithProfilePhotoURLs(providers, profilePhotoURLs), nil
}

func (s *Service) SearchProvidersByCategoryID(ctx context.Context, categoryID int) ([]readmodel.ProviderSearchResult, error) {
	providers, err := s.FilterProvidersByCategoryID(ctx, categoryID)
	if err != nil {
		return nil, err
	}

	results := make([]readmodel.ProviderSearchResult, 0, len(providers))
	if len(providers) == 0 {
		return results, nil
	}
	if s.profileReader == nil {
		return nil, ErrProfileReaderNotConfigured
	}

	providerIDs := make([]int, 0, len(providers))
	for index := range providers {
		providerIDs = append(providerIDs, providers[index].ID())
	}

	ratingStatsByProviderID, err := s.profileReader.FindRatingStatsByProviderIDs(ctx, providerIDs)
	if err != nil {
		return nil, fmt.Errorf("finding provider rating stats for search: %w", err)
	}
	if s.identityApprovalReader == nil {
		return nil, ErrIdentityApprovalReaderNotConfigured
	}
	identityApprovedByProviderID, err := s.identityApprovalReader.FindApprovedByProviderIDs(ctx, providerIDs)
	if err != nil {
		return nil, fmt.Errorf("finding approved provider identities for search: %w", err)
	}

	for index := range providers {
		foundProvider := providers[index]
		ratingSummary := ratingStatsByProviderID[foundProvider.ID()].Summary()
		results = append(results, readmodel.ProviderSearchResult{
			ID:               foundProvider.ID(),
			Name:             foundProvider.Name(),
			Surname:          foundProvider.Surname(),
			CategoryName:     categoryName(&foundProvider),
			ProfilePhoto:     foundProvider.ProfilePhoto(),
			RatingAverage:    ratingSummary.Average,
			RatingCount:      ratingSummary.Count,
			IdentityVerified: identityApprovedByProviderID[foundProvider.ID()],
		})
	}

	return results, nil
}

func (s *Service) GetProviderProfile(ctx context.Context, providerID int) (*Provider, error) {
	foundProvider, err := s.userRepository.FindProviderByID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	profilePhotoURL, err := s.fileService.ResolvePublicURL(ctx, foundProvider.ProfilePhoto().FileID)
	if err != nil {
		return nil, fmt.Errorf("resolving provider profile photo url: %w", err)
	}
	foundProvider.SetProfilePhotoURL(profilePhotoURL)

	return foundProvider, nil
}

func (s *Service) GetProviderProfileDetail(ctx context.Context, providerID int) (*readmodel.Profile, error) {
	if s.profileReader == nil {
		return nil, ErrProfileReaderNotConfigured
	}

	foundProvider, err := s.GetProviderProfile(ctx, providerID)
	if err != nil {
		return nil, err
	}

	ratingStats, err := s.profileReader.FindRatingStatsByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("finding provider rating stats: %w", err)
	}

	workOrders, err := s.profileReader.FindPaidWorkHistoryByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("finding provider paid work history: %w", err)
	}
	if workOrders == nil {
		workOrders = []readmodel.WorkOrder{}
	}
	if s.identityApprovalReader == nil {
		return nil, ErrIdentityApprovalReaderNotConfigured
	}
	identityApprovedByProviderID, err := s.identityApprovalReader.FindApprovedByProviderIDs(ctx, []int{foundProvider.ID()})
	if err != nil {
		return nil, fmt.Errorf("finding approved provider identity for profile: %w", err)
	}

	ratingSummary := ratingStats.Summary()
	return &readmodel.Profile{
		ID:               foundProvider.ID(),
		Name:             foundProvider.Name(),
		Surname:          foundProvider.Surname(),
		ProfilePhoto:     foundProvider.ProfilePhoto(),
		CategoryID:       categoryID(foundProvider),
		CategoryName:     categoryName(foundProvider),
		RatingAverage:    ratingSummary.Average,
		RatingCount:      ratingSummary.Count,
		IdentityVerified: identityApprovedByProviderID[foundProvider.ID()],
		WorkOrders:       workOrders,
	}, nil
}

func categoryID(foundProvider *Provider) int {
	if foundProvider.Category == nil {
		return 0
	}
	return foundProvider.Category.ID
}

func categoryName(foundProvider *Provider) string {
	if foundProvider.Category == nil {
		return ""
	}
	return foundProvider.Category.Name
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
