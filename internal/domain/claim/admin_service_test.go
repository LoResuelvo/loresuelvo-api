package claim

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/claim/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAdminDetailPersistsAccessBeforeSigningAndFailsClosed(t *testing.T) {
	for _, mode := range []string{"success", "audit failure", "second image failure", "second image unavailable", "read failure", "missing claim", "operator failure"} {
		t.Run(mode, func(t *testing.T) {
			reader, operators, writer, images, clock := new(adminReaderMock), new(operatorFinderMock), new(auditWriterMock), new(administrativeEvidenceImagesMock), new(clockMock)
			s := NewAdminService(reader, operators, nil, writer, images, clock)
			failure := errors.New("private failure")
			var operatorErr error
			if mode == "operator failure" {
				operatorErr = failure
			}
			operators.On("FindOperatorIDByAuthID", mock.Anything, "admin").Return(2, operatorErr).Once()
			if operatorErr == nil {
				found := &AdminDetail{GetResult: GetResult{Claim: &Claim{ID: 1, ImageFileIDs: []string{"first", "second"}}}}
				var readErr error
				if mode == "read failure" {
					readErr = failure
				}
				if mode == "missing claim" {
					found = nil
				}
				reader.On("FindAdministrativeByID", mock.Anything, 1).Return(found, readErr).Once()
				if found != nil && readErr == nil {
					now := time.Now()
					clock.On("Now").Return(now).Once()
					var auditErr error
					if mode == "audit failure" {
						auditErr = failure
					}
					persisted := false
					writer.On("Save", mock.Anything, mock.MatchedBy(func(event *audit.Event) bool {
						return event.Result() == audit.ResultPrepared && event.Action() == audit.ActionAccess && event.ResourceType() == "claim" && event.ResourceID() == "1" && event.Reason() == nil
					})).Run(func(mock.Arguments) { persisted = true }).Return(auditErr).Once()
					if auditErr == nil {
						images.On("ResolveAdministrativeClaimEvidenceImage", mock.Anything, "first").Run(func(mock.Arguments) { require.True(t, persisted) }).Return("https://private/first", nil).Once()
						var imageErr error
						if mode == "second image failure" {
							imageErr = failure
						}
						if mode == "second image unavailable" {
							imageErr = filedomain.ErrClaimEvidenceImageNotAvailable
						}
						images.On("ResolveAdministrativeClaimEvidenceImage", mock.Anything, "second").Return("https://private/second", imageErr).Once()
					}
				}
			}
			result, err := s.Get(context.Background(), "admin", 1, "correlation")
			if mode == "success" {
				require.NoError(t, err)
				require.Len(t, result.Images, 2)
			} else {
				require.Error(t, err)
				require.Nil(t, result)
			}
			if mode == "second image unavailable" {
				require.ErrorIs(t, err, ErrEvidenceAccessUnavailable)
				require.ErrorIs(t, err, filedomain.ErrClaimEvidenceImageNotAvailable)
			}
			for _, m := range []*mock.Mock{&reader.Mock, &operators.Mock, &writer.Mock, &images.Mock, &clock.Mock} {
				m.AssertExpectations(t)
			}
		})
	}
}

func TestAdminWriteStopsOnPersistenceFailureWithoutReturningPartialResult(t *testing.T) {
	for _, stage := range []string{"lookup", "record lookup", "claim", "audit", "record"} {
		t.Run(stage, func(t *testing.T) {
			operator, unit, store, clock := new(operatorFinderMock), new(administrationUnitOfWorkMock), new(administrationStoreMock), new(clockMock)
			svc := NewAdminService(nil, operator, unit, nil, nil, clock)
			key := uuid.NewString()
			failure := errors.New("persistence failure")
			operator.On("FindOperatorIDByAuthID", mock.Anything, "admin").Return(2, nil).Once()
			now := time.Now()
			found := &Claim{ID: 1, Status: StatusOpen, CreatedOn: now.Add(-time.Hour)}
			var lookupErr error
			if stage == "lookup" {
				lookupErr = failure
			}
			store.On("FindClaim", mock.Anything, 1).Return(found, lookupErr).Once()
			if lookupErr == nil {
				var recordErr error
				if stage == "record lookup" {
					recordErr = failure
				}
				store.On("FindRecord", mock.Anything, 2, key).Return((*AdministrationRecord)(nil), recordErr).Once()
				if recordErr == nil {
					clock.On("Now").Return(now).Once()
					var saveErr error
					if stage == "claim" {
						saveErr = failure
					}
					store.On("SaveClaim", mock.Anything, found).Run(func(mock.Arguments) { found.Actions[0].ID = 10 }).Return(saveErr).Once()
					if saveErr == nil {
						var auditErr error
						if stage == "audit" {
							auditErr = failure
						}
						store.On("SaveAuditEvent", mock.Anything, mock.Anything).Return(auditErr).Once()
						if auditErr == nil {
							store.On("SaveRecord", mock.Anything, mock.Anything).Return(failure).Once()
						}
					}
				}
			}
			var operationErr error
			unit.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) { operationErr = args.Get(1).(func(AdministrationStore) error)(store) }).Return(failure).Once()
			result, err := svc.StartReview(context.Background(), "admin", 1, key, "correlation")
			require.ErrorIs(t, operationErr, failure)
			require.ErrorIs(t, err, failure)
			require.Nil(t, result)
			for _, m := range []*mock.Mock{&operator.Mock, &unit.Mock, &store.Mock, &clock.Mock} {
				m.AssertExpectations(t)
			}
		})
	}
}

func TestAdminReplayReturnsOriginalActionAndCurrentFinalState(t *testing.T) {
	operator, unit, store := new(operatorFinderMock), new(administrationUnitOfWorkMock), new(administrationStoreMock)
	svc := NewAdminService(nil, operator, unit, nil, nil, nil)
	key := uuid.NewString()
	action := Action{ID: 3, Type: "review_started", ActorID: 2, ActorParty: PartyOperator, CreatedOn: time.Now()}
	found := &Claim{ID: 1, Status: StatusResolved, Actions: []Action{action}}
	operator.On("FindOperatorIDByAuthID", mock.Anything, "admin").Return(2, nil).Once()
	store.On("FindClaim", mock.Anything, 1).Return(found, nil).Once()
	store.On("FindRecord", mock.Anything, 2, key).Return(&AdministrationRecord{ClaimID: 1, ActionID: 3, Fingerprint: administrationFingerprint(1, "review_started", ResolutionInput{})}, nil).Once()
	unit.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) { require.NoError(t, args.Get(1).(func(AdministrationStore) error)(store)) }).Return(nil).Once()
	result, err := svc.StartReview(context.Background(), "admin", 1, key, "new-correlation")
	require.NoError(t, err)
	require.Equal(t, StatusResolved, result.Claim.Status)
	require.Equal(t, action, result.Action)
	store.AssertNotCalled(t, "SaveClaim", mock.Anything, mock.Anything)
	store.AssertNotCalled(t, "SaveAuditEvent", mock.Anything, mock.Anything)
	for _, m := range []*mock.Mock{&operator.Mock, &unit.Mock, &store.Mock} {
		m.AssertExpectations(t)
	}
}

func TestAdminListComputesAgeFromOneObservationWithoutAudit(t *testing.T) {
	reader, clock := new(adminReaderMock), new(clockMock)
	observed := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	page := &AdminPage{Claims: []readmodel.AdminClaimSummary{{ClaimSummary: readmodel.ClaimSummary{CreatedOn: observed.Add(-26 * time.Hour)}}}}
	reader.On("FindAdministrativePage", mock.Anything, AdminCriteria{ListCriteria: ListCriteria{Page: 1, Limit: 20}}).Return(page, nil).Once()
	clock.On("Now").Return(observed).Once()
	svc := NewAdminService(reader, nil, nil, nil, nil, clock)
	result, err := svc.List(context.Background(), AdminCriteria{})
	require.NoError(t, err)
	require.Equal(t, int64(26*3600), result.Claims[0].AgeSeconds)
	reader.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestAdminListDoesNotTurnReadFailureIntoEmptyResults(t *testing.T) {
	reader := new(adminReaderMock)
	failure := errors.New("database failure")
	reader.On("FindAdministrativePage", mock.Anything, mock.Anything).Return((*AdminPage)(nil), failure).Once()
	result, err := NewAdminService(reader, nil, nil, nil, nil, nil).List(context.Background(), AdminCriteria{})
	require.ErrorIs(t, err, failure)
	require.Nil(t, result)
	reader.AssertExpectations(t)
}
