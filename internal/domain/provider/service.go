package provider

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"

type Service struct {
	providerRepository Repository
}

func NewService(repository Repository) *Service {
	return &Service{providerRepository: repository}
}

func (s *Service) RegisterProvider(authId, email, name, surname string, coverageZones []string) error {
	if !validator.ValidateEmail(email) {
		return validator.ErrInvalidEmailFormat
	}
	if s.providerRepository.FindByEmail(email) {
		return validator.ErrEmailAlreadyRegistered
	}
	provider := NewProvider(authId, email, name, surname, coverageZones)
	return s.providerRepository.Save(provider)
}
