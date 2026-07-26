package payment

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type IntentRepository interface {
	Save(ctx context.Context, intent *Intent) error
	FindByID(ctx context.Context, id string) (*Intent, error)
	FindLatestByProposalIDAndPurpose(
		ctx context.Context,
		serviceProposalID int,
		purpose Purpose,
	) (*Intent, error)
}

type ServiceProposalFinder interface {
	FindByID(ctx context.Context, id int) (*serviceproposal.ServiceProposal, error)
}

type UserFinder interface {
	FindByAuthID(authID string) (user.User, error)
}

type PaymentAccountFinder interface {
	FindByProviderID(ctx context.Context, providerID int, paymentProvider paymentaccount.PaymentProvider) (*paymentaccount.PaymentAccount, error)
	FindByExternalAccountID(ctx context.Context, externalAccountID string, paymentProvider paymentaccount.PaymentProvider) (*paymentaccount.PaymentAccount, error)
}

type TransactionRepository interface {
	Save(ctx context.Context, transaction *Transaction) error
	FindByExternalID(
		ctx context.Context,
		processor paymentaccount.PaymentProvider,
		externalPaymentID string,
	) (*Transaction, error)
}

type LockKey struct {
	Namespace int
	Resource  string
}

type LockManager interface {
	WithinLock(ctx context.Context, key LockKey, operation func() error) error
}

type TransactionalStore interface {
	SaveIntent(ctx context.Context, intent *Intent) error
	SaveTransaction(ctx context.Context, transaction *Transaction) error
	SaveServiceProposal(ctx context.Context, proposal *serviceproposal.ServiceProposal) error
	SaveWorkOrder(ctx context.Context, order *workorder.WorkOrder) error
	SaveNotification(ctx context.Context, notification *notification.Notification) error
}

type UnitOfWork interface {
	Execute(ctx context.Context, operation func(TransactionalStore) error) error
}

type CredentialDecryptor interface {
	Decrypt(ciphertext []byte) (string, error)
}

type CheckoutGateway interface {
	Provider() paymentaccount.PaymentProvider
	CreateCheckout(ctx context.Context, accessToken string, request CheckoutRequest) (ExternalCheckout, error)
}

type PaymentVerifier interface {
	GetPayment(ctx context.Context, accessToken, externalPaymentID string) (ExternalPayment, error)
}

type Gateway interface {
	CheckoutGateway
	PaymentVerifier
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

type ExternalPaymentStatus string

const (
	ExternalPaymentStatusApproved   ExternalPaymentStatus = "approved"
	ExternalPaymentStatusProcessing ExternalPaymentStatus = "processing"
	ExternalPaymentStatusRejected   ExternalPaymentStatus = "rejected"
)

type ExternalPayment struct {
	ID                string
	SellerAccountID   string
	ExternalReference string
	Status            ExternalPaymentStatus
	Currency          string
	AmountCents       int64
}

type PaymentNotification struct {
	ExternalPaymentID string
	SellerAccountID   string
}

type IDGenerator func() string
