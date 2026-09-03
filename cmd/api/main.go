package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/LoResuelvo/loresuelvo-api/internal/bootstrap"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/LoResuelvo/loresuelvo-api/internal/observability"
)

func main() {
	logger, err := observability.NewLoggerFromEnv()
	if err != nil {
		panic(err)
	}
	slog.SetDefault(logger)
	logger.Info("application starting")

	startupContext := context.Background()
	databaseConfig, err := db.NewPostgresConfigFromEnv()
	if err != nil {
		panic(err)
	}
	database, err := db.ConnectPostgres(startupContext, databaseConfig)
	if err != nil {
		panic(err)
	}

	application, err := bootstrap.NewApplication(startupContext, database, logger)
	if err != nil {
		panic(err)
	}

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	if err := application.Run(signalContext); err != nil {
		panic(err)
	}
}
