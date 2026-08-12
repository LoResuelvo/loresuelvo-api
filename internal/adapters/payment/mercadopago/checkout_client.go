package mercadopago

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	sharedmercadopago "github.com/LoResuelvo/loresuelvo-api/internal/adapters/mercadopago"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	mercadopagoconfig "github.com/mercadopago/sdk-go/pkg/config"
	mercadopagopayment "github.com/mercadopago/sdk-go/pkg/payment"
	"github.com/mercadopago/sdk-go/pkg/preference"
)

const paymentProvider = paymentaccount.PaymentProvider("mercado_pago")

type preferenceCreator interface {
	Create(ctx context.Context, request preference.Request) (*preference.Response, error)
}

type preferenceClientFactory func(accessToken string) (preferenceCreator, error)

type paymentGetter interface {
	Get(ctx context.Context, id int) (*mercadopagopayment.Response, error)
}

type paymentClientFactory func(accessToken string) (paymentGetter, error)

type CheckoutClient struct {
	config                  Config
	preferenceClientFactory preferenceClientFactory
	paymentClientFactory    paymentClientFactory
}

func NewCheckoutClient(config Config) (*CheckoutClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &CheckoutClient{
		config:                  config,
		preferenceClientFactory: newSDKPreferenceClient,
		paymentClientFactory:    newSDKPaymentClient,
	}, nil
}

func NewCheckoutClientFromEnv() (*CheckoutClient, error) {
	config, err := NewConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewCheckoutClient(config)
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

func newSDKPaymentClient(accessToken string) (paymentGetter, error) {
	sdkConfig, err := mercadopagoconfig.New(accessToken)
	if err != nil {
		return nil, fmt.Errorf("configuring Mercado Pago SDK: %w", err)
	}
	return mercadopagopayment.NewClient(sdkConfig), nil
}

func (client *CheckoutClient) CreateCheckout(
	ctx context.Context,
	accessToken string,
	checkoutRequest payment.CheckoutRequest,
) (payment.ExternalCheckout, error) {
	if strings.TrimSpace(accessToken) == "" ||
		strings.TrimSpace(checkoutRequest.ExternalReference) == "" ||
		!validCheckoutPurpose(checkoutRequest.Purpose) ||
		checkoutRequest.Currency != "ARS" ||
		checkoutRequest.SellerAmountCents <= 0 ||
		checkoutRequest.PlatformFeeCents < 0 ||
		checkoutRequest.TotalAmountCents != checkoutRequest.SellerAmountCents+checkoutRequest.PlatformFeeCents ||
		strings.TrimSpace(checkoutRequest.PayerEmail) == "" ||
		checkoutRequest.StartsOn.IsZero() ||
		checkoutRequest.ExpiresOn.IsZero() ||
		!checkoutRequest.ExpiresOn.After(checkoutRequest.StartsOn) {
		return payment.ExternalCheckout{}, fmt.Errorf("creating Mercado Pago preference: invalid checkout request")
	}
	request := preference.Request{
		Items: []preference.ItemRequest{{
			ID:         checkoutRequest.ExternalReference,
			Title:      checkoutTitle(checkoutRequest.Purpose),
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
		ExternalReference:  checkoutRequest.ExternalReference,
		NotificationURL:    client.config.NotificationURL,
		MarketplaceFee:     sdkAmountFromCents(checkoutRequest.PlatformFeeCents),
		BinaryMode:         false,
		Expires:            true,
		ExpirationDateFrom: &checkoutRequest.StartsOn,
		ExpirationDateTo:   &checkoutRequest.ExpiresOn,
	}
	preferenceClient, err := client.preferenceClientFactory(accessToken)
	if err != nil {
		return payment.ExternalCheckout{}, err
	}
	preferenceResponse, err := preferenceClient.Create(ctx, request)
	if err != nil {
		return payment.ExternalCheckout{}, fmt.Errorf("creating Mercado Pago preference with SDK: %w", err)
	}
	if preferenceResponse == nil || strings.TrimSpace(preferenceResponse.ID) == "" {
		return payment.ExternalCheckout{}, fmt.Errorf("decoding Mercado Pago preference response: required checkout data is missing")
	}
	checkoutURL := checkoutURLForEnvironment(client.config.Environment, preferenceResponse)
	if strings.TrimSpace(checkoutURL) == "" {
		return payment.ExternalCheckout{}, fmt.Errorf("decoding Mercado Pago preference response: required checkout data is missing")
	}
	return payment.ExternalCheckout{
		ID:  preferenceResponse.ID,
		URL: checkoutURL,
	}, nil
}

func checkoutURLForEnvironment(environment sharedmercadopago.Environment, response *preference.Response) string {
	if environment.IsSandbox() {
		return response.SandboxInitPoint
	}
	return response.InitPoint
}

func validCheckoutPurpose(purpose payment.Purpose) bool {
	return purpose == payment.PurposeBookingDeposit || purpose == payment.PurposeServiceBalance
}

func checkoutTitle(purpose payment.Purpose) string {
	if purpose == payment.PurposeServiceBalance {
		return "Service balance"
	}
	return "Service booking deposit"
}

func (client *CheckoutClient) GetPayment(
	ctx context.Context,
	accessToken,
	externalPaymentID string,
) (payment.ExternalPayment, error) {
	if strings.TrimSpace(accessToken) == "" {
		return payment.ExternalPayment{}, fmt.Errorf("getting Mercado Pago payment: access token is required")
	}
	paymentID, err := strconv.Atoi(strings.TrimSpace(externalPaymentID))
	if err != nil || paymentID <= 0 {
		return payment.ExternalPayment{}, fmt.Errorf("getting Mercado Pago payment: invalid payment id")
	}
	paymentClient, err := client.paymentClientFactory(accessToken)
	if err != nil {
		return payment.ExternalPayment{}, err
	}
	response, err := paymentClient.Get(ctx, paymentID)
	if err != nil {
		return payment.ExternalPayment{}, fmt.Errorf("getting Mercado Pago payment with SDK: %w", err)
	}
	if response == nil {
		return payment.ExternalPayment{}, fmt.Errorf("getting Mercado Pago payment: empty response")
	}
	amountCents, err := centsFromSDKAmount(response.TransactionAmount)
	if err != nil {
		return payment.ExternalPayment{}, fmt.Errorf("getting Mercado Pago payment: %w", err)
	}
	return payment.ExternalPayment{
		ID:                strconv.Itoa(response.ID),
		SellerAccountID:   strconv.FormatInt(response.CollectorID, 10),
		ExternalReference: response.ExternalReference,
		Status:            externalPaymentStatus(response.Status),
		Currency:          response.CurrencyID,
		AmountCents:       amountCents,
	}, nil
}

func externalPaymentStatus(status string) payment.ExternalPaymentStatus {
	switch status {
	case "pending", "in_process":
		return payment.ExternalPaymentStatusProcessing
	default:
		return payment.ExternalPaymentStatus(status)
	}
}

func sdkAmountFromCents(cents int64) float64 {
	return float64(cents) / 100
}

func centsFromSDKAmount(amount float64) (int64, error) {
	cents := amount * 100
	rounded := math.Round(cents)
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || math.Abs(cents-rounded) > 0.000001 {
		return 0, fmt.Errorf("invalid transaction amount")
	}
	return int64(rounded), nil
}
