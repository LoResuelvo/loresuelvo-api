package spec

import (
	"testing"

	usecases "github.com/LoResuelvo/loresuelvo-api/app/use_cases"
	"github.com/LoResuelvo/loresuelvo-api/model"
)

type consumerRepositoryMock struct {
	savedConsumer model.Consumer
}

func (repository *consumerRepositoryMock) Save(consumer model.Consumer) error {
	repository.savedConsumer = consumer
	return nil
}

func TestRegisterConsumerWithValidData(t *testing.T) {
	repository := &consumerRepositoryMock{}
	consumerManager := usecases.NewConsumerManager(repository)

	err := consumerManager.RegisterConsumer(
		"ana@example.com",
		"Ana",
		"Perez",
		"Segura12345?",
	)

	if err != nil {
		t.Fatalf("se esperaba registrar el consumidor sin error, pero se obtuvo: %v", err)
	}

	if repository.savedConsumer.Email != "ana@example.com" {
		t.Fatalf("se esperaba guardar el consumidor con email ana@example.com, pero se obtuvo %s", repository.savedConsumer.Email)
	}
}
