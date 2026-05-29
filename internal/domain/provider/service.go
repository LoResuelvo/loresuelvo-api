package provider

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
)

type Service struct {
	providerRepository Repository
	categoryFinder     CategoryFinder
}

func NewService(repository Repository, categoryFinder CategoryFinder) *Service {
	return &Service{
		providerRepository: repository,
		categoryFinder:     categoryFinder,
	}
}

func (s *Service) RegisterProvider(authId, email, name, surname string, categoryID int) error {
	if s.providerRepository.FindByEmail(email) {
		return validator.ErrEmailAlreadyRegistered
	}

	category, err := s.validateCategory(categoryID)
	if err != nil {
		return err
	}

	provider, err := NewProvider(authId, email, name, surname, category)
	if err != nil {
		return err
	}

	return s.providerRepository.Save(*provider)
}

func (s *Service) FilterProvidersByCategoryID(categoryID int) ([]Provider, error) {
	existingCategory, err := s.validateCategory(categoryID)
	if err != nil {
		return nil, err
	}

	providers, err := s.providerRepository.FindByCategoryID(existingCategory.ID)
	if err != nil {
		return nil, err
	}

	for i := range providers {
		providers[i].Category = existingCategory
	}

	return providers, nil
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
