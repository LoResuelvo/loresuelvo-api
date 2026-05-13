package steps_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/cucumber/godog"

	httpAdapter "myapp/internal/adapters/http"
	"myapp/internal/adapters/http/handler"
	postgresAdapter "myapp/internal/adapters/postgres"
	"myapp/internal/domain/user"
	"myapp/internal/infrastructure/db"
)

type testSuite struct {
	server     *httptest.Server
	database   *sql.DB
	lastStatus int
	lastBody   []byte
}

func (s *testSuite) registerAllSteps(sc *godog.ScenarioContext) {
	registerHelloWorldSteps(sc, s)
	registerUserSteps(sc, s)
}
func newTestDb() *sql.DB {
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	if err != nil {
		panic(fmt.Errorf("Can not connect to test database: %w", err))
	}
	return database
}
func newTestSuite(tb testing.TB, database *sql.DB) *testSuite {
	userRepo := postgresAdapter.NewUserRepository(database)

	userService := user.NewService(userRepo)

	userHandler := handler.NewUserHandler(userService)

	router := httpAdapter.NewRouter(userHandler)
	engine := router.Setup()

	// httptest.Server wraps the engine — no port needed
	server := httptest.NewServer(engine)
	tb.Cleanup(func() {
		server.Close()
		database.Close()
	})
	return &testSuite{server: server, database: database}
}

func ScenarioInitializer(sc *godog.ScenarioContext, t *testing.T, database *sql.DB) {
	testSuit := newTestSuite(t, database)
	testSuit.registerAllSteps(sc)
	sc.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		// testSuit.database.TruncateAll()
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
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("godog tests failed")
	}
}
