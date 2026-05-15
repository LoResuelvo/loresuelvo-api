package consumer

import "regexp"

type Service struct {
	consumerRepository Repository
}

func NewService(consumerRepository Repository) *Service {
	return &Service{
		consumerRepository: consumerRepository,
	}
}

func (cm *Service) RegisterConsumer(auth0ID string, email string, name string, surname string) error {
	if !cm.validateFormatEmail(email) {
		return ErrInvalidEmailFormat
	}
	consumer := NewConsumer(auth0ID, email, name, surname)
	return cm.consumerRepository.Save(consumer)
}

func (cm *Service) validateFormatEmail(email string) bool {
	regexEmail := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	matched, err := regexp.MatchString(regexEmail, email)
	return err == nil && matched
}
