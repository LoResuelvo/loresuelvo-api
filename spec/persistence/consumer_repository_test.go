package persistence_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/model"
	"github.com/LoResuelvo/loresuelvo-api/persistence"
	"github.com/stretchr/testify/assert"
)

func TestConsumerRepositoryCanSaveAConsumer(t *testing.T) {
	repo := persistence.NewConsumerRepository()
	consumer := model.NewConsumer("josugod@gmail.com", "Josue", "el pro", "SoyUnCrack123")
	err := repo.Save(consumer)
	assert.Nil(t, err)
}
