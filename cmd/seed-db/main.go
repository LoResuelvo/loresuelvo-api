package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/LoResuelvo/loresuelvo-api/internal/bootstrap"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
)

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("database seeds failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	databaseConfig, err := db.NewPostgresConfigFromEnv()
	if err != nil {
		return fmt.Errorf("loading PostgreSQL configuration: %w", err)
	}

	database, err := db.ConnectPostgres(ctx, databaseConfig)
	if err != nil {
		return fmt.Errorf("connecting to PostgreSQL: %w", err)
	}

	if err := bootstrap.SeedDefaultDataFromEnv(ctx, database); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			slog.Error("closing PostgreSQL connection after seed failure", "error", closeErr)
		}
		return fmt.Errorf("seeding default data: %w", err)
	}

	if err := database.Close(); err != nil {
		return fmt.Errorf("closing PostgreSQL connection: %w", err)
	}

	return nil
}
