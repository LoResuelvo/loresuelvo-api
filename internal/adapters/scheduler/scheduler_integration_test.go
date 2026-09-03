package scheduler

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/locking"
	databaseadapter "github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/require"
)

func openSchedulerTestDatabases(t *testing.T) (*sql.DB, *sql.DB) {
	t.Helper()

	config, err := databaseadapter.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	databaseA, err := databaseadapter.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to first scheduler test database")
	databaseB, err := databaseadapter.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to second scheduler test database")
	t.Cleanup(func() {
		_ = databaseA.Close()
		_ = databaseB.Close()
	})
	return databaseA, databaseB
}

func waitForSchedulerSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for scheduler task to enter its lock")
	}
}

func assertNotCalled(t *testing.T, called <-chan struct{}) {
	t.Helper()
	select {
	case <-called:
		t.Fatal("scheduler task ran while another instance owned its lock")
	default:
	}
}

func TestSchedulersSkipConcurrentCyclesAcrossPostgresPools(t *testing.T) {
	databaseA, databaseB := openSchedulerTestDatabases(t)
	lockA := locking.NewPostgresAdvisoryLock(databaseA)
	lockB := locking.NewPostgresAdvisoryLock(databaseB)
	firstTask := &blockingUrgentTask{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(firstTask.release) }) }
	defer release()
	firstScheduler := NewScheduler(
		time.Hour,
		firstTask,
		WithLock(lockA, UrgentNotificationLockKey),
	)
	firstDone := make(chan error, 1)
	go func() { firstDone <- firstScheduler.RunOnce(t.Context()) }()
	waitForSchedulerSignal(t, firstTask.started)

	secondTask := &countingUrgentTask{called: make(chan struct{}, 1)}
	secondScheduler := NewScheduler(
		time.Hour,
		secondTask,
		WithLock(lockB, UrgentNotificationLockKey),
	)
	require.NoError(t, secondScheduler.RunOnce(t.Context()))
	assertNotCalled(t, secondTask.called)

	release()
	require.NoError(t, <-firstDone)
}

func TestSchedulerLockKeysAreIndependentAcrossPostgresPools(t *testing.T) {
	databaseA, databaseB := openSchedulerTestDatabases(t)
	lockA := locking.NewPostgresAdvisoryLock(databaseA)
	lockB := locking.NewPostgresAdvisoryLock(databaseB)

	t.Run("calendar sync runs while urgent notifications are locked", func(t *testing.T) {
		firstTask := &blockingUrgentTask{
			started: make(chan struct{}),
			release: make(chan struct{}),
		}
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(firstTask.release) }) }
		defer release()
		firstScheduler := NewScheduler(
			time.Hour,
			firstTask,
			WithLock(lockA, UrgentNotificationLockKey),
		)
		firstDone := make(chan error, 1)
		go func() { firstDone <- firstScheduler.RunOnce(t.Context()) }()
		waitForSchedulerSignal(t, firstTask.started)

		calendarTask := &countingCalendarTask{called: make(chan struct{}, 1)}
		calendarRunner := NewCalendarSyncRunner(
			calendarTask,
			WithLock(lockB, CalendarSyncLockKey),
		)
		require.NoError(t, calendarRunner.RunOnce(t.Context()))
		select {
		case <-calendarTask.called:
		default:
			t.Fatal("calendar sync did not run while urgent notifications were locked")
		}

		release()
		require.NoError(t, <-firstDone)
	})

	t.Run("urgent notifications run while calendar sync is locked", func(t *testing.T) {
		firstTask := &blockingCalendarTask{
			started: make(chan struct{}),
			release: make(chan struct{}),
		}
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(firstTask.release) }) }
		defer release()
		firstRunner := NewCalendarSyncRunner(
			firstTask,
			WithLock(lockA, CalendarSyncLockKey),
		)
		firstDone := make(chan error, 1)
		go func() { firstDone <- firstRunner.RunOnce(t.Context()) }()
		waitForSchedulerSignal(t, firstTask.started)

		urgentTask := &countingUrgentTask{called: make(chan struct{}, 1)}
		urgentScheduler := NewScheduler(
			time.Hour,
			urgentTask,
			WithLock(lockB, UrgentNotificationLockKey),
		)
		require.NoError(t, urgentScheduler.RunOnce(t.Context()))
		select {
		case <-urgentTask.called:
		default:
			t.Fatal("urgent notifications did not run while calendar sync was locked")
		}

		release()
		require.NoError(t, <-firstDone)
	})
}
