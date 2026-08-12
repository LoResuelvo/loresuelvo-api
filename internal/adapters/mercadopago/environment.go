package mercadopago

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const EnvironmentVariable = "MERCADO_PAGO_ENVIRONMENT"

var ErrInvalidEnvironment = errors.New("invalid Mercado Pago environment")

type Environment string

const (
	EnvironmentSandbox    Environment = "sandbox"
	EnvironmentProduction Environment = "production"
)

func EnvironmentFromEnv() (Environment, error) {
	return ParseEnvironment(os.Getenv(EnvironmentVariable))
}

func ParseEnvironment(value string) (Environment, error) {
	environment := Environment(strings.TrimSpace(value))
	if err := environment.Validate(); err != nil {
		return "", err
	}
	return environment, nil
}

func (environment Environment) Validate() error {
	switch environment {
	case EnvironmentSandbox, EnvironmentProduction:
		return nil
	default:
		return fmt.Errorf("%w: %s must be sandbox or production", ErrInvalidEnvironment, EnvironmentVariable)
	}
}

func (environment Environment) IsSandbox() bool {
	return environment == EnvironmentSandbox
}
