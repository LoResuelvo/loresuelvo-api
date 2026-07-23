package payment

import (
	"context"
	"time"

	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type IntentRepository interface {
	Save(ctx context.Context, intent *Intent) error
	SaveCheckoutReady(ctx context.Context, intent *Intent) error
}

type ServiceProposalFinder interface {
	FindByID(ctx context.Context, id int) (*serviceproposal.ServiceProposal, error)
}

type UserFinder interface {
	FindByAuthID(authID string) (user.User, error)
}

type PaymentAccountFinder interface {
	FindByProviderID(ctx context.Context, providerID int, paymentProvider paymentaccount.PaymentProvider) (*paymentaccount.PaymentAccount, error)
}

type CredentialDecryptor interface {
	Decrypt(ciphertext []byte) (string, error)
}

type CheckoutGateway interface {
	Provider() paymentaccount.PaymentProvider
	CreateCheckout(ctx context.Context, accessToken string, request CheckoutRequest) (ExternalCheckout, error)
}

type CheckoutRequest struct {
	ExternalReference string
	Currency          string
	SellerAmountCents int64
	PlatformFeeCents  int64
	TotalAmountCents  int64
	PayerEmail        string
	StartsOn          time.Time
	ExpiresOn         time.Time
}

type ExternalCheckout struct {
	ID  string
	URL string
}

type IDGenerator func() string
