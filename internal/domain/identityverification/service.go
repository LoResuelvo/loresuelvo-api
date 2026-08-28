package identityverification

import (
	"context"
	"fmt"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
)

type StartResult struct {
	Verification *IdentityVerification
	Credentials  SessionCredentials
}

type Service struct {
	providerFinder ProviderFinder
	repository     VerificationRepository
	verifier       IdentityVerifier
	clock          clock.Clock
}

func NewService(providerFinder ProviderFinder, repository VerificationRepository, verifier IdentityVerifier, systemClock clock.Clock) *Service {
	return &Service{providerFinder: providerFinder, repository: repository, verifier: verifier, clock: systemClock}
}

func (service *Service) Start(ctx context.Context, authID string) (StartResult, error) {
	provider, err := service.providerFinder.FindProviderByAuthID(strings.TrimSpace(authID))
	if err != nil {
		return StartResult{}, ErrProviderRequired
	}
	latest, err := service.repository.FindLatestByProviderID(ctx, provider.ID())
	if err != nil {
		return StartResult{}, fmt.Errorf("finding latest identity verification: %w", err)
	}
	request := SessionRequest{ProviderID: provider.ID(), VendorData: ProviderVendorData(provider.ID()), FirstName: provider.Name(), LastName: provider.Surname()}
	if latest != nil && latest.Status == StatusInProgress {
		sessionID := latest.ExternalSessionID
		request.ExistingSessionID = &sessionID
	}
	credentials, err := service.verifier.CreateSession(ctx, request)
	if err != nil {
		return StartResult{}, err
	}
	if latest != nil && latest.Status == StatusInProgress {
		return StartResult{Verification: latest, Credentials: credentials}, nil
	}
	verification, err := NewVerification(provider.ID(), credentials.SessionID, credentials.WorkflowID, credentials.Verifier, credentials.WorkflowVersion, service.clock.Now())
	if err != nil {
		return StartResult{}, ErrVerifierMisconfigured
	}
	if err := service.repository.Save(ctx, verification); err != nil {
		return StartResult{}, fmt.Errorf("saving identity verification: %w", err)
	}
	return StartResult{Verification: verification, Credentials: credentials}, nil
}
