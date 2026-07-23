package mercadopago

import (
	"context"
	"net/url"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

type FakeCheckoutClient struct {
	mu       sync.Mutex
	requests []payment.CheckoutRequest
}

func NewFakeCheckoutClient() *FakeCheckoutClient {
	return &FakeCheckoutClient{}
}

func (client *FakeCheckoutClient) Provider() paymentaccount.PaymentProvider {
	return paymentProvider
}

func (client *FakeCheckoutClient) CreateCheckout(
	_ context.Context,
	_ string,
	request payment.CheckoutRequest,
) (payment.ExternalCheckout, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.requests = append(client.requests, request)

	preferenceID := "fake-preference-" + request.ExternalReference
	checkoutURL := &url.URL{
		Scheme: "https",
		Host:   "checkout.mercadopago.test",
		Path:   "/preferences/" + preferenceID,
	}
	return payment.ExternalCheckout{ID: preferenceID, URL: checkoutURL.String()}, nil
}

func (client *FakeCheckoutClient) Reset() {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.requests = nil
}
