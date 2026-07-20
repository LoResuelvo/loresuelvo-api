package paymentaccount

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type UserRepository interface {
	FindByAuthID(authID string) (user.User, error)
}

type AuthorizationAttemptRepository interface {
	Save(ctx context.Context, attempt *AuthorizationAttempt) error
	FindByStateDigest(ctx context.Context, stateDigest []byte) (*AuthorizationAttempt, error)
	Consume(ctx context.Context, attempt *AuthorizationAttempt) error
}

type PaymentAccountRepository interface {
	SaveFromAuthorization(ctx context.Context, attemptID int, account *PaymentAccount) error
	FindByProviderID(ctx context.Context, providerID int, paymentProvider PaymentProvider) (*PaymentAccount, error)
}

type Clock interface {
	Now() time.Time
}
