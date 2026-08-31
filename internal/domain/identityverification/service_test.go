package identityverification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestServiceStartCreatesSessionAfterVerifierSuccess(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	category := &category.Category{ID: 2, Name: "Plomeria"}
	foundProvider, err := provider.NewProvider("auth0|provider", "juan@example.com", "Juan", "Gomez", category, nil, []coveragezone.CoverageZone{{ID: 1, Enabled: true}})
	require.NoError(t, err)
	foundProvider.SetPersistenceID(7)
	finder := providerFinderStub{provider: foundProvider}
	repo := &verificationRepositoryStub{}
	sessionID, workflowID := uuid.New(), uuid.New()
	verifier := &verifierStub{credentials: SessionCredentials{SessionID: sessionID, SessionToken: "secret", VerificationURL: "https://verify.example", Status: StatusNotStarted, Verifier: "fake", WorkflowID: workflowID, WorkflowVersion: 1}}
	service := NewService(finder, repo, verifier, fixedClock{now})

	result, err := service.Start(context.Background(), foundProvider.AuthID())

	require.NoError(t, err)
	require.Equal(t, sessionID, result.Credentials.SessionID)
	require.Equal(t, foundProvider.ID(), repo.saved.ProviderID)
	require.Equal(t, ProviderVendorData(foundProvider.ID()), verifier.request.VendorData)
	require.Equal(t, foundProvider.Name(), verifier.request.FirstName)
	require.Equal(t, foundProvider.Surname(), verifier.request.LastName)
}

func TestServiceStartRejectsConsumer(t *testing.T) {
	service := NewService(providerFinderStub{err: errors.New("not found")}, &verificationRepositoryStub{}, &verifierStub{}, fixedClock{now: time.Now()})
	_, err := service.Start(context.Background(), "auth0|consumer")
	require.ErrorIs(t, err, ErrProviderRequired)
}

func TestServiceStartReusesInProgressSession(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	foundProvider, err := provider.NewProvider("auth0|provider", "juan@example.com", "Juan", "Gomez", &category.Category{ID: 2, Name: "Plomeria"}, nil, []coveragezone.CoverageZone{{ID: 1, Enabled: true}})
	require.NoError(t, err)
	foundProvider.SetPersistenceID(7)
	active, err := NewVerification(7, uuid.New(), uuid.New(), "fake", 1, now)
	require.NoError(t, err)
	active.Status = StatusInProgress
	repo := &verificationRepositoryStub{latest: active}
	verifier := &verifierStub{credentials: SessionCredentials{SessionID: active.ExternalSessionID, SessionToken: "secret", VerificationURL: "https://verify.example", Status: StatusNotStarted, Verifier: "fake", WorkflowID: active.WorkflowID, WorkflowVersion: active.WorkflowVersion}}
	service := NewService(providerFinderStub{provider: foundProvider}, repo, verifier, fixedClock{now})

	result, err := service.Start(context.Background(), foundProvider.AuthID())

	require.NoError(t, err)
	require.Equal(t, active.ExternalSessionID, result.Credentials.SessionID)
	require.Equal(t, active, result.Verification)
	require.NotNil(t, verifier.request.ExistingSessionID)
}

func TestServiceApplyResultMarksSessionInProgress(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sessionID, workflowID := uuid.New(), uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, now)
	require.NoError(t, err)
	repo := &verificationRepositoryStub{byID: verification}
	service := NewService(providerFinderStub{}, repo, &verifierStub{}, fixedClock{now})

	err = service.ApplyResult(context.Background(), VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
		WorkflowID: workflowID, WorkflowVersion: 1, Status: StatusInProgress,
		OccurredOn: now.Add(time.Minute),
	})

	require.NoError(t, err)
	require.Equal(t, StatusInProgress, repo.saved.Status)
	require.Equal(t, now, repo.saved.UpdatedOn)
}

func TestServiceApplyResultMarksSessionAwaitingUser(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sessionID, workflowID := uuid.New(), uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, now)
	require.NoError(t, err)
	repo := &verificationRepositoryStub{byID: verification}
	service := NewService(providerFinderStub{}, repo, &verifierStub{}, fixedClock{now})

	err = service.ApplyResult(context.Background(), VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
		WorkflowID: workflowID, WorkflowVersion: 1, Status: StatusAwaitingUser,
		OccurredOn: now.Add(time.Minute),
	})

	require.NoError(t, err)
	require.Equal(t, StatusAwaitingUser, repo.saved.Status)
}

func TestServiceApplyResultMarksSessionInReview(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sessionID, workflowID := uuid.New(), uuid.New()
	verification, err := NewVerification(7, sessionID, workflowID, "fake", 1, now)
	require.NoError(t, err)
	repo := &verificationRepositoryStub{byID: verification}
	service := NewService(providerFinderStub{}, repo, &verifierStub{}, fixedClock{now})

	err = service.ApplyResult(context.Background(), VerificationResult{
		EventID: uuid.New(), SessionID: sessionID, ProviderID: 7, VendorData: ProviderVendorData(7),
		WorkflowID: workflowID, WorkflowVersion: 1, Status: StatusInReview,
		OccurredOn: now.Add(time.Minute),
	})

	require.NoError(t, err)
	require.Equal(t, StatusInReview, repo.saved.Status)
}

func TestServiceCurrentStatusReturnsLatestVerificationStatus(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	verification, err := NewVerification(7, uuid.New(), uuid.New(), "fake", 1, now)
	require.NoError(t, err)
	verification.Status = StatusInProgress
	repo := &verificationRepositoryStub{latest: verification}
	service := NewService(providerFinderStub{}, repo, &verifierStub{}, fixedClock{now})

	status, err := service.CurrentStatus(context.Background(), verification.ProviderID)

	require.NoError(t, err)
	require.Equal(t, StatusInProgress, status)
}

func TestServiceCurrentStatusReturnsUnverifiedWithoutSessions(t *testing.T) {
	service := NewService(providerFinderStub{}, &verificationRepositoryStub{}, &verifierStub{}, fixedClock{time.Now().UTC()})

	status, err := service.CurrentStatus(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, StatusUnverified, status)
}

func TestServiceCurrentStatusDetailsReturnsVerificationDate(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	verification, err := NewVerification(7, uuid.New(), uuid.New(), "fake", 1, now)
	require.NoError(t, err)
	verification.Status = StatusApproved
	verification.VerifiedOn = &now
	repo := &verificationRepositoryStub{latest: verification}
	service := NewService(providerFinderStub{}, repo, &verifierStub{}, fixedClock{now})

	details, err := service.CurrentStatusDetails(context.Background(), verification.ProviderID)

	require.NoError(t, err)
	require.Equal(t, StatusApproved, details.Status)
	require.NotNil(t, details.VerifiedOn)
	require.Equal(t, now, *details.VerifiedOn)
}
