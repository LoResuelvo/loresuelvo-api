package steps

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/db"
	"github.com/LoResuelvo/loresuelvo-api/persistence"
	"github.com/cucumber/godog"
)

var acceptanceTestContext *testContext

type testContext struct {
	responseBody       string
	statusCode         int
	database           *sql.DB
	consumerRepository *persistence.ConsumerRepository
}

func newTestContext() *testContext {
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	if err != nil {
		panic(fmt.Errorf("no se pudo conectar a la base de datos de test: %w", err))
	}

	return &testContext{
		database:           database,
		consumerRepository: persistence.NewConsumerRepository(database),
	}
}

func InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		acceptanceTestContext = newTestContext()
	})

	ctx.AfterSuite(func() {
		if acceptanceTestContext != nil && acceptanceTestContext.database != nil {
			_ = acceptanceTestContext.database.Close()
		}
	})
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	tCtx := acceptanceTestContext

	ctx.BeforeScenario(func(*godog.Scenario) {
		if err := queElSistemaEstaIniciado(tCtx); err != nil {
			panic(fmt.Errorf("no se pudo iniciar el sistema para los tests: %w", err))
		}

		if err := tCtx.consumerRepository.DeleteAll(); err != nil {
			panic(fmt.Errorf("no se pudo limpiar la base de datos de test: %w", err))
		}
	})

	registrarPasosDeHello(ctx, tCtx)
	registrarPasosDeCuentaConsumidor(ctx, tCtx)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer:  InitializeScenario,
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
