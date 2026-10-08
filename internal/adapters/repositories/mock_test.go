package repositories_test

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
)

// administrationRecordReadBarrier observes a real unit of work and coordinates
// its first two missing-record reads, forcing the cross-claim uniqueness race.
type administrationRecordReadBarrier struct {
	unit      claim.AdministrationUnitOfWork
	arrived   atomic.Int32
	conflicts atomic.Int32
	ready     chan struct{}
}

func (barrier *administrationRecordReadBarrier) Execute(ctx context.Context, operation func(claim.AdministrationStore) error) error {
	err := barrier.unit.Execute(ctx, func(store claim.AdministrationStore) error {
		return operation(&administrationRecordReadBarrierStore{AdministrationStore: store, barrier: barrier})
	})
	if errors.Is(err, claim.ErrAdministrationRecordConflict) {
		barrier.conflicts.Add(1)
	}
	return err
}

type administrationRecordReadBarrierStore struct {
	claim.AdministrationStore
	barrier *administrationRecordReadBarrier
}

func (store *administrationRecordReadBarrierStore) FindRecord(ctx context.Context, operator int, key string) (*claim.AdministrationRecord, error) {
	record, err := store.AdministrationStore.FindRecord(ctx, operator, key)
	if err != nil || record != nil {
		return record, err
	}
	if store.barrier.arrived.Add(1) == 2 {
		close(store.barrier.ready)
	}
	select {
	case <-store.barrier.ready:
		return record, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// administrationCommitBarrier holds only the first successful callback before
// its real PostgreSQL commit, so a replay must wait on the same claim row.
type administrationCommitBarrier struct {
	unit     claim.AdministrationUnitOfWork
	calls    atomic.Int32
	prepared chan struct{}
	release  chan struct{}
}

func (barrier *administrationCommitBarrier) Execute(ctx context.Context, operation func(claim.AdministrationStore) error) error {
	first := barrier.calls.Add(1) == 1
	return barrier.unit.Execute(ctx, func(store claim.AdministrationStore) error {
		if err := operation(store); err != nil {
			return err
		}
		if !first {
			return nil
		}
		close(barrier.prepared)
		select {
		case <-barrier.release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}
