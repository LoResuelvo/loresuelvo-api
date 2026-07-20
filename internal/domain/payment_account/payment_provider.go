package paymentaccount

import "strings"

type PaymentProvider string

func NewPaymentProvider(value string) (PaymentProvider, error) {
	provider := PaymentProvider(strings.TrimSpace(value))
	if provider == "" {
		return "", ErrPaymentProviderRequired
	}
	return provider, nil
}
