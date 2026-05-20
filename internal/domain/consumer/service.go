package consumer

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"

type Service struct {
	consumerRepository Repository
}

func NewService(consumerRepository Repository) *Service {
	return &Service{
		consumerRepository: consumerRepository,
	}
}

func (cm *Service) RegisterConsumer(auth0ID string, email string, name string, surname string) error {
	if cm.consumerRepository.FindByEmail(email) {
		return validator.ErrEmailAlreadyRegistered
	}
	consumer, err := NewConsumer(auth0ID, email, name, surname)
	if err != nil {
		return err
	}
	return cm.consumerRepository.Save(*consumer)
}
