package identity_verification_handler

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/stretchr/testify/mock"
)

type identityVerificationServiceMock struct{ mock.Mock }

func (m *identityVerificationServiceMock) Start(ctx context.Context, authID string) (identityverification.StartResult, error) {
	args := m.Called(ctx, authID)
	return args.Get(0).(identityverification.StartResult), args.Error(1)
}

func (m *identityVerificationServiceMock) ApplyResult(ctx context.Context, result identityverification.VerificationResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

type identityVerificationWebhookMock struct{ mock.Mock }

func (m *identityVerificationWebhookMock) Authenticate(body []byte, signature, timestamp string, now time.Time) error {
	args := m.Called(body, signature, timestamp, now)
	return args.Error(0)
}

func (m *identityVerificationWebhookMock) Translate(body []byte) (identityverification.VerificationResult, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return identityverification.VerificationResult{}, args.Error(1)
	}
	return args.Get(0).(identityverification.VerificationResult), args.Error(1)
}

type identityVerificationClockStub struct{ now time.Time }

func (stub identityVerificationClockStub) Now() time.Time { return stub.now }
