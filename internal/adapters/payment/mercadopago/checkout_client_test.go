package mercadopago

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/mercadopago/sdk-go/pkg/preference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type preferenceCreatorStub struct {
	request  preference.Request
	response *preference.Response
	err      error
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
		SuccessURL:      "http://localhost:3000/payments/success",
		PendingURL:      "https://app.loresuelvo.test/payments/pending",
		FailureURL:      "https://app.loresuelvo.test/payments/failure",
		NotificationURL: "https://api.loresuelvo.test/webhooks/mercado-pago",
	})

	assert.Nil(t, client)
	assert.True(t, errors.Is(err, ErrInvalidCheckoutConfiguration))
}

func TestCheckoutClientCreatesMarketplacePreferenceAndReturnsInitPoint(t *testing.T) {
	startsOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	expiresOn := startsOn.Add(30 * time.Minute)
	client, err := NewCheckoutClient(Config{
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
	assert.Equal(t, "service", creator.request.Items[0].Type)
	assert.Equal(t, "ARS", creator.request.Items[0].CurrencyID)
	assert.Equal(t, float64(21000), creator.request.Items[0].UnitPrice)
	require.NotNil(t, creator.request.Payer)
	assert.Equal(t, "ana@example.com", creator.request.Payer.Email)
	require.NotNil(t, creator.request.BackURLs)
	assert.Equal(t, "https://app.loresuelvo.test/payments/success", creator.request.BackURLs.Success)
	assert.Equal(t, "https://app.loresuelvo.test/payments/pending", creator.request.BackURLs.Pending)
	assert.Equal(t, "https://app.loresuelvo.test/payments/failure", creator.request.BackURLs.Failure)
	assert.Equal(t, "https://api.loresuelvo.test/webhooks/mercado-pago", creator.request.NotificationURL)
	require.NotNil(t, creator.request.PaymentMethods)
	require.Len(t, creator.request.PaymentMethods.ExcludedPaymentTypes, 1)
	assert.Equal(t, "ticket", creator.request.PaymentMethods.ExcludedPaymentTypes[0].ID)
	assert.True(t, creator.request.Expires)
	require.NotNil(t, creator.request.ExpirationDateFrom)
	assert.Equal(t, startsOn, *creator.request.ExpirationDateFrom)
	require.NotNil(t, creator.request.ExpirationDateTo)
	assert.Equal(t, expiresOn, *creator.request.ExpirationDateTo)
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
