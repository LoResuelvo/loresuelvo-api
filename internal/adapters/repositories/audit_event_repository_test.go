package repositories

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func newAuditRepositoryTest(t *testing.T) (*sql.DB, *AuditEventRepository) {
	t.Helper()
	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	return database, NewAuditEventRepository(database)
}

func newAuditRepositoryEvent(t *testing.T, id uuid.UUID) *audit.Event {
	t.Helper()
	event, err := audit.NewEvent(audit.EventParams{
		ID: id, OperatorID: 42, Action: audit.ActionCreate,
		ResourceType: "category", ResourceID: "123",
		OccurredOn: time.Date(2026, 9, 23, 12, 30, 1, 123456000, time.FixedZone("ART", -3*3600)),
		Result:     audit.ResultSucceeded, CorrelationID: "request-42",
	})
	require.NoError(t, err)
	return event
}

func TestAuditEventRepositorySaveAndReadBack(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	id := uuid.New()
	reason, err := audit.NewReason("Approved by an administrator")
	require.NoError(t, err)
	change, err := audit.NewStateChange("status", "pending", "active")
	require.NoError(t, err)
	event, err := audit.NewEvent(audit.EventParams{
		ID: id, OperatorID: 7, Action: audit.ActionExecute,
		ResourceType: "category", ResourceID: "category:12",
		OccurredOn: time.Date(2026, 9, 23, 12, 30, 1, 123456000, time.FixedZone("ART", -3*3600)),
		Result:     audit.ResultSucceeded, CorrelationID: "request-42",
		Reason: reason, StateChange: change,
	})
	require.NoError(t, err)

	require.NoError(t, repository.Save(context.Background(), event))
	loaded, err := repository.FindByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, event.ID(), loaded.ID())
	require.Equal(t, event.OperatorID(), loaded.OperatorID())
	require.Equal(t, event.Action(), loaded.Action())
	require.Equal(t, event.ResourceType(), loaded.ResourceType())
	require.Equal(t, event.ResourceID(), loaded.ResourceID())
	require.Equal(t, event.OccurredOn(), loaded.OccurredOn())
	require.Equal(t, time.UTC, loaded.OccurredOn().Location())
	require.Equal(t, event.Result(), loaded.Result())
	require.Equal(t, event.CorrelationID(), loaded.CorrelationID())
	require.Equal(t, reason.Text(), loaded.Reason().Text())
	require.Equal(t, change.Field(), loaded.StateChange().Field())
	require.Equal(t, change.From(), loaded.StateChange().From())
	require.Equal(t, change.To(), loaded.StateChange().To())
}

func TestAuditEventRepositoryReadsAbsentOptionalMetadata(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	event := newAuditRepositoryEvent(t, uuid.New())
	require.NoError(t, repository.Save(context.Background(), event))
	loaded, err := repository.FindByID(context.Background(), event.ID())
	require.NoError(t, err)
	require.Nil(t, loaded.Reason())
	require.Nil(t, loaded.StateChange())
}

func TestAuditEventRepositoryDuplicateIDAndPersistenceErrors(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	event := newAuditRepositoryEvent(t, uuid.New())
	require.NoError(t, repository.Save(context.Background(), event))
	err := repository.Save(context.Background(), event)
	require.ErrorIs(t, err, audit.ErrPersistence)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23505", pgErr.Code)

	_, err = repository.FindByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, audit.ErrNotFound)

	closedDB, _ := newAuditRepositoryTest(t)
	require.NoError(t, closedDB.Close())
	closedRepository := NewAuditEventRepository(closedDB)
	err = closedRepository.Save(context.Background(), newAuditRepositoryEvent(t, uuid.New()))
	require.ErrorIs(t, err, audit.ErrPersistence)
	_, err = closedRepository.FindByID(context.Background(), event.ID())
	require.ErrorIs(t, err, audit.ErrPersistence)
}

func TestAuditEventRepositoryRespectsContextCancellation(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := repository.Save(ctx, newAuditRepositoryEvent(t, uuid.New()))
	require.ErrorIs(t, err, audit.ErrPersistence)
	require.ErrorIs(t, err, context.Canceled)
	_, err = repository.FindByID(ctx, uuid.New())
	require.ErrorIs(t, err, audit.ErrPersistence)
	require.ErrorIs(t, err, context.Canceled)
}

func TestAuditEventRepositorySupportsConcurrentDistinctWrites(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	const count = 12
	ids := make([]uuid.UUID, count)
	events := make([]*audit.Event, count)
	for i := range ids {
		ids[i] = uuid.New()
		events[i] = newAuditRepositoryEvent(t, ids[i])
	}
	var wg sync.WaitGroup
	errorsByIndex := make([]error, count)
	for i, event := range events {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errorsByIndex[i] = repository.Save(context.Background(), event)
		}()
	}
	wg.Wait()
	for i, err := range errorsByIndex {
		require.NoError(t, err, "writer %d", i)
		loaded, err := repository.FindByID(context.Background(), ids[i])
		require.NoError(t, err)
		require.Equal(t, ids[i], loaded.ID())
	}
}

func TestAuditEventRepositoryParticipatesInCallerTransaction(t *testing.T) {
	database, repository := newAuditRepositoryTest(t)
	conn, err := database.Conn(context.Background())
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()
	_, err = conn.ExecContext(context.Background(), `CREATE TEMP TABLE audit_transaction_probe (marker TEXT PRIMARY KEY)`)
	require.NoError(t, err)

	for _, tc := range []struct {
		name   string
		commit bool
	}{
		{name: "commit", commit: true},
		{name: "rollback", commit: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			tx, err := conn.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			require.NoError(t, repository.saveWithExecutor(context.Background(), tx, newAuditRepositoryEvent(t, id)))
			_, err = tx.ExecContext(context.Background(), `INSERT INTO audit_transaction_probe (marker) VALUES ($1)`, tc.name)
			require.NoError(t, err)
			if tc.commit {
				require.NoError(t, tx.Commit())
			} else {
				require.NoError(t, tx.Rollback())
			}

			var auditCount, probeCount int
			require.NoError(t, conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM audit_events WHERE id = $1`, id).Scan(&auditCount))
			require.NoError(t, conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM audit_transaction_probe WHERE marker = $1`, tc.name).Scan(&probeCount))
			want := 0
			if tc.commit {
				want = 1
			}
			require.Equal(t, want, auditCount)
			require.Equal(t, want, probeCount)
		})
	}
}

func TestAuditEventRepositoryExposesNoMutationMethod(t *testing.T) {
	typ := reflect.TypeOf((*AuditEventRepository)(nil))
	methods := make([]string, 0, typ.NumMethod())
	for i := range typ.NumMethod() {
		methods = append(methods, typ.Method(i).Name)
	}
	require.ElementsMatch(t, []string{"Save", "FindByID", "FindLatest"}, methods)
	var _ audit.Writer = (*AuditEventRepository)(nil)
	var _ audit.Reader = (*AuditEventRepository)(nil)
	var _ audit.LogReader = (*AuditEventRepository)(nil)
}

func TestAuditEventRepositoryRejectsNilEvent(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	err := repository.Save(context.Background(), nil)
	require.True(t, errors.Is(err, audit.ErrInvalidEvent))
}

func TestAuditEventRepositoryFindLatestFiltersOrdersBoundsAndRehydrates(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	operatorID := 1_000_000_000 + rand.IntN(100_000_000)
	base := time.Now().UTC().AddDate(100, 0, 0)
	makeEvent := func(id uuid.UUID, operator int, at time.Time) *audit.Event {
		event, err := audit.NewEvent(audit.EventParams{
			ID: id, OperatorID: operator, Action: audit.ActionCreate,
			ResourceType: "category", ResourceID: "17", OccurredOn: at,
			Result: audit.ResultSucceeded, CorrelationID: "query-test",
		})
		require.NoError(t, err)
		return event
	}
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	otherOperatorEventID := uuid.New()
	if ids[1].String() < ids[2].String() {
		ids[1], ids[2] = ids[2], ids[1]
	}
	for _, event := range []*audit.Event{
		makeEvent(ids[0], operatorID, base.Add(time.Minute)),
		makeEvent(ids[1], operatorID, base),
		makeEvent(ids[2], operatorID, base),
		makeEvent(otherOperatorEventID, operatorID+1, base.Add(time.Hour)),
	} {
		require.NoError(t, repository.Save(context.Background(), event))
	}

	events, err := repository.FindLatest(context.Background(), audit.LogFilter{OperatorID: &operatorID}, 2)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, []uuid.UUID{ids[0], ids[1]}, []uuid.UUID{events[0].ID(), events[1].ID()})
	require.Equal(t, operatorID, events[0].OperatorID())
	require.Equal(t, "category", events[0].ResourceType())
	require.Equal(t, "17", events[0].ResourceID())
	require.Equal(t, "query-test", events[0].CorrelationID())

	all, err := repository.FindLatest(context.Background(), audit.LogFilter{OperatorID: &operatorID}, 20)
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Equal(t, []uuid.UUID{ids[0], ids[1], ids[2]}, []uuid.UUID{all[0].ID(), all[1].ID(), all[2].ID()})

	allOperators, err := repository.FindLatest(context.Background(), audit.LogFilter{}, 1)
	require.NoError(t, err)
	require.Len(t, allOperators, 1)
	require.Equal(t, otherOperatorEventID, allOperators[0].ID())
}

func TestAuditEventRepositoryFindLatestReturnsNonNilEmptyCollection(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	operatorID := 1_000_000_000 + rand.IntN(100_000_000)
	events, err := repository.FindLatest(context.Background(), audit.LogFilter{OperatorID: &operatorID}, 20)
	require.NoError(t, err)
	require.NotNil(t, events)
	require.Empty(t, events)
}

func TestAuditEventRepositoryFindLatestRejectsInvalidArgumentsAndReadFailures(t *testing.T) {
	database, repository := newAuditRepositoryTest(t)
	invalidOperatorID := 0
	tooLargeOperatorID := int(int64(1 << 31))
	for _, tc := range []struct {
		operatorID *int
		limit      int
	}{
		{nil, 0}, {nil, 101}, {&invalidOperatorID, 20}, {&tooLargeOperatorID, 20},
	} {
		_, err := repository.FindLatest(context.Background(), audit.LogFilter{OperatorID: tc.operatorID}, tc.limit)
		require.ErrorIs(t, err, audit.ErrInvalidQuery)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := repository.FindLatest(ctx, audit.LogFilter{}, 20)
	require.ErrorIs(t, err, audit.ErrPersistence)
	require.ErrorIs(t, err, context.Canceled)

	require.NoError(t, database.Close())
	_, err = repository.FindLatest(context.Background(), audit.LogFilter{}, 20)
	require.ErrorIs(t, err, audit.ErrPersistence)
}

func TestAuditEventRepositoryFindLatestCombinesEveryFilterWithAND(t *testing.T) {
	_, repository := newAuditRepositoryTest(t)
	ctx := context.Background()
	operatorID := 1_000_000_000 + rand.IntN(100_000_000)
	base := time.Now().UTC().AddDate(100, 0, 0).Truncate(time.Second)
	end := base.Add(time.Hour)
	art := time.FixedZone("ART", -3*3600)
	baseWithOffset := base.In(art)
	endWithOffset := end.In(art)
	action := audit.ActionExecute
	resourceType := "payment"
	resourceID := "42"
	result := audit.ResultSucceeded

	type fixture struct {
		name         string
		operatorID   int
		action       audit.Action
		resourceType string
		resourceID   string
		result       audit.Result
		occurredOn   time.Time
	}
	fixtures := []fixture{
		{"A", operatorID, action, resourceType, resourceID, result, base},
		{"B", operatorID + 1, action, resourceType, resourceID, result, base},
		{"C", operatorID, audit.ActionCreate, resourceType, resourceID, result, base},
		{"D", operatorID, action, "category", resourceID, result, base},
		{"E", operatorID, action, resourceType, "43", result, base},
		{"F", operatorID, action, resourceType, resourceID, audit.ResultFailed, base},
		{"G", operatorID, action, resourceType, resourceID, result, end},
		{"H", operatorID, action, resourceType, resourceID, result, base.Add(-time.Second)},
	}
	ids := make(map[string]uuid.UUID, len(fixtures))
	for _, fixture := range fixtures {
		id := uuid.New()
		ids[fixture.name] = id
		event, err := audit.NewEvent(audit.EventParams{
			ID: id, OperatorID: fixture.operatorID, Action: fixture.action,
			ResourceType: fixture.resourceType, ResourceID: fixture.resourceID,
			OccurredOn: fixture.occurredOn, Result: fixture.result,
			CorrelationID: "filter-test-" + fixture.name,
		})
		require.NoError(t, err)
		require.NoError(t, repository.Save(ctx, event))
	}

	for _, tc := range []struct {
		name   string
		filter audit.LogFilter
		want   []string
	}{
		{"operator", audit.LogFilter{OperatorID: &operatorID}, []string{"A", "C", "D", "E", "F", "G", "H"}},
		{"action", audit.LogFilter{OperatorID: &operatorID, Action: &action}, []string{"A", "D", "E", "F", "G", "H"}},
		{"resource type", audit.LogFilter{OperatorID: &operatorID, ResourceType: &resourceType}, []string{"A", "C", "E", "F", "G", "H"}},
		{"resource ID", audit.LogFilter{OperatorID: &operatorID, ResourceID: &resourceID}, []string{"A", "C", "D", "F", "G", "H"}},
		{"result", audit.LogFilter{OperatorID: &operatorID, Result: &result}, []string{"A", "C", "D", "E", "G", "H"}},
		{"inclusive start", audit.LogFilter{OperatorID: &operatorID, OccurredFrom: &base}, []string{"A", "C", "D", "E", "F", "G"}},
		{"exclusive end", audit.LogFilter{OperatorID: &operatorID, OccurredTo: &end}, []string{"A", "C", "D", "E", "F", "H"}},
		{"all dimensions", audit.LogFilter{OperatorID: &operatorID, Action: &action, ResourceType: &resourceType, ResourceID: &resourceID, Result: &result, OccurredFrom: &base, OccurredTo: &end}, []string{"A"}},
		{"offset-equivalent window", audit.LogFilter{OperatorID: &operatorID, Action: &action, ResourceType: &resourceType, ResourceID: &resourceID, Result: &result, OccurredFrom: &baseWithOffset, OccurredTo: &endWithOffset}, []string{"A"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events, err := repository.FindLatest(ctx, tc.filter, 20)
			require.NoError(t, err)
			actual := make([]uuid.UUID, 0, len(events))
			for _, event := range events {
				actual = append(actual, event.ID())
			}
			want := make([]uuid.UUID, 0, len(tc.want))
			for _, name := range tc.want {
				want = append(want, ids[name])
			}
			require.ElementsMatch(t, want, actual)
		})
	}

	prepared := audit.ResultPrepared
	created := audit.ActionCreate
	events, err := repository.FindLatest(ctx, audit.LogFilter{OperatorID: &operatorID, Action: &created, Result: &prepared}, 20)
	require.NoError(t, err)
	require.Empty(t, events)
}
