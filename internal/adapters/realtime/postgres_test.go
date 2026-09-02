package realtime

import (
	"context"
	"database/sql"
	"testing"

	databaseadapter "github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
)

func openRealtimeTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	config, err := databaseadapter.NewTestPostgresConfigFromEnv()
	if err != nil {
		t.Fatalf("invalid test database configuration: %v", err)
	}
	database, err := databaseadapter.ConnectPostgres(context.Background(), config)
	if err != nil {
		t.Skipf("PostgreSQL is unavailable: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
