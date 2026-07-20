package paymentaccount

import (
	"strings"
	"time"
)

const StatusConnected = "connected"

type PaymentAccount struct {
	providerID             int
	paymentProvider        PaymentProvider
	externalAccountID      string
	accessTokenCiphertext  []byte
	refreshTokenCiphertext []byte
	tokenExpiresOn         time.Time
}

func NewPaymentAccount(
	providerID int,
	paymentProvider PaymentProvider,
	externalAccountID string,
	accessTokenCiphertext,
	refreshTokenCiphertext []byte,
	tokenExpiresOn time.Time,
) (*PaymentAccount, error) {
	if paymentProvider == "" {
		return nil, ErrPaymentProviderRequired
	}
	externalAccountID = strings.TrimSpace(externalAccountID)
	if externalAccountID == "" {
		return nil, ErrExternalAccountIDRequired
	}
	if len(accessTokenCiphertext) == 0 {
		return nil, ErrAccessTokenRequired
	}
	return &PaymentAccount{
		providerID:             providerID,
		paymentProvider:        paymentProvider,
		externalAccountID:      externalAccountID,
		accessTokenCiphertext:  append([]byte(nil), accessTokenCiphertext...),
		refreshTokenCiphertext: append([]byte(nil), refreshTokenCiphertext...),
		tokenExpiresOn:         tokenExpiresOn.UTC(),
	}, nil
}

func (account *PaymentAccount) ProviderID() int                  { return account.providerID }
func (account *PaymentAccount) PaymentProvider() PaymentProvider { return account.paymentProvider }
func (account *PaymentAccount) ExternalAccountID() string        { return account.externalAccountID }
func (account *PaymentAccount) AccessTokenCiphertext() []byte {
	return append([]byte(nil), account.accessTokenCiphertext...)
}
func (account *PaymentAccount) RefreshTokenCiphertext() []byte {
	return append([]byte(nil), account.refreshTokenCiphertext...)
}
func (account *PaymentAccount) TokenExpiresOn() time.Time { return account.tokenExpiresOn }
func (account *PaymentAccount) Status() string            { return StatusConnected }
func (account *PaymentAccount) CanReceivePayments() bool {
	return account.Status() == StatusConnected
}
func (account *PaymentAccount) CanSendServiceProposals() bool { return account.CanReceivePayments() }
