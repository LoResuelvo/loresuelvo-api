package db

import (
	"context"
	"database/sql"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultPostgresURL     = "postgres://postgres:postgres@localhost:5432/loresuelvo?sslmode=disable"
	defaultTestPostgresURL = "postgres://postgres:postgres@localhost:5432/loresuelvo_test?sslmode=disable"
)

type PostgresConfig struct {
	URL string
}

func NewPostgresConfigFromEnv() PostgresConfig {
	return PostgresConfig{
		URL: envOrDefault("DATABASE_URL", defaultPostgresURL),
	}
}

func NewTestPostgresConfigFromEnv() PostgresConfig {
	return PostgresConfig{
		URL: envOrDefault("TEST_DATABASE_URL", defaultTestPostgresURL),
	}
}

func ConnectPostgres(ctx context.Context, config PostgresConfig) (*sql.DB, error) {
	database, err := sql.Open("pgx", config.URL)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := database.PingContext(pingCtx); err != nil {
		_ = database.Close()
		return nil, err
	}

	return database, nil
}

func envOrDefault(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
