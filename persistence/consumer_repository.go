package persistence

import "github.com/LoResuelvo/loresuelvo-api/model"

type ConsumerRepository struct {
	consumers []model.Consumer
}

func NewConsumerRepository() *ConsumerRepository {
	return &ConsumerRepository{
		consumers: []model.Consumer{},
	}
}

func (repository *ConsumerRepository) Save(consumer model.Consumer) error {
	repository.consumers = append(repository.consumers, consumer)
	return nil
}
