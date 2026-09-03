package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/lifecycle"
)

type Application struct {
	coordinator *lifecycle.Coordinator
}

func NewApplication(ctx context.Context, database *sql.DB, logger *slog.Logger) (*Application, error) {
	if err := SeedDefaultDataFromEnv(ctx, database); err != nil {
		return nil, fmt.Errorf("seeding default data: %w", err)
	}

	dependencies, err := NewDependencies(database)
	if err != nil {
		return nil, fmt.Errorf("building application dependencies: %w", err)
	}
	auth0Validator, err := auth0.NewValidatorFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring Auth0 validator: %w", err)
	}

	router := httpadapter.NewRouter(dependencies.RouterConfig(auth0Validator, logger, routerEnvironmentFromEnv()))
	engine, err := router.SetUp()
	if err != nil {
		return nil, fmt.Errorf("setting up HTTP router: %w", err)
	}

	server := &http.Server{Addr: ":8080", Handler: engine}
	coordinator, err := lifecycle.NewCoordinator(dependencies.Runtime.lifecycleConfig(server, database, logger))
	if err != nil {
		return nil, fmt.Errorf("configuring application lifecycle: %w", err)
	}
	return &Application{coordinator: coordinator}, nil
}

func (application *Application) Run(ctx context.Context) error {
	return application.coordinator.Run(ctx)
}

func routerEnvironmentFromEnv() httpadapter.Environment {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT"))) {
	case "dev", "development":
		return httpadapter.DevelopmentEnvironment
	case "test":
		return httpadapter.TestEnvironment
	case "staging":
		return httpadapter.StagingEnvironment
	case "production":
		return httpadapter.ProductionEnvironment
	default:
		return ""
	}
}
