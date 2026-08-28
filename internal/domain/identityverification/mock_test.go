package identityverification

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/google/uuid"
)

type providerFinderStub struct {
	provider *provider.Provider
	err      error
}

func (stub providerFinderStub) FindProviderByAuthID(context.Context, string) (*provider.Provider, error) {
	return stub.provider, stub.err
}

type verificationRepositoryStub struct {
	latest *IdentityVerification
	byID   *IdentityVerification
	saved  *IdentityVerification
	err    error
}

func (stub *verificationRepositoryStub) Save(_ context.Context, verification *IdentityVerification) error {
	if stub.err != nil {
		return stub.err
	}
	stub.saved = verification
	return nil
}

func (stub *verificationRepositoryStub) FindBySessionID(context.Context, uuid.UUID) (*IdentityVerification, error) {
	return stub.byID, stub.err
}

func (stub *verificationRepositoryStub) FindLatestByProviderID(context.Context, int) (*IdentityVerification, error) {
	return stub.latest, stub.err
}

type verifierStub struct {
	credentials SessionCredentials
	request     SessionRequest
	err         error
}

func (stub *verifierStub) CreateSession(_ context.Context, request SessionRequest) (SessionCredentials, error) {
	stub.request = request
	return stub.credentials, stub.err
}

type unitOfWorkStub struct {
	event        VerificationEvent
	verification *IdentityVerification
	err          error
}

func (stub *unitOfWorkStub) SaveResult(_ context.Context, event VerificationEvent, verification *IdentityVerification) error {
	if stub.err != nil {
		return stub.err
	}
	stub.event, stub.verification = event, verification
	return nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }
