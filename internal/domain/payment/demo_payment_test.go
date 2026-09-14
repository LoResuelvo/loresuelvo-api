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

func TestIsDemoModeEnabledReflectsConstructor(t *testing.T) {
	plainService := payment.NewService(
		&intentRepositoryStub{},
		&transactionRepositoryStub{},
		proposalFinderStub{},
		nil,
		userFinderStub{},
		paymentAccountFinderStub{},
		lockManagerStub{},
		&unitOfWorkStub{},
		credentialDecryptorStub{},
		&checkoutGatewayStub{},
		&checkoutGatewayStub{},
		&notificatorStub{},
		func() string { return "unused" },
		clockStub{now: time.Now()},
	)
	assert.False(t, plainService.IsDemoModeEnabled(), "default constructor must leave demo mode disabled")

	demoService := payment.NewServiceWithDemoMode(
		&intentRepositoryStub{},
		&transactionRepositoryStub{},
		proposalFinderStub{},
		nil,
		userFinderStub{},
		paymentAccountFinderStub{},
		lockManagerStub{},
		&unitOfWorkStub{},
		credentialDecryptorStub{},
		&checkoutGatewayStub{},
		&checkoutGatewayStub{},
		&notificatorStub{},
		func() string { return "unused" },
		clockStub{now: time.Now()},
	)
	assert.True(t, demoService.IsDemoModeEnabled(), "WithDemoMode constructor must enable demo mode")
}

func TestSimulateApprovedDemoPaymentRejectsWhenDisabled(t *testing.T) {
	now := time.Date(2026, time.September, 14, 18, 0, 0, 0, time.UTC)
	scheduledOn := now.Add(48 * time.Hour)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, scheduledOn)
	require.NoError(t, err)
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		42,
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

	service := payment.NewService(
		&intentRepositoryStub{found: intent},
		&transactionRepositoryStub{},
		proposalFinderStub{},
		nil,
		userFinderStub{},
		paymentAccountFinderStub{},
		lockManagerStub{},
		&unitOfWorkStub{},
		credentialDecryptorStub{},
		&checkoutGatewayStub{},
		&checkoutGatewayStub{},
		&notificatorStub{},
		func() string { return "unused" },
		clockStub{now: now},
	)

	err = service.SimulateApprovedDemoPayment(context.Background(), intent, "mp-provider")
	assert.ErrorIs(t, err, payment.ErrDemoPaymentDisabled,
		"demo simulator must refuse to run when the service was constructed without the demo flag")
	assert.Equal(t, payment.StatusCheckoutReady, intent.Status,
		"intent must remain checkout_ready when demo simulation is refused")
}

func TestSimulateApprovedDemoPaymentTransitionsToPaidAndAcceptsBooking(t *testing.T) {
	now := time.Date(2026, time.September, 14, 18, 0, 0, 0, time.UTC)
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
	gateway := &checkoutGatewayStub{}
	unitOfWork := &unitOfWorkStub{}
	notificator := &notificatorStub{}
	transactionRepository := &transactionRepositoryStub{}
	service := payment.NewServiceWithDemoMode(
		intentRepository,
		transactionRepository,
		proposalFinderStub{proposal: proposal},
		nil,
		userFinderStub{},
		paymentAccountFinderStub{account: account},
		lockManagerStub{},
		unitOfWork,
		credentialDecryptorStub{},
		gateway,
		gateway,
		notificator,
		func() string { return "unused" },
		clockStub{now: now},
	)

	err = service.SimulateApprovedDemoPayment(context.Background(), intent, "mp-provider")

	require.NoError(t, err, "demo simulation must reuse the same persistence pipeline as the real webhook")
	assert.Equal(t, payment.StatusPaid, intent.Status,
		"demo simulation must drive the intent to Paid")
	assert.Equal(t, serviceproposal.StatusAccepted, proposal.Status,
		"demo simulation must accept the service proposal through the same visitor as the real webhook")
	require.NotNil(t, unitOfWork.transaction,
		"demo simulation must persist a Transaction")
	assert.Equal(t,
		"demo-payment-"+intent.ID,
		unitOfWork.transaction.ExternalPaymentID,
		"demo simulation must tag the persisted Transaction with the deterministic demo payment id")
	assert.Equal(t, "mp-provider", unitOfWork.transaction.SellerAccountID)
	assert.Same(t, intent, unitOfWork.intent)
	require.NotNil(t, unitOfWork.order,
		"demo simulation must schedule a WorkOrder via the standard outcome visitor")
	assert.Equal(t, proposal.ID, unitOfWork.order.ServiceProposalID())
	assert.Equal(t, workorder.StatusScheduled, unitOfWork.order.Status())
	require.NotNil(t, unitOfWork.notification,
		"demo simulation must create the provider-accepted notification")
	assert.Equal(t, notification.TypeServiceProposalAccepted, unitOfWork.notification.Type)
	assert.Equal(t, proposalProvider.ID(), unitOfWork.notification.UserID)
	require.NotNil(t, notificator.notification,
		"demo simulation must dispatch the notification through the standard notificator")
	assert.Equal(t, 1, notificator.calls)
}

func TestSimulateApprovedDemoPaymentIsIdempotent(t *testing.T) {
	now := time.Date(2026, time.September, 14, 18, 30, 0, 0, time.UTC)
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
	unitOfWork := &unitOfWorkStub{}
	notificator := &notificatorStub{}
	transactionRepository := &transactionRepositoryStub{}
	service := payment.NewServiceWithDemoMode(
		&intentRepositoryStub{found: intent},
		transactionRepository,
		proposalFinderStub{proposal: proposal},
		nil,
		userFinderStub{},
		paymentAccountFinderStub{account: account},
		lockManagerStub{},
		unitOfWork,
		credentialDecryptorStub{},
		&checkoutGatewayStub{},
		&checkoutGatewayStub{},
		notificator,
		func() string { return "unused" },
		clockStub{now: now},
	)

	require.NoError(t, service.SimulateApprovedDemoPayment(context.Background(), intent, "mp-provider"))

	transactionRepository.found = unitOfWork.transaction
	require.NoError(t, service.SimulateApprovedDemoPayment(context.Background(), intent, "mp-provider"),
		"re-simulating the demo payment must be a no-op and must not return an error")
	require.NoError(t, service.SimulateApprovedDemoPayment(context.Background(), intent, "mp-provider"))

	assert.Equal(t, 1, unitOfWork.calls,
		"the transactional persistence must only run once across repeated demo simulations")
	assert.Equal(t, 1, notificator.calls,
		"the notificator must only be invoked once across repeated demo simulations")
	assert.Equal(t, payment.StatusPaid, intent.Status)
}

func TestSimulateApprovedDemoPaymentRejectsIntentNotInCheckoutReady(t *testing.T) {
	now := time.Date(2026, time.September, 14, 18, 45, 0, 0, time.UTC)
	scheduledOn := now.Add(48 * time.Hour)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, scheduledOn)
	require.NoError(t, err)
	intent, err := payment.NewBookingDepositIntent(
		"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		42,
		terms,
		now.Add(-time.Minute),
	)
	require.NoError(t, err)

	service := payment.NewServiceWithDemoMode(
		&intentRepositoryStub{found: intent},
		&transactionRepositoryStub{},
		proposalFinderStub{},
		nil,
		userFinderStub{},
		paymentAccountFinderStub{},
		lockManagerStub{},
		&unitOfWorkStub{},
		credentialDecryptorStub{},
		&checkoutGatewayStub{},
		&checkoutGatewayStub{},
		&notificatorStub{},
		func() string { return "unused" },
		clockStub{now: now},
	)

	err = service.SimulateApprovedDemoPayment(context.Background(), intent, "mp-provider")
	assert.ErrorIs(t, err, payment.ErrDemoPaymentIntentNotReady,
		"the demo simulator must only run against intents in the checkout_ready state")
}

func TestSimulateApprovedDemoPaymentProducesSameOutcomeAsRealWebhook(t *testing.T) {
	now := time.Date(2026, time.September, 14, 19, 0, 0, 0, time.UTC)
	scheduledOn := now.Add(48 * time.Hour)
	terms, err := serviceproposal.NewBookingPolicy().Calculate(10000000, scheduledOn)
	require.NoError(t, err)
	proposalConsumer := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(
		10, "auth0|consumer", "ana@example.com", "Ana", "Pérez", consumer.Role, nil,
	)}
	proposalProvider := &provider.Provider{BaseUser: user.RehydrateBaseUser(
		20, "auth0|provider", "juan@example.com", "Juan", "Gómez", provider.Role, nil,
	)}

	buildProposal := func() *serviceproposal.ServiceProposal {
		return &serviceproposal.ServiceProposal{
			ID:           42,
			Consumer:     proposalConsumer,
			Provider:     proposalProvider,
			Status:       serviceproposal.StatusPending,
			ScheduledOn:  scheduledOn,
			BookingTerms: terms,
		}
	}
	buildIntent := func() *payment.Intent {
		intent, err := payment.NewBookingDepositIntent(
			"f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
			42,
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
		return intent
	}
	buildAccount := func() *paymentaccount.PaymentAccount {
		account, err := paymentaccount.NewPaymentAccount(
			proposalProvider.ID(),
			paymentaccount.PaymentProvider("mercado_pago"),
			"mp-provider",
			[]byte("encrypted-access-token"),
			nil,
			now.Add(24*time.Hour),
		)
		require.NoError(t, err)
		return account
	}

	realProposal := buildProposal()
	realIntent := buildIntent()
	realAccount := buildAccount()
	realGateway := &checkoutGatewayStub{payment: payment.ExternalPayment{
		ID:                "real-mp-payment-id-42",
		SellerAccountID:   "mp-provider",
		ExternalReference: realIntent.ID,
		Status:            payment.ExternalPaymentStatusApproved,
		Currency:          "ARS",
		AmountCents:       realIntent.TotalAmountCents,
	}}
	realUnitOfWork := &unitOfWorkStub{}
	realNotificator := &notificatorStub{}
	realService := payment.NewService(
		&intentRepositoryStub{found: realIntent},
		&transactionRepositoryStub{},
		proposalFinderStub{proposal: realProposal},
		nil,
		userFinderStub{},
		paymentAccountFinderStub{account: realAccount},
		lockManagerStub{},
		realUnitOfWork,
		credentialDecryptorStub{},
		realGateway,
		realGateway,
		realNotificator,
		func() string { return "unused" },
		clockStub{now: now},
	)
	require.NoError(t, realService.ProcessPaymentNotification(context.Background(), payment.PaymentNotification{
		ExternalPaymentID: "real-mp-payment-id-42",
		SellerAccountID:   "mp-provider",
	}))

	demoProposal := buildProposal()
	demoIntent := buildIntent()
	demoAccount := buildAccount()
	demoUnitOfWork := &unitOfWorkStub{}
	demoNotificator := &notificatorStub{}
	demoService := payment.NewServiceWithDemoMode(
		&intentRepositoryStub{found: demoIntent},
		&transactionRepositoryStub{},
		proposalFinderStub{proposal: demoProposal},
		nil,
		userFinderStub{},
		paymentAccountFinderStub{account: demoAccount},
		lockManagerStub{},
		demoUnitOfWork,
		credentialDecryptorStub{},
		&checkoutGatewayStub{},
		&checkoutGatewayStub{},
		demoNotificator,
		func() string { return "unused" },
		clockStub{now: now},
	)
	require.NoError(t, demoService.SimulateApprovedDemoPayment(context.Background(), demoIntent, "mp-provider"))

	assert.Equal(t, realIntent.Status, demoIntent.Status,
		"demo simulation must drive the intent to the same final status as the real webhook")
	assert.Equal(t, realProposal.Status, demoProposal.Status,
		"demo simulation must accept the proposal through the same outcome as the real webhook")
	require.NotNil(t, realUnitOfWork.transaction)
	require.NotNil(t, demoUnitOfWork.transaction)
	assert.Equal(t,
		realUnitOfWork.transaction.Currency,
		demoUnitOfWork.transaction.Currency,
		"demo transaction must record the same currency as the real one")
	assert.Equal(t,
		realUnitOfWork.transaction.AmountCents,
		demoUnitOfWork.transaction.AmountCents,
		"demo transaction must record the same amount as the real one")
	assert.Equal(t,
		realUnitOfWork.transaction.SellerAccountID,
		demoUnitOfWork.transaction.SellerAccountID,
		"demo transaction must record the same seller account as the real one")
	assert.Equal(t,
		realUnitOfWork.transaction.PaymentIntentID,
		demoUnitOfWork.transaction.PaymentIntentID,
		"demo transaction must reference the same payment intent as the real one")
	require.NotNil(t, realUnitOfWork.order)
	require.NotNil(t, demoUnitOfWork.order)
	assert.Equal(t,
		realUnitOfWork.order.Status(),
		demoUnitOfWork.order.Status(),
		"demo work order must reach the same status as the real one")
	require.NotNil(t, realUnitOfWork.notification)
	require.NotNil(t, demoUnitOfWork.notification)
	assert.Equal(t,
		realUnitOfWork.notification.Type,
		demoUnitOfWork.notification.Type,
		"demo notification must use the same type as the real one")
}
