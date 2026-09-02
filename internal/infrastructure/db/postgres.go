package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultPostgresURL     = "postgres://postgres:postgres@localhost:5432/loresuelvo?sslmode=disable"
	defaultTestPostgresURL = "postgres://postgres:postgres@localhost:5432/loresuelvo_test?sslmode=disable"
	defaultMaxOpenConns    = 20
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute

	dbMaxOpenConnsEnv    = "DB_MAX_OPEN_CONNS"
	dbMaxIdleConnsEnv    = "DB_MAX_IDLE_CONNS"
	dbConnMaxLifetimeEnv = "DB_CONN_MAX_LIFETIME"
	dbConnMaxIdleTimeEnv = "DB_CONN_MAX_IDLE_TIME"
)

type PostgresConfig struct {
	URL             string
	Logger          *slog.Logger
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func NewPostgresConfigFromEnv() (PostgresConfig, error) {
	return newPostgresConfigFromEnv("DATABASE_URL", defaultPostgresURL)
}

func NewTestPostgresConfigFromEnv() (PostgresConfig, error) {
	return newPostgresConfigFromEnv("TEST_DATABASE_URL", defaultTestPostgresURL)
}

func ConnectPostgres(ctx context.Context, config PostgresConfig) (*sql.DB, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating PostgreSQL configuration: %w", err)
	}
	connectionConfig, err := pgx.ParseConfig(config.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing PostgreSQL URL: %w", err)
	}
	connectionConfig.Tracer = NewQueryTracer(config.Logger)
	database := stdlib.OpenDB(*connectionConfig)
	configurePool(database, config)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := database.PingContext(pingCtx); err != nil {
		_ = database.Close()
		return nil, err
	}

	return database, nil
}

func (config PostgresConfig) Validate() error {
	if strings.TrimSpace(config.URL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if config.MaxOpenConns <= 0 {
		return fmt.Errorf("%s must be greater than zero", dbMaxOpenConnsEnv)
	}
	if config.MaxIdleConns < 0 {
		return fmt.Errorf("%s must not be negative", dbMaxIdleConnsEnv)
	}
	if config.MaxIdleConns > config.MaxOpenConns {
		return fmt.Errorf("%s must not exceed %s", dbMaxIdleConnsEnv, dbMaxOpenConnsEnv)
	}
	if config.ConnMaxLifetime < 0 {
		return fmt.Errorf("%s must not be negative", dbConnMaxLifetimeEnv)
	}
	if config.ConnMaxIdleTime < 0 {
		return fmt.Errorf("%s must not be negative", dbConnMaxIdleTimeEnv)
	}
	return nil
}

type poolConfigurer interface {
	SetMaxOpenConns(int)
	SetMaxIdleConns(int)
	SetConnMaxLifetime(time.Duration)
	SetConnMaxIdleTime(time.Duration)
}

func configurePool(database poolConfigurer, config PostgresConfig) {
	database.SetMaxOpenConns(config.MaxOpenConns)
	database.SetMaxIdleConns(config.MaxIdleConns)
	database.SetConnMaxLifetime(config.ConnMaxLifetime)
	database.SetConnMaxIdleTime(config.ConnMaxIdleTime)
}

func newPostgresConfigFromEnv(urlKey, defaultURL string) (PostgresConfig, error) {
	maxOpenConns, err := intEnvOrDefault(dbMaxOpenConnsEnv, defaultMaxOpenConns, 1)
	if err != nil {
		return PostgresConfig{}, err
	}
	maxIdleConns, err := intEnvOrDefault(dbMaxIdleConnsEnv, defaultMaxIdleConns, 0)
	if err != nil {
		return PostgresConfig{}, err
	}
	connMaxLifetime, err := durationEnvOrDefault(dbConnMaxLifetimeEnv, defaultConnMaxLifetime)
	if err != nil {
		return PostgresConfig{}, err
	}
	connMaxIdleTime, err := durationEnvOrDefault(dbConnMaxIdleTimeEnv, defaultConnMaxIdleTime)
	if err != nil {
		return PostgresConfig{}, err
	}
	config := PostgresConfig{
		URL:             envOrDefault(urlKey, defaultURL),
		Logger:          slog.Default(),
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,
	}
	if err := config.Validate(); err != nil {
		return PostgresConfig{}, fmt.Errorf("validating PostgreSQL configuration: %w", err)
	}
	return config, nil
}

func intEnvOrDefault(key string, defaultValue, minimum int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: must be an integer: %w", key, err)
	}
	if parsed < minimum {
		return 0, fmt.Errorf("parsing %s: must be at least %d", key, minimum)
	}
	return parsed, nil
}

func durationEnvOrDefault(key string, defaultValue time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}
	if value == "0" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: must be a duration such as 30s or 5m: %w", key, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("parsing %s: must not be negative", key)
	}
	return parsed, nil
}

func envOrDefault(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
