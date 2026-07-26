package payment_test

import (
	"context"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type intentRepositoryStub struct {
	saved              *payment.Intent
	checkoutReadySaved *payment.Intent
	processingSaved    *payment.Intent
	rejectedSaved      *payment.Intent
	expiredSaved       *payment.Intent
	found              *payment.Intent
}

func (repository *intentRepositoryStub) WithinBookingCheckoutLock(
	_ context.Context,
	_ int,
	operation func() error,
) error {
	return operation()
}

func (repository *intentRepositoryStub) FindByID(context.Context, string) (*payment.Intent, error) {
	if repository.found == nil {
		return nil, payment.ErrIntentDoesNotExist
	}
	return repository.found, nil
}

func (repository *intentRepositoryStub) FindActiveBookingCheckout(
	context.Context,
	int,
) (*payment.Intent, error) {
	if repository.found == nil ||
		repository.found.CheckoutSession == nil ||
		(repository.found.Status != payment.StatusCheckoutReady &&
			repository.found.Status != payment.StatusProcessing) {
		return nil, payment.ErrIntentDoesNotExist
	}
	return repository.found, nil
}

func (repository *intentRepositoryStub) Save(_ context.Context, intent *payment.Intent) error {
	repository.saved = intent
	return nil
}

func (repository *intentRepositoryStub) SaveCheckoutReady(_ context.Context, intent *payment.Intent) error {
	repository.checkoutReadySaved = intent
	return nil
}

func (repository *intentRepositoryStub) SaveProcessing(_ context.Context, intent *payment.Intent) error {
	repository.processingSaved = intent
	return nil
}

func (repository *intentRepositoryStub) SaveRejected(_ context.Context, intent *payment.Intent) error {
	repository.rejectedSaved = intent
	return nil
}

func (repository *intentRepositoryStub) SaveExpired(_ context.Context, intent *payment.Intent) error {
	repository.expiredSaved = intent
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

func (finder paymentAccountFinderStub) FindByExternalAccountID(
	context.Context,
	string,
	paymentaccount.PaymentProvider,
) (*paymentaccount.PaymentAccount, error) {
	return finder.account, nil
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
	payment     payment.ExternalPayment
	createCalls int
}

func (gateway *checkoutGatewayStub) Provider() paymentaccount.PaymentProvider {
	return paymentaccount.PaymentProvider("mercado_pago")
}

func (gateway *checkoutGatewayStub) CreateCheckout(
	_ context.Context,
	accessToken string,
	request payment.CheckoutRequest,
) (payment.ExternalCheckout, error) {
	gateway.createCalls++
	gateway.accessToken = accessToken
	gateway.request = request
	return payment.ExternalCheckout{
		ID:  "mp-preference-42",
		URL: "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
	}, nil
}

func (gateway *checkoutGatewayStub) GetPayment(
	context.Context,
	string,
	string,
) (payment.ExternalPayment, error) {
	return gateway.payment, nil
}

type paidBookingConfirmerStub struct {
	intent       *payment.Intent
	order        *workorder.WorkOrder
	notification *notification.Notification
}

func (confirmer *paidBookingConfirmerStub) ConfirmPaidBooking(
	_ context.Context,
	intent *payment.Intent,
	order *workorder.WorkOrder,
	acceptedNotification *notification.Notification,
) (*payment.PaidBookingConfirmation, error) {
	confirmer.intent = intent
	confirmer.order = order
	confirmer.notification = acceptedNotification
	savedNotification := *acceptedNotification
	savedNotification.ID = 99
	return &payment.PaidBookingConfirmation{
		WorkOrder:    order,
		Notification: &savedNotification,
	}, nil
}

type notificatorStub struct {
	notification *notification.Notification
}

func (notificator *notificatorStub) Notify(
	_ context.Context,
	savedNotification *notification.Notification,
) error {
	notificator.notification = savedNotification
	return nil
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
		checkoutGateway,
		nil,
		nil,
		func() string { return "f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36" },
		clockStub{now: now},
	)

	result, err := service.StartBookingCheckout(context.Background(), "auth0|consumer", proposal.ID)

	require.NoError(t, err)
	assert.True(t, result.Created)
	intent := result.Intent
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

	intentRepository.found = intent
	reusedResult, err := service.StartBookingCheckout(
		context.Background(),
		"auth0|consumer",
		proposal.ID,
	)
	require.NoError(t, err)
	assert.False(t, reusedResult.Created)
	assert.Equal(t, intent.ID, reusedResult.Intent.ID)
	assert.Equal(t, intent.CheckoutSession.URL, reusedResult.Intent.CheckoutSession.URL)
	assert.Equal(t, 1, checkoutGateway.createCalls)
}

func TestStartBookingCheckoutEnforcesBookingPaymentDeadline(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, scheduledOn)
	require.NoError(t, err)
	deadline := terms.BookingPaymentDeadline()

	tests := []struct {
		name      string
		now       time.Time
		wantError bool
	}{
		{
			name: "before deadline",
			now:  deadline.Add(-time.Nanosecond),
		},
		{
			name:      "exactly at deadline",
			now:       deadline,
			wantError: true,
		},
		{
			name:      "after deadline",
			now:       deadline.Add(time.Nanosecond),
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
				deadline.Add(24*time.Hour),
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
				checkoutGateway,
				nil,
				nil,
				func() string { return "f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36" },
				clockStub{now: test.now},
			)

			result, err := service.StartBookingCheckout(
				context.Background(),
				"auth0|consumer",
				proposal.ID,
			)

			if test.wantError {
				assert.ErrorIs(t, err, payment.ErrBookingPaymentDeadlineReached)
				assert.Nil(t, result)
				assert.Nil(t, intentRepository.saved)
				assert.Empty(t, checkoutGateway.accessToken)
				return
			}
			require.NoError(t, err)
			intent := result.Intent
			require.NotNil(t, intent)
			require.NotNil(t, intent.CheckoutSession)
			assert.Equal(t, deadline, intent.CheckoutSession.ExpiresOn)
		})
	}
}

func TestProcessApprovedPaymentConfirmsPaidBooking(t *testing.T) {
	now := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	scheduledOn := now.Add(48 * time.Hour)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, scheduledOn)
	require.NoError(t, err)
	proposalConsumer := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(
		10, "auth0|consumer", "ana@example.com", "Ana", "Pérez", consumer.Role, nil,
	)}
	proposalProvider := &provider.Provider{BaseUser: user.RehydrateBaseUser(
		20, "auth0|provider", "juan@example.com", "Juan", "Gómez", provider.Role, nil,
	)}
	proposal := &serviceproposal.ServiceProposal{
		ID:           42,
		Consumer:     proposalConsumer,
		Provider:     proposalProvider,
		Status:       serviceproposal.StatusPending,
		ScheduledOn:  scheduledOn,
		Description:  "Reparación de pérdida de agua.",
		BookingTerms: terms,
	}
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		proposal.ID,
		terms,
		now.Add(-time.Minute),
	)
	require.NoError(t, err)
	require.NoError(t, intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		now.Add(29*time.Minute),
		now.Add(-30*time.Second),
	))
	account, err := paymentaccount.NewPaymentAccount(
		proposalProvider.ID(),
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-provider",
		[]byte("encrypted-access-token"),
		nil,
		now.Add(24*time.Hour),
	)
	require.NoError(t, err)
	intentRepository := &intentRepositoryStub{found: intent}
	gateway := &checkoutGatewayStub{payment: payment.ExternalPayment{
		ID:                "123456",
		SellerAccountID:   "mp-provider",
		ExternalReference: intent.ID,
		Status:            payment.ExternalPaymentStatusApproved,
		Currency:          "ARS",
		AmountCents:       intent.TotalAmountCents,
	}}
	confirmer := &paidBookingConfirmerStub{}
	notificator := &notificatorStub{}
	service := payment.NewService(
		intentRepository,
		proposalFinderStub{proposal: proposal},
		userFinderStub{},
		paymentAccountFinderStub{account: account},
		credentialDecryptorStub{},
		gateway,
		gateway,
		confirmer,
		notificator,
		func() string { return "unused" },
		clockStub{now: now},
	)

	err = service.ProcessPaymentNotification(context.Background(), payment.PaymentNotification{
		ExternalPaymentID: "123456",
		SellerAccountID:   "mp-provider",
	})

	require.NoError(t, err)
	assert.Equal(t, payment.StatusPaid, intent.Status)
	assert.Equal(t, serviceproposal.StatusAccepted, proposal.Status)
	assert.Same(t, intent, confirmer.intent)
	require.NotNil(t, confirmer.order)
	assert.Equal(t, proposal.ID, confirmer.order.ServiceProposalID())
	assert.Equal(t, workorder.StatusScheduled, confirmer.order.Status)
	assert.Equal(t, now, confirmer.order.AcceptedOn)
	require.NotNil(t, confirmer.notification)
	assert.Equal(t, proposalProvider.ID(), confirmer.notification.UserID)
	assert.Equal(t, notification.TypeServiceProposalAccepted, confirmer.notification.Type)
	assert.Equal(t, notification.ResourceServiceProposal, confirmer.notification.ResourceType)
	assert.Equal(t, proposal.ID, confirmer.notification.ResourceID)
	require.NotNil(t, notificator.notification)
	assert.Equal(t, 99, notificator.notification.ID)
	assert.Equal(t, confirmer.notification.Type, notificator.notification.Type)
}

func TestProcessProcessingPaymentOnlyUpdatesIntent(t *testing.T) {
	now := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	scheduledOn := now.Add(48 * time.Hour)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, scheduledOn)
	require.NoError(t, err)
	proposalConsumer := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(
		10, "auth0|consumer", "ana@example.com", "Ana", "Pérez", consumer.Role, nil,
	)}
	proposalProvider := &provider.Provider{BaseUser: user.RehydrateBaseUser(
		20, "auth0|provider", "juan@example.com", "Juan", "Gómez", provider.Role, nil,
	)}
	proposal := &serviceproposal.ServiceProposal{
		ID:           42,
		Consumer:     proposalConsumer,
		Provider:     proposalProvider,
		Status:       serviceproposal.StatusPending,
		ScheduledOn:  scheduledOn,
		BookingTerms: terms,
	}
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		proposal.ID,
		terms,
		now.Add(-time.Minute),
	)
	require.NoError(t, err)
	require.NoError(t, intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		now.Add(29*time.Minute),
		now.Add(-30*time.Second),
	))
	account, err := paymentaccount.NewPaymentAccount(
		proposalProvider.ID(),
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-provider",
		[]byte("encrypted-access-token"),
		nil,
		now.Add(24*time.Hour),
	)
	require.NoError(t, err)
	intentRepository := &intentRepositoryStub{found: intent}
	gateway := &checkoutGatewayStub{payment: payment.ExternalPayment{
		ID:                "123456",
		SellerAccountID:   "mp-provider",
		ExternalReference: intent.ID,
		Status:            payment.ExternalPaymentStatusProcessing,
		Currency:          "ARS",
		AmountCents:       intent.TotalAmountCents,
	}}
	confirmer := &paidBookingConfirmerStub{}
	notificator := &notificatorStub{}
	service := payment.NewService(
		intentRepository,
		proposalFinderStub{proposal: proposal},
		userFinderStub{},
		paymentAccountFinderStub{account: account},
		credentialDecryptorStub{},
		gateway,
		gateway,
		confirmer,
		notificator,
		func() string { return "unused" },
		clockStub{now: now},
	)

	err = service.ProcessPaymentNotification(context.Background(), payment.PaymentNotification{
		ExternalPaymentID: "123456",
		SellerAccountID:   "mp-provider",
	})

	require.NoError(t, err)
	require.NoError(t, service.ProcessPaymentNotification(context.Background(), payment.PaymentNotification{
		ExternalPaymentID: "123456",
		SellerAccountID:   "mp-provider",
	}))
	assert.Equal(t, payment.StatusProcessing, intent.Status)
	assert.Same(t, intent, intentRepository.processingSaved)
	assert.Equal(t, serviceproposal.StatusPending, proposal.Status)
	assert.Nil(t, confirmer.order)
	assert.Nil(t, confirmer.notification)
	assert.Nil(t, notificator.notification)
}

func TestProcessRejectedPaymentOnlyUpdatesIntent(t *testing.T) {
	now := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	scheduledOn := now.Add(48 * time.Hour)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, scheduledOn)
	require.NoError(t, err)
	proposalConsumer := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(
		10, "auth0|consumer", "ana@example.com", "Ana", "Pérez", consumer.Role, nil,
	)}
	proposalProvider := &provider.Provider{BaseUser: user.RehydrateBaseUser(
		20, "auth0|provider", "juan@example.com", "Juan", "Gómez", provider.Role, nil,
	)}
	proposal := &serviceproposal.ServiceProposal{
		ID:           42,
		Consumer:     proposalConsumer,
		Provider:     proposalProvider,
		Status:       serviceproposal.StatusPending,
		ScheduledOn:  scheduledOn,
		BookingTerms: terms,
	}
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		proposal.ID,
		terms,
		now.Add(-time.Minute),
	)
	require.NoError(t, err)
	require.NoError(t, intent.MarkCheckoutReady(
		"mp-preference-42",
		"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=mp-preference-42",
		now.Add(29*time.Minute),
		now.Add(-30*time.Second),
	))
	account, err := paymentaccount.NewPaymentAccount(
		proposalProvider.ID(),
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-provider",
		[]byte("encrypted-access-token"),
		nil,
		now.Add(24*time.Hour),
	)
	require.NoError(t, err)
	intentRepository := &intentRepositoryStub{found: intent}
	gateway := &checkoutGatewayStub{payment: payment.ExternalPayment{
		ID:                "123456",
		SellerAccountID:   "mp-provider",
		ExternalReference: intent.ID,
		Status:            payment.ExternalPaymentStatusRejected,
		Currency:          "ARS",
		AmountCents:       intent.TotalAmountCents,
	}}
	confirmer := &paidBookingConfirmerStub{}
	notificator := &notificatorStub{}
	service := payment.NewService(
		intentRepository,
		proposalFinderStub{proposal: proposal},
		userFinderStub{},
		paymentAccountFinderStub{account: account},
		credentialDecryptorStub{},
		gateway,
		gateway,
		confirmer,
		notificator,
		func() string { return "unused" },
		clockStub{now: now},
	)

	err = service.ProcessPaymentNotification(context.Background(), payment.PaymentNotification{
		ExternalPaymentID: "123456",
		SellerAccountID:   "mp-provider",
	})

	require.NoError(t, err)
	require.NoError(t, service.ProcessPaymentNotification(context.Background(), payment.PaymentNotification{
		ExternalPaymentID: "123456",
		SellerAccountID:   "mp-provider",
	}))
	assert.Equal(t, payment.StatusRejected, intent.Status)
	assert.Same(t, intent, intentRepository.rejectedSaved)
	assert.Equal(t, serviceproposal.StatusPending, proposal.Status)
	assert.Nil(t, confirmer.order)
	assert.Nil(t, confirmer.notification)
	assert.Nil(t, notificator.notification)
}
