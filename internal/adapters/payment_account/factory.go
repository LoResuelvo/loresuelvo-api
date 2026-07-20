package paymentaccountadapter

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account/mercadopago"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

const defaultPaymentProvider = "mercado_pago"

var ErrUnsupportedPaymentProvider = errors.New("unsupported payment account provider")

func NewOAuthConnectorFromEnv() (paymentaccount.OAuthConnector, error) {
	provider := paymentProviderFromEnv()
	switch provider {
	case defaultPaymentProvider:
		client, err := mercadopago.NewOAuthClientFromEnv()
		if err != nil {
			return nil, fmt.Errorf("configuring Mercado Pago OAuth: %w", err)
		}
		return client, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedPaymentProvider, provider)
	}
}

func paymentProviderFromEnv() string {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_ACCOUNT_PROVIDER")))
	if provider == "" {
		return defaultPaymentProvider
	}
	return provider
}
