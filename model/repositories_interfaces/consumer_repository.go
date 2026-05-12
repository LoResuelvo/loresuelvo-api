package repositories_interfaces

import "github.com/LoResuelvo/loresuelvo-api/model"

type ConsumerRepository interface {
	Save(consumer model.Consumer) error
}
