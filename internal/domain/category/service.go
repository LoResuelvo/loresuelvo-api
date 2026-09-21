package category

import "fmt"

type Service struct {
	categoryRepository Repository
}

func NewService(categoryRepository Repository) *Service {
	return &Service{categoryRepository: categoryRepository}
}

func (s *Service) CreateCategory(name string) (*Category, error) {
	category, err := New(name)
	if err != nil {
		return nil, err
	}

	savedCategory, err := s.categoryRepository.Save(*category)
	if err != nil {
		return nil, fmt.Errorf("saving category: %w", err)
	}

	return savedCategory, nil
}

func (s *Service) ListCategories() ([]Category, error) {
	categories, err := s.categoryRepository.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}

	return categories, nil
}
