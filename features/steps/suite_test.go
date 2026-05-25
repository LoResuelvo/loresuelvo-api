package steps_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0/testhelper"
	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/bootstrap"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/cucumber/godog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSuite struct {
	server             *httptest.Server
	database           *sql.DB
	consumerRepository *repositories.ConsumerRepository
	providerRepository *repositories.ProviderRepository
	userRepository     *repositories.UserRepository
	auth0Validator     *validator.Validator
	tokenBuilder       *testhelper.TokenBuilder
	lastStatus         int
	lastBody           []byte
	currentAuth0ID     string
}

func (s *testSuite) registerAllSteps(sc *godog.ScenarioContext) {
	registerHelloWorldSteps(sc, s)
	registerConsumerAccountSteps(sc, s)
	registerProviderAccountSteps(sc, s)
	registerCreateCategorySteps(sc, s)
	registerLoginSteps(sc, s)
}

func newTestDb() *sql.DB {
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	if err != nil {
		panic(fmt.Errorf("cannot connect to test database: %w", err))
	}
	return database
}

func newTestSuite(tb testing.TB, database *sql.DB) *testSuite {
	dependencies := bootstrap.NewDependencies(database)
	auth0Validator := testhelper.NewTestValidator(tb)
	tokenBuilder := testhelper.NewTokenBuilder()

	router := httpadapter.NewRouter(dependencies.ConsumerHandler, dependencies.ProviderHandler, dependencies.UserHandler, auth0Validator)
	engine, err := router.SetUp()
	require.NoError(tb, err, "could not initialize router")

	// httptest.Server wraps the engine — no port needed
	server := httptest.NewServer(engine)
	tb.Cleanup(func() {
		server.Close()
	})

	return &testSuite{
		server:             server,
		database:           database,
		consumerRepository: dependencies.ConsumerRepository,
		providerRepository: dependencies.ProviderRepository,
		userRepository:     dependencies.UserRepository,
		auth0Validator:     auth0Validator,
		tokenBuilder:       tokenBuilder,
	}
}

func ScenarioInitializer(sc *godog.ScenarioContext, t *testing.T, database *sql.DB) {
	testSuite := newTestSuite(t, database)
	testSuite.registerAllSteps(sc)
	sc.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		if err := testSuite.userRepository.DeleteAll(); err != nil {
			return ctx, fmt.Errorf("could not clean test database: %w", err)
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

	assert.Equal(t, 0, suite.Run(), "godog tests failed")
}
