package steps_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"testing"

	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/cucumber/godog"
)

type testSuite struct {
	server             *httptest.Server
	database           *sql.DB
	consumerRepository *repositories.ConsumerRepository
	lastStatus         int
	lastBody           []byte
}

func (s *testSuite) registerAllSteps(sc *godog.ScenarioContext) {
	registerHelloWorldSteps(sc, s)
	registerConsumerAccountSteps(sc, s)
}

func newTestDb() *sql.DB {
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	if err != nil {
		panic(fmt.Errorf("Can not connect to test database: %w", err))
	}
	return database
}

func newTestSuite(tb testing.TB, database *sql.DB) *testSuite {
	consumerRepository := repositories.NewConsumerRepository(database)
	consumerService := consumer.NewService(consumerRepository)
	consumerHandler := handler.NewConsumerHandler(consumerService)

	router := httpadapter.NewRouter(consumerHandler)
	engine := router.SetUp()

	// httptest.Server wraps the engine — no port needed
	server := httptest.NewServer(engine)
	tb.Cleanup(func() {
		server.Close()
	})

	return &testSuite{
		server:             server,
		database:           database,
		consumerRepository: consumerRepository,
	}
}

func ScenarioInitializer(sc *godog.ScenarioContext, t *testing.T, database *sql.DB) {
	testSuite := newTestSuite(t, database)
	testSuite.registerAllSteps(sc)
	sc.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		if err := testSuite.consumerRepository.DeleteAll(); err != nil {
			return ctx, fmt.Errorf("no se pudo limpiar la base de datos de test: %w", err)
		}
		return ctx, nil
	})
}

// Godog entry point
func TestFeatures(t *testing.T) {
	database := newTestDb()
	t.Cleanup(func() { database.Close() })

	suite := godog.TestSuite{
		Name: "LoResuelvo Features",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			ScenarioInitializer(sc, t, database)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../"},
			Tags:     "~@wip",
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("godog tests failed")
	}
}
