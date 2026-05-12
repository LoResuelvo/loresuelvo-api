package steps

import (
	"testing"

	"github.com/cucumber/godog"
)

type testContext struct {
	responseBody string
	statusCode   int
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	tCtx := &testContext{}

	registrarPasosDeHello(ctx, tCtx)
	registrarPasosDeCuentaConsumidor(ctx, tCtx)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{".."},
			Tags:     "~@wip",
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("Fallo la ejecución de las pruebas de aceptación")
	}
}
