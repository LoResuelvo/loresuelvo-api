package repositories_test

import (
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCategoryUnitOfWorkTest(t *testing.T) (*repositories.CategoryUnitOfWork, *repositories.CategoryRepository, *repositories.AuditEventRepository, *sql.DB) {
	t.Helper()
	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	categories := repositories.NewCategoryRepository(database)
	auditEvents := repositories.NewAuditEventRepository(database)
	return repositories.NewCategoryUnitOfWork(database, categories, auditEvents), categories, auditEvents, database
}

func TestCategoryUnitOfWorkCommitsCategoryAndAuditEventTogether(t *testing.T) {
	unit, categories, auditEvents, database := newCategoryUnitOfWorkTest(t)
	ctx := context.Background()
	name := "Audited " + uuid.NewString()
	toSave, err := category.New(name)
	require.NoError(t, err)
	eventID := uuid.New()
	occurredOn := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	var saved *category.Category
	err = unit.Execute(ctx, func(store category.TransactionalStore) error {
		var saveErr error
		saved, saveErr = store.SaveCategory(ctx, *toSave)
		if saveErr != nil {
			return saveErr
		}
		event, eventErr := audit.NewEvent(audit.EventParams{
			ID: eventID, OperatorID: 71, Action: audit.ActionCreate,
			ResourceType: "category", ResourceID: strconv.Itoa(saved.ID),
			OccurredOn: occurredOn, Result: audit.ResultSucceeded,
			CorrelationID: "request-" + uuid.NewString(),
		})
		if eventErr != nil {
			return eventErr
		}
		return store.SaveAuditEvent(ctx, event)
	})
	require.NoError(t, err)
	require.NotNil(t, saved)
	t.Cleanup(func() {
		_, cleanupErr := database.ExecContext(context.Background(), `DELETE FROM categories WHERE id = $1`, saved.ID)
		require.NoError(t, cleanupErr)
	})
	assert.Equal(t, saved, categories.FindByID(saved.ID))
	persistedEvent, err := auditEvents.FindByID(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(saved.ID), persistedEvent.ResourceID())
	assert.Equal(t, audit.ResultSucceeded, persistedEvent.Result())
	assert.Equal(t, occurredOn, persistedEvent.OccurredOn())
}

func TestCategoryUnitOfWorkRejectsNilOperation(t *testing.T) {
	unit := repositories.NewCategoryUnitOfWork(nil, nil, nil)
	err := unit.Execute(context.Background(), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "operation is required")
}

func TestCategoryUnitOfWorkRollsBackOnOperationPanic(t *testing.T) {
	unit, _, _, database := newCategoryUnitOfWorkTest(t)
	database.SetMaxOpenConns(1)
	name := "Panic " + uuid.NewString()
	toSave, err := category.New(name)
	require.NoError(t, err)

	assert.PanicsWithValue(t, "simulated callback panic", func() {
		_ = unit.Execute(context.Background(), func(store category.TransactionalStore) error {
			_, saveErr := store.SaveCategory(context.Background(), *toSave)
			require.NoError(t, saveErr)
			panic("simulated callback panic")
		})
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var count int
	err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE normalized_name = $1`, toSave.NormalizedName).Scan(&count)
	require.NoError(t, err, "the connection must remain usable after a panic")
	assert.Zero(t, count, "the category inserted before the panic must be rolled back")
}
