package use_cases

import (
	"github.com/LoResuelvo/loresuelvo-api/model"
	repositories "github.com/LoResuelvo/loresuelvo-api/model/repositories_interfaces"
)

type ConsumerManager struct {
	consumerRepository repositories.ConsumerRepository
}

func NewConsumerManager(consumerRepository repositories.ConsumerRepository) *ConsumerManager {
	return &ConsumerManager{
		consumerRepository: consumerRepository,
	}
}

func (cm *ConsumerManager) RegisterConsumer(email string, name string, surname string, password string) error {
	consumer := model.NewConsumer(email, name, surname, password)
	return cm.consumerRepository.Save(consumer)
}
