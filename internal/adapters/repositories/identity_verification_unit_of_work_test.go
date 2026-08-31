package repositories_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIdentityVerificationUnitOfWorkCommitsEventAndSessionTogether(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	unit := repositories.NewIdentityVerificationUnitOfWork(testContext.database, testContext.repository)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	verification, err := identityverification.NewVerification(testContext.providerID, uuid.New(), uuid.New(), "didit", 1, now.Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, testContext.repository.Save(t.Context(), verification))
	result := identityverification.VerificationResult{
		EventID: uuid.New(), SessionID: verification.ExternalSessionID, ProviderID: testContext.providerID,
		VendorData: identityverification.ProviderVendorData(testContext.providerID), WorkflowID: verification.WorkflowID,
		WorkflowVersion: verification.WorkflowVersion, Status: identityverification.StatusApproved, OccurredOn: now,
	}
	event, err := identityverification.NewVerificationEvent(result, now)
	require.NoError(t, err)

	err = unit.Execute(t.Context(), func(store identityverification.TransactionalStore) error {
		stored, saveErr := store.SaveEvent(t.Context(), event)
		if saveErr != nil {
			return saveErr
		}
		require.True(t, stored)
		if applyErr := verification.ApplyResult(result, now); applyErr != nil {
			return applyErr
		}
		return store.SaveVerification(t.Context(), verification)
	})

	require.NoError(t, err)
	found, err := testContext.repository.FindBySessionID(t.Context(), verification.ExternalSessionID)
	require.NoError(t, err)
	require.Equal(t, identityverification.StatusApproved, found.Status)
	count, err := testContext.repository.CountEventsByID(t.Context(), event.EventID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestIdentityVerificationUnitOfWorkDoesNotInsertDuplicateEvent(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	unit := repositories.NewIdentityVerificationUnitOfWork(testContext.database, testContext.repository)
	now := time.Now().UTC()
	verification, err := identityverification.NewVerification(testContext.providerID, uuid.New(), uuid.New(), "didit", 1, now)
	require.NoError(t, err)
	require.NoError(t, testContext.repository.Save(t.Context(), verification))
	event := &identityverification.VerificationEvent{EventID: uuid.New(), SessionID: verification.ExternalSessionID, OccurredOn: now, ReceivedOn: now}

	for _, expectedStored := range []bool{true, false} {
		err = unit.Execute(t.Context(), func(store identityverification.TransactionalStore) error {
			stored, saveErr := store.SaveEvent(t.Context(), event)
			require.Equal(t, expectedStored, stored)
			return saveErr
		})
		require.NoError(t, err)
	}
	count, err := testContext.repository.CountEventsByID(t.Context(), event.EventID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestIdentityVerificationUnitOfWorkRollsBackEventWhenOperationFails(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	unit := repositories.NewIdentityVerificationUnitOfWork(testContext.database, testContext.repository)
	now := time.Now().UTC()
	verification, err := identityverification.NewVerification(testContext.providerID, uuid.New(), uuid.New(), "didit", 1, now)
	require.NoError(t, err)
	require.NoError(t, testContext.repository.Save(t.Context(), verification))
	event := &identityverification.VerificationEvent{EventID: uuid.New(), SessionID: verification.ExternalSessionID, OccurredOn: now, ReceivedOn: now}
	expectedErr := errors.New("operation failed")

	err = unit.Execute(t.Context(), func(store identityverification.TransactionalStore) error {
		stored, saveErr := store.SaveEvent(t.Context(), event)
		require.NoError(t, saveErr)
		require.True(t, stored)
		return expectedErr
	})

	require.ErrorIs(t, err, expectedErr)
	count, err := testContext.repository.CountEventsByID(t.Context(), event.EventID)
	require.NoError(t, err)
	require.Zero(t, count)
}
