package consumer

type Service struct {
	consumerRepository Repository
}

func NewService(consumerRepository Repository) *Service {
	return &Service{
		consumerRepository: consumerRepository,
	}
}

func (cm *Service) RegisterConsumer(email string, name string, surname string, password string) error {
	consumer := NewConsumer(email, name, surname, password)
	return cm.consumerRepository.Save(consumer)
}
