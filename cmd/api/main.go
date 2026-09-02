package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/bootstrap"

	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/LoResuelvo/loresuelvo-api/internal/observability"

	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
)

func main() {
	logger, err := observability.NewLoggerFromEnv()
	if err != nil {
		panic(err)
	}
	slog.SetDefault(logger)
	logger.Info("application starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	databaseConfig, err := db.NewPostgresConfigFromEnv()
	if err != nil {
		panic(err)
	}
	database, err := db.ConnectPostgres(ctx, databaseConfig)
	if err != nil {
		panic(err)
	}
	defer database.Close()

	if err := bootstrap.SeedDefaultDataFromEnv(ctx, database); err != nil {
		panic(err)
	}

	dependencies, err := bootstrap.NewDependencies(database)
	if err != nil {
		panic(err)
	}
	defer dependencies.Close()
	go dependencies.UrgentWorkOrderScheduler.Run(ctx)

	auth0Validator, err := auth0.NewValidatorFromEnv()
	if err != nil {
		panic(err)
	}

	router := httpadapter.NewRouter(dependencies.RouterConfig(auth0Validator, logger, routerEnvironmentFromEnv()))
	engine, err := router.SetUp()
	if err != nil {
		panic(err)
	}

	if err := engine.Run(":8080"); err != nil {
		panic(err)
	}
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
