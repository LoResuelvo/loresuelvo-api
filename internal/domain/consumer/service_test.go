package consumer_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
)

type consumerRepositoryMock struct {
	savedConsumer consumer.Consumer
}

func (repository *consumerRepositoryMock) Save(consumer consumer.Consumer) error {
	repository.savedConsumer = consumer
	return nil
}

func TestRegisterConsumerWithValidData(t *testing.T) {
	repository := &consumerRepositoryMock{}
	consumerManager := consumer.NewService(repository)

	err := consumerManager.RegisterConsumer(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
	)

	if err != nil {
		t.Fatalf("se esperaba registrar el consumidor sin error, pero se obtuvo: %v", err)
	}

	if repository.savedConsumer.Auth0ID != "auth0|ana" {
		t.Fatalf("se esperaba guardar el consumidor con auth0_id auth0|ana, pero se obtuvo %s", repository.savedConsumer.Auth0ID)
	}

	if repository.savedConsumer.Email != "ana@example.com" {
		t.Fatalf("se esperaba guardar el consumidor con email ana@example.com, pero se obtuvo %s", repository.savedConsumer.Email)
	}
}
