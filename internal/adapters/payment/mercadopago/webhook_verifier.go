package mercadopago

import (
	"fmt"
	"os"
	"strings"

	"github.com/mercadopago/sdk-go/pkg/webhook"
)

type WebhookVerifier struct {
	secret string
}

func NewWebhookVerifier(secret string) (*WebhookVerifier, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("configuring Mercado Pago webhook verifier: secret is required")
	}
	return &WebhookVerifier{secret: secret}, nil
}

func NewWebhookVerifierFromEnv() (*WebhookVerifier, error) {
	return NewWebhookVerifier(os.Getenv("MERCADO_PAGO_WEBHOOK_SECRET"))
}

func (verifier *WebhookVerifier) Validate(signature, requestID, dataID string) error {
	if err := webhook.ValidateSignature(signature, requestID, dataID, verifier.secret); err != nil {
		return fmt.Errorf("validating Mercado Pago webhook signature: %w", err)
	}
	return nil
}
