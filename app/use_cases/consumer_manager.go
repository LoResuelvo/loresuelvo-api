package use_cases

type ConsumerManager struct {
	consumerRepository ConsumerRepository
}

type ConsumerRepository interface{}

func NewConsumerManager(consumerRepository ConsumerRepository) *ConsumerManager {
	return &ConsumerManager{
		consumerRepository: consumerRepository,
	}
}

func (cm *ConsumerManager) RegisterConsumer(email string, name string, surname string, password string) error {
	return nil
}
