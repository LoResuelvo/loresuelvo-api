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
