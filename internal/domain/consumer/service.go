package consumer

type Service struct {
	consumerRepository Repository
}

func NewService(consumerRepository Repository) *Service {
	return &Service{
		consumerRepository: consumerRepository,
	}
}

func (cm *Service) RegisterConsumer(auth0ID string, email string, name string, surname string) error {
	consumer := NewConsumer(auth0ID, email, name, surname)
	return cm.consumerRepository.Save(consumer)
}
