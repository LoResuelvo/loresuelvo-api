package provider

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"

type Service struct {
	providerRepository Repository
}

func NewService(repository Repository) *Service {
	return &Service{providerRepository: repository}
}

func (s *Service) RegisterProvider(authId, email, name, surname string, coverageZones []string) error {
	if s.providerRepository.FindByEmail(email) {
		return validator.ErrEmailAlreadyRegistered
	}
	provider, err := NewProvider(authId, email, name, surname, coverageZones)
	if err != nil {
		return err
	}

	return s.providerRepository.Save(*provider)
}
