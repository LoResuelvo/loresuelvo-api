package mercadopago

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sharedmercadopago "github.com/LoResuelvo/loresuelvo-api/internal/adapters/mercadopago"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	mercadopagopayment "github.com/mercadopago/sdk-go/pkg/payment"
	"github.com/mercadopago/sdk-go/pkg/preference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type preferenceCreatorStub struct {
	request  preference.Request
	response *preference.Response
	err      error
}

type paymentGetterStub struct {
	id       int
	response *mercadopagopayment.Response
	err      error
}

func (getter *paymentGetterStub) Get(_ context.Context, id int) (*mercadopagopayment.Response, error) {
	getter.id = id
	return getter.response, getter.err
}

func (creator *preferenceCreatorStub) Create(
	_ context.Context,
	request preference.Request,
) (*preference.Response, error) {
	creator.request = request
	return creator.response, creator.err
}

func TestCheckoutClientRequiresPublicHTTPSCallbackURLs(t *testing.T) {
	client, err := NewCheckoutClient(Config{
		Environment:     sharedmercadopago.EnvironmentProduction,
		SuccessURL:      "http://localhost:3000/payments/success",
		PendingURL:      "https://app.loresuelvo.test/payments/pending",
		FailureURL:      "https://app.loresuelvo.test/payments/failure",
		NotificationURL: "https://api.loresuelvo.test/webhooks/mercado-pago",
	})

	assert.Nil(t, client)
	assert.True(t, errors.Is(err, ErrInvalidCheckoutConfiguration))
}

func TestCheckoutClientCreatesMarketplacePreferenceAndReturnsProductionInitPoint(t *testing.T) {
	startsOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	expiresOn := startsOn.Add(30 * time.Minute)
	client, err := NewCheckoutClient(Config{
		Environment:     sharedmercadopago.EnvironmentProduction,
		SuccessURL:      "https://app.loresuelvo.test/payments/success",
		PendingURL:      "https://app.loresuelvo.test/payments/pending",
		FailureURL:      "https://app.loresuelvo.test/payments/failure",
		NotificationURL: "https://api.loresuelvo.test/webhooks/mercado-pago",
	})
	require.NoError(t, err)
	creator := &preferenceCreatorStub{response: &preference.Response{
		ID:        "preference-123",
		InitPoint: "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=preference-123",
	}}
	var accessToken string
	client.preferenceClientFactory = func(token string) (preferenceCreator, error) {
		accessToken = token
		return creator, nil
	}

	checkout, err := client.CreateCheckout(context.Background(), "seller-access-token", payment.CheckoutRequest{
		ExternalReference: "payment-intent-123",
		Purpose:           payment.PurposeBookingDeposit,
		Currency:          "ARS",
		SellerAmountCents: 2000000,
		PlatformFeeCents:  100000,
		TotalAmountCents:  2100000,
		PayerEmail:        "ana@example.com",
		StartsOn:          startsOn,
		ExpiresOn:         expiresOn,
	})

	require.NoError(t, err)
	assert.Equal(t, "preference-123", checkout.ID)
	assert.Equal(t, "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=preference-123", checkout.URL)
	assert.Equal(t, "seller-access-token", accessToken)
	assert.Equal(t, "payment-intent-123", creator.request.ExternalReference)
	assert.Equal(t, float64(1000), creator.request.MarketplaceFee)
	assert.False(t, creator.request.BinaryMode)
	require.Len(t, creator.request.Items, 1)
	assert.Equal(t, "payment-intent-123", creator.request.Items[0].ID)
	assert.Equal(t, "Service booking deposit", creator.request.Items[0].Title)
	assert.Equal(t, "service", creator.request.Items[0].Type)
	assert.Equal(t, "ARS", creator.request.Items[0].CurrencyID)
	assert.Equal(t, float64(21000), creator.request.Items[0].UnitPrice)
	require.NotNil(t, creator.request.Payer)
	assert.Equal(t, "ana@example.com", creator.request.Payer.Email)
	require.NotNil(t, creator.request.BackURLs)
	assert.Equal(t, "https://app.loresuelvo.test/payments/success", creator.request.BackURLs.Success)
	assert.Equal(t, "https://app.loresuelvo.test/payments/pending", creator.request.BackURLs.Pending)
	assert.Equal(t, "https://app.loresuelvo.test/payments/failure", creator.request.BackURLs.Failure)
	assert.Equal(t, "https://api.loresuelvo.test/webhooks/mercado-pago?source_news=webhooks", creator.request.NotificationURL)
	require.NotNil(t, creator.request.PaymentMethods)
	require.Len(t, creator.request.PaymentMethods.ExcludedPaymentTypes, 1)
	assert.Equal(t, "ticket", creator.request.PaymentMethods.ExcludedPaymentTypes[0].ID)
	assert.True(t, creator.request.Expires)
	assert.Nil(t, creator.request.ExpirationDateFrom)
	require.NotNil(t, creator.request.ExpirationDateTo)
	assert.Equal(t, expiresOn, *creator.request.ExpirationDateTo)
}

func TestCheckoutClientForcesWebhookNotificationSource(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		inputURL string
		wantURL  string
	}{
		{
			name:     "source is absent",
			inputURL: "https://api.loresuelvo.test/webhooks/mercado-pago",
			wantURL:  "https://api.loresuelvo.test/webhooks/mercado-pago?source_news=webhooks",
		},
		{
			name:     "IPN source is configured",
			inputURL: "https://api.loresuelvo.test/webhooks/mercado-pago?source_news=ipn",
			wantURL:  "https://api.loresuelvo.test/webhooks/mercado-pago?source_news=webhooks",
		},
		{
			name:     "other query parameters are present",
			inputURL: "https://api.loresuelvo.test/webhooks/mercado-pago?seller=123",
			wantURL:  "https://api.loresuelvo.test/webhooks/mercado-pago?seller=123&source_news=webhooks",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := validCheckoutConfig(sharedmercadopago.EnvironmentProduction)
			config.NotificationURL = testCase.inputURL

			client, err := NewCheckoutClient(config)

			require.NoError(t, err)
			assert.Equal(t, testCase.wantURL, client.config.NotificationURL)
		})
	}
}

func TestCheckoutClientDescribesServiceBalancePreference(t *testing.T) {
	now := time.Date(2026, time.July, 6, 13, 0, 0, 0, time.UTC)
	client, err := NewCheckoutClient(Config{
		Environment:     sharedmercadopago.EnvironmentProduction,
		SuccessURL:      "https://app.loresuelvo.test/payments/success",
		PendingURL:      "https://app.loresuelvo.test/payments/pending",
		FailureURL:      "https://app.loresuelvo.test/payments/failure",
		NotificationURL: "https://api.loresuelvo.test/webhooks/mercado-pago",
	})
	require.NoError(t, err)
	creator := &preferenceCreatorStub{response: &preference.Response{
		ID:        "preference-balance",
		InitPoint: "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=preference-balance",
	}}
	client.preferenceClientFactory = func(string) (preferenceCreator, error) { return creator, nil }

	_, err = client.CreateCheckout(t.Context(), "seller-access-token", payment.CheckoutRequest{
		ExternalReference: "balance-intent-123",
		Purpose:           payment.PurposeServiceBalance,
		Currency:          "ARS",
		SellerAmountCents: 8000000,
		PlatformFeeCents:  400000,
		TotalAmountCents:  8400000,
		PayerEmail:        "ana@example.com",
		StartsOn:          now,
		ExpiresOn:         now.Add(30 * time.Minute),
	})

	require.NoError(t, err)
	require.Len(t, creator.request.Items, 1)
	assert.Equal(t, "Service balance", creator.request.Items[0].Title)
}

func TestSDKAmountFromCentsPreservesCentPrecisionAtAdapterBoundary(t *testing.T) {
	request := preference.Request{
		Items: []preference.ItemRequest{{
			UnitPrice: sdkAmountFromCents(10000003),
		}},
		MarketplaceFee: sdkAmountFromCents(500003),
	}

	encoded, err := json.Marshal(request)

	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"unit_price":100000.03`)
	assert.Contains(t, string(encoded), `"marketplace_fee":5000.03`)
}

func TestCheckoutClientGetsVerifiedPaymentWithSellerCredential(t *testing.T) {
	client, err := NewCheckoutClient(Config{
		Environment:     sharedmercadopago.EnvironmentProduction,
		SuccessURL:      "https://app.loresuelvo.test/payments/success",
		PendingURL:      "https://app.loresuelvo.test/payments/pending",
		FailureURL:      "https://app.loresuelvo.test/payments/failure",
		NotificationURL: "https://api.loresuelvo.test/webhooks/mercado-pago",
	})
	require.NoError(t, err)
	getter := &paymentGetterStub{response: &mercadopagopayment.Response{
		ID:                123456,
		CollectorID:       987654,
		ExternalReference: "f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
		Status:            "approved",
		CurrencyID:        "ARS",
		TransactionAmount: 21000,
	}}
	var accessToken string
	client.paymentClientFactory = func(token string) (paymentGetter, error) {
		accessToken = token
		return getter, nil
	}

	externalPayment, err := client.GetPayment(context.Background(), "seller-token", "123456")

	require.NoError(t, err)
	assert.Equal(t, 123456, getter.id)
	assert.Equal(t, "seller-token", accessToken)
	assert.Equal(t, "123456", externalPayment.ID)
	assert.Equal(t, "987654", externalPayment.SellerAccountID)
	assert.Equal(t, "f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36", externalPayment.ExternalReference)
	assert.Equal(t, payment.ExternalPaymentStatusApproved, externalPayment.Status)
	assert.Equal(t, "ARS", externalPayment.Currency)
	assert.Equal(t, int64(2100000), externalPayment.AmountCents)
}

func TestCheckoutClientMapsMercadoPagoPendingStatusesToProcessing(t *testing.T) {
	for _, mercadoPagoStatus := range []string{"pending", "in_process"} {
		t.Run(mercadoPagoStatus, func(t *testing.T) {
			client, err := NewCheckoutClient(Config{
				Environment:     sharedmercadopago.EnvironmentProduction,
				SuccessURL:      "https://app.loresuelvo.test/payments/success",
				PendingURL:      "https://app.loresuelvo.test/payments/pending",
				FailureURL:      "https://app.loresuelvo.test/payments/failure",
				NotificationURL: "https://api.loresuelvo.test/webhooks/mercado-pago",
			})
			require.NoError(t, err)
			client.paymentClientFactory = func(string) (paymentGetter, error) {
				return &paymentGetterStub{response: &mercadopagopayment.Response{
					ID:                123456,
					CollectorID:       987654,
					ExternalReference: "f69bfe31-ce5d-4f85-a8c5-643ca2dcaa36",
					Status:            mercadoPagoStatus,
					CurrencyID:        "ARS",
					TransactionAmount: 21000,
				}}, nil
			}

			externalPayment, err := client.GetPayment(
				context.Background(),
				"seller-token",
				"123456",
			)

			require.NoError(t, err)
			assert.Equal(t, payment.ExternalPaymentStatusProcessing, externalPayment.Status)
		})
	}
}

func TestCheckoutClientReturnsSandboxInitPoint(t *testing.T) {
	client, err := NewCheckoutClient(validCheckoutConfig(sharedmercadopago.EnvironmentSandbox))
	require.NoError(t, err)
	creator := &preferenceCreatorStub{response: &preference.Response{
		ID:               "preference-sandbox",
		InitPoint:        "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=preference-sandbox",
		SandboxInitPoint: "https://sandbox.mercadopago.com.ar/checkout/v1/redirect?pref_id=preference-sandbox",
	}}
	client.preferenceClientFactory = func(string) (preferenceCreator, error) { return creator, nil }

	checkout, err := client.CreateCheckout(t.Context(), "seller-test-token", validCheckoutRequest())

	require.NoError(t, err)
	assert.Equal(t, "https://sandbox.mercadopago.com.ar/checkout/v1/redirect?pref_id=preference-sandbox", checkout.URL)
}

func TestCheckoutClientRejectsResponseWithoutEnvironmentInitPoint(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		environment sharedmercadopago.Environment
		response    *preference.Response
	}{
		{
			name:        "sandbox with only production URL",
			environment: sharedmercadopago.EnvironmentSandbox,
			response: &preference.Response{
				ID:        "preference-123",
				InitPoint: "https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=preference-123",
			},
		},
		{
			name:        "production with only sandbox URL",
			environment: sharedmercadopago.EnvironmentProduction,
			response: &preference.Response{
				ID:               "preference-123",
				SandboxInitPoint: "https://sandbox.mercadopago.com.ar/checkout/v1/redirect?pref_id=preference-123",
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client, err := NewCheckoutClient(validCheckoutConfig(testCase.environment))
			require.NoError(t, err)
			client.preferenceClientFactory = func(string) (preferenceCreator, error) {
				return &preferenceCreatorStub{response: testCase.response}, nil
			}

			checkout, err := client.CreateCheckout(t.Context(), "seller-token", validCheckoutRequest())

			assert.Empty(t, checkout)
			assert.ErrorContains(t, err, "required checkout data is missing")
		})
	}
}

func TestNewCheckoutClientFromEnvRejectsMissingOrInvalidEnvironment(t *testing.T) {
	t.Setenv("PAYMENT_CHECKOUT_SUCCESS_URL", "https://app.loresuelvo.test/payments/success")
	t.Setenv("PAYMENT_CHECKOUT_PENDING_URL", "https://app.loresuelvo.test/payments/pending")
	t.Setenv("PAYMENT_CHECKOUT_FAILURE_URL", "https://app.loresuelvo.test/payments/failure")
	t.Setenv("MERCADO_PAGO_NOTIFICATION_URL", "https://api.loresuelvo.test/webhooks/mercado-pago")

	for _, environment := range []string{"", "test"} {
		t.Run(environment, func(t *testing.T) {
			t.Setenv("MERCADO_PAGO_ENVIRONMENT", environment)

			client, err := NewCheckoutClientFromEnv()

			assert.Nil(t, client)
			assert.ErrorIs(t, err, ErrInvalidCheckoutConfiguration)
		})
	}
}

func validCheckoutConfig(environment sharedmercadopago.Environment) Config {
	return Config{
		Environment:     environment,
		SuccessURL:      "https://app.loresuelvo.test/payments/success",
		PendingURL:      "https://app.loresuelvo.test/payments/pending",
		FailureURL:      "https://app.loresuelvo.test/payments/failure",
		NotificationURL: "https://api.loresuelvo.test/webhooks/mercado-pago",
	}
}

func validCheckoutRequest() payment.CheckoutRequest {
	startsOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	return payment.CheckoutRequest{
		ExternalReference: "payment-intent-123",
		Purpose:           payment.PurposeBookingDeposit,
		Currency:          "ARS",
		SellerAmountCents: 2000000,
		PlatformFeeCents:  100000,
		TotalAmountCents:  2100000,
		PayerEmail:        "ana@example.com",
		StartsOn:          startsOn,
		ExpiresOn:         startsOn.Add(30 * time.Minute),
	}
}
