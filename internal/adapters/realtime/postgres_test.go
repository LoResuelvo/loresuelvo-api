package realtime

import (
	"context"
	"database/sql"
	"testing"

	databaseadapter "github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
)

func openRealtimeTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database, err := databaseadapter.ConnectPostgres(context.Background(), databaseadapter.NewTestPostgresConfigFromEnv())
	if err != nil {
		t.Skipf("PostgreSQL is unavailable: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
