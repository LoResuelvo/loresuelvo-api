package category

type Service struct {
	categoryRepository Repository
}

func NewService(categoryRepository Repository) *Service {
	return &Service{categoryRepository: categoryRepository}
}

func (s *Service) CreateCategory(name string) error {
	category, err := New(name)
	if err != nil {
		return err
	}

	if s.categoryRepository.FindByNormalizedName(category.NormalizedName) {
		return ErrAlreadyExists
	}

	return s.categoryRepository.Save(*category)
}
