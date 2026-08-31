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

func (stub providerFinderStub) FindProviderByAuthID(string) (*provider.Provider, error) {
	return stub.provider, stub.err
}

type verificationRepositoryStub struct {
	saved  *IdentityVerification
	byID   *IdentityVerification
	latest *IdentityVerification
	err    error
}

type transactionalStoreStub struct {
	byID                *IdentityVerification
	saved               *IdentityVerification
	savedEvent          *VerificationEvent
	duplicateEvent      bool
	saveEventErr        error
	findErr             error
	saveVerificationErr error
}

func (stub *transactionalStoreStub) SaveVerification(_ context.Context, verification *IdentityVerification) error {
	if stub.saveVerificationErr != nil {
		return stub.saveVerificationErr
	}
	stub.saved = verification
	return nil
}

func (stub *transactionalStoreStub) FindVerificationBySessionID(context.Context, uuid.UUID) (*IdentityVerification, error) {
	return stub.byID, stub.findErr
}

func (stub *transactionalStoreStub) SaveEvent(_ context.Context, event *VerificationEvent) (bool, error) {
	if stub.saveEventErr != nil {
		return false, stub.saveEventErr
	}
	stub.savedEvent = event
	return !stub.duplicateEvent, nil
}

type unitOfWorkStub struct {
	store TransactionalStore
	err   error
}

func (stub unitOfWorkStub) Execute(ctx context.Context, operation func(TransactionalStore) error) error {
	if stub.err != nil {
		return stub.err
	}
	return operation(stub.store)
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

func (stub *verificationRepositoryStub) FindByProviderID(context.Context, int) ([]IdentityVerification, error) {
	if stub.latest == nil {
		return nil, stub.err
	}
	return []IdentityVerification{*stub.latest}, stub.err
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

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }
