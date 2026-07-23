package mercadopago

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookVerifierValidatesMercadoPagoSignature(t *testing.T) {
	const (
		secret    = "webhook-secret"
		requestID = "request-123"
		dataID    = "123456"
		timestamp = "1784836800000"
	)
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte("id:" + dataID + ";request-id:" + requestID + ";ts:" + timestamp + ";"))
	require.NoError(t, err)
	signature := "ts=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
	verifier, err := NewWebhookVerifier(secret)
	require.NoError(t, err)

	assert.NoError(t, verifier.Validate(signature, requestID, dataID))
	assert.Error(t, verifier.Validate(signature, requestID, "tampered"))
}
