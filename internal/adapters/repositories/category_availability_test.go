package repositories_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCategoryCheckedLookupPreservesStorageFailure(t *testing.T) {
	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	database, err := db.ConnectPostgres(t.Context(), config)
	require.NoError(t, err)
	require.NoError(t, database.Close())
	found, err := repositories.NewCategoryRepository(database).FindByID(t.Context(), 17)
	require.Error(t, err)
	require.NotErrorIs(t, err, category.ErrDoesNotExist)
	require.Nil(t, found)
}

func TestCategoryRenameConflictsWithDisabledCategory(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	first, err := repo.Save(validCategory())
	require.NoError(t, err)
	other, err := category.New("Electricidad")
	require.NoError(t, err)
	other.Enabled = false
	_, err = repo.Save(*other)
	require.NoError(t, err)
	changed, err := first.Edit(category.Edit{ExpectedVersion: 1, Name: new(" ELECTRICIDAD ")}, false)
	require.NoError(t, err)
	require.True(t, changed)
	_, err = repo.Save(*first)
	require.ErrorIs(t, err, category.ErrAlreadyExists)
	stored, err := repo.FindByID(t.Context(), first.ID)
	require.NoError(t, err)
	require.Equal(t, "Plomería", stored.Name)
	require.Equal(t, 1, stored.Version)
}

func TestCategoryUnitOfWorkRollsBackEditWhenAuditFails(t *testing.T) {
	unit, categories, events, _ := newCategoryUnitOfWorkTest(t)
	current, err := category.New("Rollback " + uuid.NewString())
	require.NoError(t, err)
	current, err = categories.Save(*current)
	require.NoError(t, err)
	original := *current
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: 71, Action: audit.ActionExecute, ResourceType: "category", ResourceID: strconv.Itoa(current.ID), OccurredOn: time.Now(), Result: audit.ResultSucceeded, CorrelationID: uuid.NewString()})
	require.NoError(t, err)
	require.NoError(t, events.Save(t.Context(), event))
	err = unit.Execute(t.Context(), func(store category.TransactionalStore) error {
		locked, err := store.FindCategory(t.Context(), current.ID)
		if err != nil {
			return err
		}
		if _, err := locked.Edit(category.Edit{ExpectedVersion: 1, Name: new("Updated " + uuid.NewString()), Enabled: new(false), Reason: "Retirar oferta"}, false); err != nil {
			return err
		}
		if _, err := store.SaveCategory(t.Context(), *locked); err != nil {
			return err
		}
		return store.SaveAuditEvent(t.Context(), event)
	})
	require.Error(t, err)
	stored, err := categories.FindByID(t.Context(), current.ID)
	require.NoError(t, err)
	require.Equal(t, original, *stored)
}

func TestJobRequestCreationHoldsCategoryEligibilityUntilCommit(t *testing.T) {
	fixture := newJobRequestRepositoryTest(t)
	consumerID, providerID := savedJobRequestParticipants(t, fixture)
	foundProvider, err := fixture.userRepository.FindProviderByID(t.Context(), providerID)
	require.NoError(t, err)
	unit := repositories.NewJobRequestCreationUnitOfWork(fixture.database, fixture.jobRequestRepository, fixture.categoryRepository, fixture.userRepository)
	editing := repositories.NewCategoryUnitOfWork(fixture.database, fixture.categoryRepository, repositories.NewAuditEventRepository(fixture.database))
	locked := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	saved := make(chan error, 1)
	go func() {
		saved <- unit.Execute(t.Context(), func(store jobrequest.CreationStore) error {
			selected, err := store.FindProviderCategory(t.Context(), providerID)
			if err != nil {
				return err
			}
			if err := selected.RequireEnabled(); err != nil {
				return err
			}
			close(locked)
			<-release
			request, err := jobrequest.New(consumerID, providerID, "Repair", "Leak", nil)
			if err != nil {
				return err
			}
			request.CreatedOn = time.Now()
			pending, err := conversation.NewPendingConversation(consumerID, providerID)
			if err != nil {
				return err
			}
			_, err = store.SaveWithConversation(t.Context(), *request, pending)
			return err
		})
	}()
	<-locked
	// The bounded attempt cannot acquire an exclusive category lock while creation holds SHARE.
	bounded, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	err = editing.Execute(bounded, func(store category.TransactionalStore) error {
		_, err := store.FindCategory(bounded, foundProvider.Category.ID)
		return err
	})
	cancel()
	require.Error(t, err)
	close(release)
	require.NoError(t, <-saved)
	err = editing.Execute(t.Context(), func(store category.TransactionalStore) error {
		current, err := store.FindCategory(t.Context(), foundProvider.Category.ID)
		if err != nil {
			return err
		}
		if _, err := current.Edit(category.Edit{ExpectedVersion: 1, Enabled: new(false), Reason: "Retirar oferta"}, false); err != nil {
			return err
		}
		_, err = store.SaveCategory(t.Context(), *current)
		return err
	})
	require.NoError(t, err)
	require.ErrorIs(t, unit.Execute(t.Context(), func(store jobrequest.CreationStore) error {
		current, err := store.FindProviderCategory(t.Context(), providerID)
		if err != nil {
			return err
		}
		return current.RequireEnabled()
	}), category.ErrDisabled)
	var count int
	require.NoError(t, fixture.database.QueryRow("SELECT count(*) FROM job_requests").Scan(&count))
	require.Equal(t, 1, count)
}

func TestProviderRegistrationChecksFreshDisabledCategory(t *testing.T) {
	fixture := newJobRequestRepositoryTest(t)
	toSave := validProviderWithData(t, fixture.categoryRepository, fixture.database, "auth0|fresh-provider", "fresh@example.com", "New", "Provider", "Plomeria")
	current := *toSave.Category
	current.Enabled = false
	_, err := fixture.categoryRepository.Save(current)
	require.NoError(t, err)
	unit := repositories.NewProviderRegistrationUnitOfWork(fixture.database, fixture.userRepository, fixture.categoryRepository)
	err = unit.Execute(t.Context(), func(store provider.RegistrationStore) error {
		current, err := store.FindCategory(t.Context(), toSave.Category.ID)
		if err != nil {
			return err
		}
		if err := current.RequireEnabled(); err != nil {
			return err
		}
		_, err = store.SaveUser(t.Context(), toSave)
		return err
	})
	require.ErrorIs(t, err, category.ErrDisabled)
	require.False(t, fixture.userRepository.FindByEmail(toSave.Email()))
}

func TestCategoryUnitOfWorkSerializesEditsUsingTheSameVersion(t *testing.T) {
	unit, categories, _, database := newCategoryUnitOfWorkTest(t)
	current, err := category.New("Concurrent " + uuid.NewString())
	require.NoError(t, err)
	current, err = categories.Save(*current)
	require.NoError(t, err)
	ready := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-ready
			results <- unit.Execute(t.Context(), func(store category.TransactionalStore) error {
				locked, err := store.FindCategory(t.Context(), current.ID)
				if err != nil {
					return err
				}
				if _, err := locked.Edit(category.Edit{ExpectedVersion: 1, Name: new("Renamed " + uuid.NewString())}, false); err != nil {
					return err
				}
				if _, err := store.SaveCategory(t.Context(), *locked); err != nil {
					return err
				}
				event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: 71, Action: audit.ActionExecute, ResourceType: "category", ResourceID: strconv.Itoa(current.ID), OccurredOn: time.Now(), Result: audit.ResultSucceeded, CorrelationID: uuid.NewString()})
				if err != nil {
					return err
				}
				return store.SaveAuditEvent(t.Context(), event)
			})
		}()
	}
	close(ready)
	first, second := <-results, <-results
	if first == nil {
		require.ErrorIs(t, second, category.ErrVersionConflict)
	} else {
		require.ErrorIs(t, first, category.ErrVersionConflict)
		require.NoError(t, second)
	}
	stored, err := categories.FindByID(t.Context(), current.ID)
	require.NoError(t, err)
	require.Equal(t, 2, stored.Version)
	var count int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM audit_events WHERE resource_type='category' AND resource_id=$1`, strconv.Itoa(current.ID)).Scan(&count))
	require.Equal(t, 1, count)
}
