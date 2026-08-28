package steps_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const testDiditWebhookSecret = "test-didit-webhook-secret"

type identityVerificationWebhookSignerStub struct {
	secret string
}

func newIdentityVerificationWebhookSignerStub() identityVerificationWebhookSignerStub {
	return identityVerificationWebhookSignerStub{secret: testDiditWebhookSecret}
}

func (stub identityVerificationWebhookSignerStub) Sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(stub.secret))
	if _, err := mac.Write(body); err != nil {
		panic(err)
	}
	return hex.EncodeToString(mac.Sum(nil))
}
