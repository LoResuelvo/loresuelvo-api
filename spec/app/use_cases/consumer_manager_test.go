package spec

import (
	"testing"

	usecases "github.com/LoResuelvo/loresuelvo-api/app/use_cases"
)

func TestRegisterConsumerWithValidData(t *testing.T) {
	consumerManager := usecases.NewConsumerManager(nil)

	err := consumerManager.RegisterConsumer(
		"ana@example.com",
		"Ana",
		"Perez",
		"Segura12345?",
	)

	if err != nil {
		t.Fatalf("se esperaba registrar el consumidor sin error, pero se obtuvo: %v", err)
	}
}
