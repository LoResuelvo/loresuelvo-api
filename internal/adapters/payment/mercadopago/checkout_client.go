package mercadopago

import (
	"context"
	"fmt"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	mercadopagoconfig "github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/preference"
)

const paymentProvider = paymentaccount.PaymentProvider("mercado_pago")

type preferenceCreator interface {
	Create(ctx context.Context, request preference.Request) (*preference.Response, error)
}

type preferenceClientFactory func(accessToken string) (preferenceCreator, error)

type CheckoutClient struct {
	config                  Config
	preferenceClientFactory preferenceClientFactory
}

func NewCheckoutClient(config Config) (*CheckoutClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &CheckoutClient{
		config:                  config,
		preferenceClientFactory: newSDKPreferenceClient,
	}, nil
}

func NewCheckoutClientFromEnv() (*CheckoutClient, error) {
	return NewCheckoutClient(NewConfigFromEnv())
}

func (client *CheckoutClient) Provider() paymentaccount.PaymentProvider {
	return paymentProvider
}

func newSDKPreferenceClient(accessToken string) (preferenceCreator, error) {
	sdkConfig, err := mercadopagoconfig.New(accessToken)
	if err != nil {
		return nil, fmt.Errorf("configuring Mercado Pago SDK: %w", err)
	}
	return preference.NewClient(sdkConfig), nil
}

func (client *CheckoutClient) CreateCheckout(
	ctx context.Context,
	accessToken string,
	checkoutRequest payment.CheckoutRequest,
) (payment.ExternalCheckout, error) {
	if strings.TrimSpace(accessToken) == "" ||
		strings.TrimSpace(checkoutRequest.ExternalReference) == "" ||
		checkoutRequest.Currency != "ARS" ||
		checkoutRequest.SellerAmountCents <= 0 ||
		checkoutRequest.PlatformFeeCents < 0 ||
		checkoutRequest.TotalAmountCents != checkoutRequest.SellerAmountCents+checkoutRequest.PlatformFeeCents ||
		strings.TrimSpace(checkoutRequest.PayerEmail) == "" {
		return payment.ExternalCheckout{}, fmt.Errorf("creating Mercado Pago preference: invalid checkout request")
	}
	request := preference.Request{
		Items: []preference.ItemRequest{{
			ID:         checkoutRequest.ExternalReference,
			Title:      "Service booking deposit",
			Type:       "service",
			CurrencyID: checkoutRequest.Currency,
			Quantity:   1,
			UnitPrice:  sdkAmountFromCents(checkoutRequest.TotalAmountCents),
		}},
		Payer: &preference.PayerRequest{Email: checkoutRequest.PayerEmail},
		BackURLs: &preference.BackURLsRequest{
			Success: client.config.SuccessURL,
			Pending: client.config.PendingURL,
			Failure: client.config.FailureURL,
		},
		PaymentMethods: &preference.PaymentMethodsRequest{
			ExcludedPaymentTypes: []preference.ExcludedPaymentTypeRequest{{ID: "ticket"}},
		},
		ExternalReference: checkoutRequest.ExternalReference,
		NotificationURL:   client.config.NotificationURL,
		MarketplaceFee:    sdkAmountFromCents(checkoutRequest.PlatformFeeCents),
		BinaryMode:        false,
	}
	preferenceClient, err := client.preferenceClientFactory(accessToken)
	if err != nil {
		return payment.ExternalCheckout{}, err
	}
	preferenceResponse, err := preferenceClient.Create(ctx, request)
	if err != nil {
		return payment.ExternalCheckout{}, fmt.Errorf("creating Mercado Pago preference with SDK: %w", err)
	}
	if preferenceResponse == nil ||
		strings.TrimSpace(preferenceResponse.ID) == "" ||
		strings.TrimSpace(preferenceResponse.InitPoint) == "" {
		return payment.ExternalCheckout{}, fmt.Errorf("decoding Mercado Pago preference response: required checkout data is missing")
	}
	return payment.ExternalCheckout{
		ID:  preferenceResponse.ID,
		URL: preferenceResponse.InitPoint,
	}, nil
}

func sdkAmountFromCents(cents int64) float64 {
	return float64(cents) / 100
}
