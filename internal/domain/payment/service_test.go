package payment_test

import (
	"context"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type intentRepositoryStub struct {
	saved              *payment.Intent
	checkoutReadySaved *payment.Intent
}

func (repository *intentRepositoryStub) Save(_ context.Context, intent *payment.Intent) error {
	repository.saved = intent
	return nil
}

func (repository *intentRepositoryStub) SaveCheckoutReady(_ context.Context, intent *payment.Intent) error {
	repository.checkoutReadySaved = intent
	return nil
}

type proposalFinderStub struct {
	proposal *serviceproposal.ServiceProposal
}

func (finder proposalFinderStub) FindByID(_ context.Context, _ int) (*serviceproposal.ServiceProposal, error) {
	return finder.proposal, nil
}

type userFinderStub struct {
	found user.User
}

func (finder userFinderStub) FindByAuthID(string) (user.User, error) {
	return finder.found, nil
}

type paymentAccountFinderStub struct {
	account *paymentaccount.PaymentAccount
}

func (finder paymentAccountFinderStub) FindByProviderID(
	context.Context,
	int,
	paymentaccount.PaymentProvider,
) (*paymentaccount.PaymentAccount, error) {
	return finder.account, nil
}

type credentialDecryptorStub struct{}

func (credentialDecryptorStub) Decrypt([]byte) (string, error) {
	return "seller-access-token", nil
}

type checkoutGatewayStub struct {
	accessToken string
	request     payment.CheckoutRequest
}

func (gateway *checkoutGatewayStub) Provider() paymentaccount.PaymentProvider {
	return paymentaccount.PaymentProvider("mercado_pago")
}

func (gateway *checkoutGatewayStub) CreateCheckout(
	_ context.Context,
	accessToken string,
	request payment.CheckoutRequest,
) (payment.ExternalCheckout, error) {
	gateway.accessToken = accessToken
	gateway.request = request
	return payment.ExternalCheckout{
		ID:  "mp-preference-42",
		URL: "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
	}, nil
}

type clockStub struct {
	now time.Time
}

func (clock clockStub) Now() time.Time {
	return clock.now
}

func TestStartBookingCheckoutCreatesReadyIntentWithFrozenProposalPricing(t *testing.T) {
	now := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	scheduledOn := now.Add(48 * time.Hour)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, scheduledOn)
	require.NoError(t, err)
	proposalConsumer := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(
		10,
		"auth0|consumer",
		"ana@example.com",
		"Ana",
		"Pérez",
		consumer.Role,
		nil,
	)}
	proposalProvider := &provider.Provider{BaseUser: user.RehydrateBaseUser(
		20,
		"auth0|provider",
		"juan@example.com",
		"Juan",
		"Gómez",
		provider.Role,
		nil,
	)}
	proposal := &serviceproposal.ServiceProposal{
		ID:           42,
		Consumer:     proposalConsumer,
		Provider:     proposalProvider,
		Status:       serviceproposal.StatusPending,
		BookingTerms: terms,
	}
	account, err := paymentaccount.NewPaymentAccount(
		proposalProvider.ID(),
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-provider",
		[]byte("encrypted-access-token"),
		nil,
		now.Add(24*time.Hour),
	)
	require.NoError(t, err)
	intentRepository := &intentRepositoryStub{}
	checkoutGateway := &checkoutGatewayStub{}
	service := payment.NewService(
		intentRepository,
		proposalFinderStub{proposal: proposal},
		userFinderStub{found: proposalConsumer},
		paymentAccountFinderStub{account: account},
		credentialDecryptorStub{},
		checkoutGateway,
		func() string { return "f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36" },
		clockStub{now: now},
	)

	intent, err := service.StartBookingCheckout(context.Background(), "auth0|consumer", proposal.ID)

	require.NoError(t, err)
	require.NotNil(t, intent)
	assert.Equal(t, payment.StatusCheckoutReady, intent.Status)
	assert.Equal(t, terms, intent.BookingTerms)
	assert.Same(t, intent, intentRepository.saved)
	assert.Same(t, intent, intentRepository.checkoutReadySaved)
	assert.Equal(t, "seller-access-token", checkoutGateway.accessToken)
	assert.Equal(t, terms.DepositCents(), checkoutGateway.request.SellerAmountCents)
	assert.Equal(t, terms.PlatformFeeDueNowCents(), checkoutGateway.request.PlatformFeeCents)
	assert.Equal(t, terms.AmountDueNowCents(), checkoutGateway.request.TotalAmountCents)
	assert.Equal(t, proposalConsumer.Email(), checkoutGateway.request.PayerEmail)
	assert.Equal(t, now, checkoutGateway.request.StartsOn)
	assert.Equal(t, now.Add(30*time.Minute), checkoutGateway.request.ExpiresOn)
	require.NotNil(t, intent.CheckoutSession)
	assert.Equal(t, checkoutGateway.request.ExpiresOn, intent.CheckoutSession.ExpiresOn)
	assert.Equal(t, serviceproposal.StatusPending, proposal.Status)
}
