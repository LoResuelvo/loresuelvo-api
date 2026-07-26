package mercadopago

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

type FakeCheckoutClient struct {
	mu       sync.Mutex
	requests []payment.CheckoutRequest
	payments map[string]payment.ExternalPayment
}

func NewFakeCheckoutClient() *FakeCheckoutClient {
	return &FakeCheckoutClient{payments: make(map[string]payment.ExternalPayment)}
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

func (client *FakeCheckoutClient) AddApprovedPayment(
	externalReference,
	sellerAccountID string,
	amountCents int64,
) string {
	return client.addPayment(
		externalReference,
		sellerAccountID,
		amountCents,
		payment.ExternalPaymentStatusApproved,
	)
}

func (client *FakeCheckoutClient) AddProcessingPayment(
	externalReference,
	sellerAccountID string,
	amountCents int64,
) string {
	return client.addPayment(
		externalReference,
		sellerAccountID,
		amountCents,
		payment.ExternalPaymentStatusProcessing,
	)
}

func (client *FakeCheckoutClient) addPayment(
	externalReference,
	sellerAccountID string,
	amountCents int64,
	status payment.ExternalPaymentStatus,
) string {
	client.mu.Lock()
	defer client.mu.Unlock()
	externalPaymentID := "fake-payment-" + externalReference
	client.payments[externalPaymentID] = payment.ExternalPayment{
		ID:                externalPaymentID,
		SellerAccountID:   sellerAccountID,
		ExternalReference: externalReference,
		Status:            status,
		Currency:          "ARS",
		AmountCents:       amountCents,
	}
	return externalPaymentID
}

func (client *FakeCheckoutClient) GetPayment(
	_ context.Context,
	_,
	externalPaymentID string,
) (payment.ExternalPayment, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	externalPayment, exists := client.payments[externalPaymentID]
	if !exists {
		return payment.ExternalPayment{}, fmt.Errorf("fake Mercado Pago payment %q does not exist", externalPaymentID)
	}
	return externalPayment, nil
}

func (client *FakeCheckoutClient) Reset() {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.requests = nil
	client.payments = make(map[string]payment.ExternalPayment)
}
