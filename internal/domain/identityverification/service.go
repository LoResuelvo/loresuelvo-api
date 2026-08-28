package identityverification

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
)

type StartResult struct {
	Verification *IdentityVerification
	Credentials  SessionCredentials
}

type Service struct {
	providerFinder ProviderFinder
	repository     VerificationRepository
	verifier       IdentityVerifier
	unitOfWork     IdentityVerificationUnitOfWork
	clock          clock.Clock
}

func NewService(providerFinder ProviderFinder, repository VerificationRepository, verifier IdentityVerifier, unitOfWork IdentityVerificationUnitOfWork, systemClock clock.Clock) *Service {
	return &Service{providerFinder: providerFinder, repository: repository, verifier: verifier, unitOfWork: unitOfWork, clock: systemClock}
}

func (service *Service) Start(ctx context.Context, authID string) (StartResult, error) {
	provider, err := service.providerFinder.FindProviderByAuthID(ctx, strings.TrimSpace(authID))
	if err != nil {
		return StartResult{}, ErrProviderRequired
	}

	latest, err := service.repository.FindLatestByProviderID(ctx, provider.ID())
	if err != nil {
		return StartResult{}, fmt.Errorf("finding latest identity verification: %w", err)
	}
	if latest != nil {
		if latest.Status == StatusApproved {
			return StartResult{}, ErrVerificationAlreadyApproved
		}
		if latest.Status == StatusInReview {
			return StartResult{}, ErrVerificationInReview
		}
	}

	request := SessionRequest{
		ProviderID: provider.ID(),
		VendorData: providerVendorData(provider.ID()),
		FirstName:  provider.Name(),
		LastName:   provider.Surname(),
	}
	if latest != nil && latest.Status.IsReusable() {
		sessionID := latest.ExternalSessionID
		request.ExistingSessionID = &sessionID
	}
	credentials, err := service.verifier.CreateSession(ctx, request)
	if err != nil {
		return StartResult{}, err
	}
	if credentials.SessionID == uuid.Nil || strings.TrimSpace(credentials.SessionToken) == "" || strings.TrimSpace(credentials.VerificationURL) == "" || !credentials.Status.Valid() || credentials.WorkflowID == uuid.Nil || strings.TrimSpace(credentials.Verifier) == "" {
		return StartResult{}, ErrVerifierMisconfigured
	}
	if request.ExistingSessionID != nil && credentials.SessionID != *request.ExistingSessionID {
		return StartResult{}, ErrVerifierMisconfigured
	}
	if latest != nil && latest.Status.IsReusable() {
		return StartResult{Verification: latest, Credentials: credentials}, nil
	}

	now := service.clock.Now().UTC()
	verification, err := NewVerification(provider.ID(), credentials.SessionID, credentials.WorkflowID, credentials.Verifier, credentials.WorkflowVersion, now)
	if err != nil {
		return StartResult{}, err
	}
	verification.Status = credentials.Status
	if err := service.repository.Save(ctx, verification); err != nil {
		return StartResult{}, fmt.Errorf("saving identity verification: %w", err)
	}
	return StartResult{Verification: verification, Credentials: credentials}, nil
}

func (service *Service) ApplyResult(ctx context.Context, result VerificationResult) error {
	if result.EventID == uuid.Nil || result.SessionID == uuid.Nil || !result.Status.Valid() || result.OccurredOn.IsZero() {
		return ErrInvalidResult
	}
	verification, err := service.repository.FindBySessionID(ctx, result.SessionID)
	if errors.Is(err, ErrSessionNotFound) || verification == nil {
		return ErrSessionNotFound
	}
	if err != nil {
		return fmt.Errorf("finding identity verification session: %w", err)
	}
	if result.EventType == "" {
		result.EventType = "status.updated"
	}
	if err := verification.Apply(result, service.clock.Now().UTC()); err != nil {
		return err
	}
	event := VerificationEvent{
		ExternalEventID:   result.EventID,
		ExternalSessionID: result.SessionID,
		EventType:         result.EventType,
		OccurredOn:        result.OccurredOn.UTC(),
		ReceivedOn:        service.clock.Now().UTC(),
	}
	return service.unitOfWork.SaveResult(ctx, event, verification)
}

func (service *Service) Status(ctx context.Context, providerID int) (StatusSnapshot, error) {
	if providerID <= 0 {
		return StatusSnapshot{}, ErrProviderRequired
	}
	verification, err := service.repository.FindLatestByProviderID(ctx, providerID)
	if err != nil {
		return StatusSnapshot{}, err
	}
	if verification == nil {
		return StatusSnapshot{Status: StatusUnverified}, nil
	}
	return StatusSnapshot{Status: verification.Status, Verified: verification.IsApproved(), VerifiedOn: cloneTime(verification.VerifiedOn), RiskCodes: append([]string(nil), verification.RiskCodes...)}, nil
}

// providerVendorData is deterministic and opaque to the external provider.
// Keep this helper available to adapters and tests without exposing provider
// identifiers in HTTP contracts.
func ProviderVendorData(providerID int) string { return providerVendorData(providerID) }
