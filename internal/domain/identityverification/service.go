package identityverification

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
)

type StartResult struct {
	Verification *IdentityVerification
	Credentials  SessionCredentials
}

type VerificationStatusDetails struct {
	Status     VerificationStatus
	VerifiedOn *time.Time
}

type Service struct {
	providerFinder ProviderFinder
	repository     VerificationRepository
	unitOfWork     UnitOfWork
	verifier       IdentityVerifier
	clock          clock.Clock
}

func NewService(providerFinder ProviderFinder, repository VerificationRepository, unitOfWork UnitOfWork, verifier IdentityVerifier, systemClock clock.Clock) *Service {
	return &Service{providerFinder: providerFinder, repository: repository, unitOfWork: unitOfWork, verifier: verifier, clock: systemClock}
}

func (service *Service) ApplyResult(ctx context.Context, result VerificationResult) error {
	if result.EventID == uuid.Nil || result.SessionID == uuid.Nil || result.ProviderID <= 0 || result.WorkflowID == uuid.Nil || result.WorkflowVersion < 0 || !result.Status.CanApplyResult() || result.OccurredOn.IsZero() {
		return ErrInvalidVerification
	}
	now := service.clock.Now()
	event, err := NewVerificationEvent(result, now)
	if err != nil {
		return err
	}
	if err := service.unitOfWork.Execute(ctx, func(store TransactionalStore) error {
		stored, err := store.SaveEvent(ctx, event)
		if err != nil {
			return fmt.Errorf("saving identity verification event: %w", err)
		}
		if !stored {
			return nil
		}
		verification, err := store.FindVerificationBySessionID(ctx, result.SessionID)
		if err != nil {
			return fmt.Errorf("finding identity verification session: %w", err)
		}
		if verification == nil {
			return ErrSessionNotFound
		}
		if err := verification.ApplyResult(result, now); err != nil {
			return err
		}
		if err := store.SaveVerification(ctx, verification); err != nil {
			return fmt.Errorf("saving identity verification result: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("applying identity verification result: %w", err)
	}
	return nil
}

func (service *Service) CurrentStatus(ctx context.Context, providerID int) (VerificationStatus, error) {
	details, err := service.CurrentStatusDetails(ctx, providerID)
	if err != nil {
		return "", err
	}
	return details.Status, nil
}

func (service *Service) CurrentStatusDetails(ctx context.Context, providerID int) (VerificationStatusDetails, error) {
	latest, err := service.repository.FindLatestByProviderID(ctx, providerID)
	if err != nil {
		return VerificationStatusDetails{}, fmt.Errorf("finding current identity verification: %w", err)
	}
	if latest == nil {
		return VerificationStatusDetails{Status: StatusUnverified}, nil
	}
	return VerificationStatusDetails{Status: latest.Status, VerifiedOn: latest.VerifiedOn}, nil
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
	if latest != nil && latest.Status == StatusApproved {
		return StartResult{}, ErrVerificationAlreadyApproved
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
