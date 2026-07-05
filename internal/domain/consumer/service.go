package consumer

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
)

type Service struct {
	userRepository UserRepository
}

func NewService(userRepository UserRepository) *Service {
	return &Service{
		userRepository: userRepository,
	}
}

func (cm *Service) RegisterConsumer(ctx context.Context, auth0ID string, email string, name string, surname string) error {
	if cm.userRepository.FindByEmail(email) {
		return validator.ErrEmailAlreadyRegistered
	}
	consumer, err := NewConsumer(auth0ID, email, name, surname)
	if err != nil {
		return err
	}
	_, err = cm.userRepository.Save(ctx, consumer)
	return err
}
