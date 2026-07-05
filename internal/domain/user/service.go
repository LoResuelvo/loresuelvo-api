package user

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetCurrentUser(authID string) (User, error) {
	user, err := s.repository.FindByAuthID(authID)
	if err != nil {
		return nil, ErrNotFound
	}
	return user, nil
}
