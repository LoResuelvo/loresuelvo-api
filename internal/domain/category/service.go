package category

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

	if s.categoryRepository.FindByNormalizedName(category.NormalizedName) != nil {
		return nil, ErrAlreadyExists
	}

	return s.categoryRepository.Save(*category)
}

func (s *Service) ListCategories() ([]Category, error) {
	return s.categoryRepository.ListAll()
}
