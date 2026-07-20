package paymentaccount_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	providerAuthID = "auth0|provider-payment-account-test"
	providerID     = 42
)

var fixedNow = time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)

type userFinderStub struct {
	found user.User
	err   error
}

func (stub userFinderStub) FindByAuthID(string) (user.User, error) {
	return stub.found, stub.err
}

type secretGeneratorStub struct {
	values []string
}

func (stub *secretGeneratorStub) Generate() (string, error) {
	value := stub.values[0]
	stub.values = stub.values[1:]
	return value, nil
}

type clockStub struct{}

func (clockStub) Now() time.Time { return fixedNow }

type credentialProtectorStub struct {
	encryptedPlaintexts []string
	decryptedValues     [][]byte
}

func (stub *credentialProtectorStub) Encrypt(plaintext string) ([]byte, error) {
	stub.encryptedPlaintexts = append(stub.encryptedPlaintexts, plaintext)
	return []byte("encrypted:" + plaintext), nil
}

func (stub *credentialProtectorStub) Decrypt(ciphertext []byte) (string, error) {
	stub.decryptedValues = append(stub.decryptedValues, append([]byte(nil), ciphertext...))
	return string(ciphertext[len("encrypted:"):]), nil
}

type oauthConnectorStub struct {
	provider              paymentaccount.PaymentProvider
	authorizationURL      string
	authorizationErr      error
	credentials           paymentaccount.OAuthCredentials
	authorizationVerifier string
	exchangedCode         string
	exchangedVerifier     string
}

func (stub *oauthConnectorStub) Provider() paymentaccount.PaymentProvider {
	if stub.provider != "" {
		return stub.provider
	}
	return paymentaccount.PaymentProvider("mercado_pago")
}

func (stub *oauthConnectorStub) AuthorizationURL(state, codeVerifier string) (string, error) {
	if stub.authorizationErr != nil {
		return "", stub.authorizationErr
	}
	stub.authorizationVerifier = codeVerifier
	return stub.authorizationURL + "?state=" + state, nil
}

func (stub *oauthConnectorStub) ExchangeAuthorizationCode(_ context.Context, code, codeVerifier string) (paymentaccount.OAuthCredentials, error) {
	stub.exchangedCode = code
	stub.exchangedVerifier = codeVerifier
	return stub.credentials, nil
}

type repositoryStub struct {
	savedAttempt    *paymentaccount.AuthorizationAttempt
	foundAttempt    *paymentaccount.AuthorizationAttempt
	savedAccount    *paymentaccount.PaymentAccount
	foundAccount    *paymentaccount.PaymentAccount
	consumedAttempt *paymentaccount.AuthorizationAttempt
	consumeErr      error
	saveAccountErr  error
}

func (stub *repositoryStub) Save(_ context.Context, attempt *paymentaccount.AuthorizationAttempt) error {
	stub.savedAttempt = attempt
	return nil
}

func (stub *repositoryStub) FindByStateDigest(_ context.Context, _ []byte) (*paymentaccount.AuthorizationAttempt, error) {
	return stub.foundAttempt, nil
}

func (stub *repositoryStub) Consume(_ context.Context, attempt *paymentaccount.AuthorizationAttempt) error {
	stub.consumedAttempt = attempt
	return stub.consumeErr
}

func (stub *repositoryStub) SaveFromAuthorization(_ context.Context, _ int, account *paymentaccount.PaymentAccount) error {
	stub.savedAccount = account
	return stub.saveAccountErr
}

func (stub *repositoryStub) FindByProviderID(_ context.Context, _ int, _ paymentaccount.PaymentProvider) (*paymentaccount.PaymentAccount, error) {
	if stub.foundAccount == nil {
		return nil, paymentaccount.ErrConnectionNotFound
	}
	return stub.foundAccount, nil
}

func TestRejectAuthorizationConsumesMatchingAttemptWithoutConnectingAccount(t *testing.T) {
	attempt := paymentaccount.NewAuthorizationAttempt(
		providerID,
		paymentaccount.PaymentProvider("mercado_pago"),
		[]byte("stored-state-digest"),
		[]byte("encrypted:pkce-verifier"),
		fixedNow.Add(10*time.Minute),
	)
	repository := &repositoryStub{foundAttempt: attempt}
	oauthConnector := &oauthConnectorStub{}
	credentialProtector := &credentialProtectorStub{}
	service := paymentaccount.NewService(
		userFinderStub{}, repository, repository, oauthConnector,
		credentialProtector, &secretGeneratorStub{}, clockStub{},
	)

	err := service.RejectAuthorization(context.Background(), "state-secret")

	require.NoError(t, err)
	assert.Same(t, attempt, repository.consumedAttempt)
	assert.Nil(t, repository.savedAccount)
	assert.Empty(t, oauthConnector.exchangedCode)
	assert.Empty(t, credentialProtector.decryptedValues)
}

func TestRejectAuthorizationDoesNotConsumeExpiredAttempt(t *testing.T) {
	attempt := paymentaccount.NewAuthorizationAttempt(
		providerID,
		paymentaccount.PaymentProvider("mercado_pago"),
		[]byte("stored-state-digest"),
		[]byte("encrypted:pkce-verifier"),
		fixedNow,
	)
	repository := &repositoryStub{foundAttempt: attempt}
	service := paymentaccount.NewService(
		userFinderStub{}, repository, repository, &oauthConnectorStub{},
		&credentialProtectorStub{}, &secretGeneratorStub{}, clockStub{},
	)

	err := service.RejectAuthorization(context.Background(), "state-secret")

	require.ErrorIs(t, err, paymentaccount.ErrAuthorizationAttemptExpired)
	assert.Nil(t, repository.consumedAttempt)
}

func TestRejectAuthorizationReturnsConsumeError(t *testing.T) {
	attempt := paymentaccount.NewAuthorizationAttempt(
		providerID,
		paymentaccount.PaymentProvider("mercado_pago"),
		[]byte("stored-state-digest"),
		[]byte("encrypted:pkce-verifier"),
		fixedNow.Add(10*time.Minute),
	)
	repository := &repositoryStub{foundAttempt: attempt, consumeErr: assert.AnError}
	service := paymentaccount.NewService(
		userFinderStub{}, repository, repository, &oauthConnectorStub{},
		&credentialProtectorStub{}, &secretGeneratorStub{}, clockStub{},
	)

	err := service.RejectAuthorization(context.Background(), "state-secret")

	require.ErrorIs(t, err, assert.AnError)
	assert.ErrorContains(t, err, "consuming rejected payment account authorization")
}

func TestStartAuthorizationRejectsAlreadyConnectedPaymentAccount(t *testing.T) {
	providerUser := registeredProvider(t)
	connectedAccount, err := paymentaccount.NewPaymentAccount(
		providerID,
		paymentaccount.PaymentProvider("mercado_pago"),
		"mp-juan",
		[]byte("encrypted-access-token"),
		nil,
		fixedNow.Add(time.Hour),
		true,
	)
	require.NoError(t, err)
	repository := &repositoryStub{foundAccount: connectedAccount}
	service := paymentaccount.NewService(
		userFinderStub{found: providerUser},
		repository,
		repository,
		&oauthConnectorStub{},
		&credentialProtectorStub{},
		&secretGeneratorStub{},
		clockStub{},
	)

	_, err = service.StartAuthorization(context.Background(), providerAuthID)

	require.ErrorIs(t, err, paymentaccount.ErrAlreadyConnected)
	assert.Nil(t, repository.savedAttempt)
}

func TestStartAuthorizationCreatesProviderBoundAttemptAndPKCEURL(t *testing.T) {
	providerUser := registeredProvider(t)
	repository := &repositoryStub{}
	oauthConnector := &oauthConnectorStub{authorizationURL: "https://auth.payments.test/authorization"}
	credentialProtector := &credentialProtectorStub{}
	service := paymentaccount.NewService(
		userFinderStub{found: providerUser},
		repository,
		repository,
		oauthConnector,
		credentialProtector,
		&secretGeneratorStub{values: []string{"state-secret", "pkce-verifier"}},
		clockStub{},
	)

	authorization, err := service.StartAuthorization(context.Background(), providerAuthID)

	require.NoError(t, err)
	require.NotNil(t, repository.savedAttempt)
	assert.Equal(t, providerID, repository.savedAttempt.ProviderID)
	assert.Equal(t, paymentaccount.PaymentProvider("mercado_pago"), repository.savedAttempt.PaymentProvider)
	assert.NotEqual(t, []byte("state-secret"), repository.savedAttempt.StateDigest)
	assert.Equal(t, []byte("encrypted:pkce-verifier"), repository.savedAttempt.CodeVerifierCiphertext)
	assert.Equal(t, []string{"pkce-verifier"}, credentialProtector.encryptedPlaintexts)
	assert.Equal(t, "pkce-verifier", oauthConnector.authorizationVerifier)
	assert.Equal(t, fixedNow.Add(10*time.Minute), repository.savedAttempt.ExpiresOn)
	assert.Equal(t, "state-secret", authorization.State)
	assert.Contains(t, authorization.URL, "state=state-secret")
}

func TestStartAuthorizationDoesNotPersistAttemptWhenAuthorizationURLCannotBeBuilt(t *testing.T) {
	providerUser := registeredProvider(t)
	repository := &repositoryStub{}
	service := paymentaccount.NewService(
		userFinderStub{found: providerUser},
		repository,
		repository,
		&oauthConnectorStub{authorizationErr: errors.New("authorization URL unavailable")},
		&credentialProtectorStub{},
		&secretGeneratorStub{values: []string{"state-secret", "pkce-verifier"}},
		clockStub{},
	)

	_, err := service.StartAuthorization(context.Background(), providerAuthID)

	require.Error(t, err)
	assert.Nil(t, repository.savedAttempt)
}

func TestCompleteAuthorizationExchangesCodeAndConnectsAccount(t *testing.T) {
	providerUser := registeredProvider(t)
	attempt := paymentaccount.NewAuthorizationAttempt(providerID, paymentaccount.PaymentProvider("mercado_pago"), []byte("stored-state-digest"), []byte("encrypted:pkce-verifier"), fixedNow.Add(10*time.Minute))
	attempt.ID = 7
	repository := &repositoryStub{foundAttempt: attempt}
	oauthConnector := &oauthConnectorStub{credentials: paymentaccount.OAuthCredentials{
		ExternalAccountID:             "mp-juan",
		AccessToken:                   "access-token",
		RefreshToken:                  "refresh-token",
		ExpiresOn:                     fixedNow.Add(180 * 24 * time.Hour),
		CanReceiveMarketplacePayments: true,
	}}
	credentialProtector := &credentialProtectorStub{}
	service := paymentaccount.NewService(
		userFinderStub{found: providerUser},
		repository,
		repository,
		oauthConnector,
		credentialProtector,
		&secretGeneratorStub{},
		clockStub{},
	)

	account, err := service.CompleteAuthorization(context.Background(), "state-secret", "authorization-code")

	require.NoError(t, err)
	assert.Equal(t, "authorization-code", oauthConnector.exchangedCode)
	assert.Equal(t, "pkce-verifier", oauthConnector.exchangedVerifier)
	assert.Equal(t, [][]byte{[]byte("encrypted:pkce-verifier")}, credentialProtector.decryptedValues)
	assert.Equal(t, []string{"access-token", "refresh-token"}, credentialProtector.encryptedPlaintexts)
	require.NotNil(t, repository.savedAccount)
	assert.Equal(t, providerID, repository.savedAccount.ProviderID())
	assert.Equal(t, paymentaccount.PaymentProvider("mercado_pago"), repository.savedAccount.PaymentProvider())
	assert.Equal(t, "mp-juan", repository.savedAccount.ExternalAccountID())
	assert.Equal(t, []byte("encrypted:access-token"), repository.savedAccount.AccessTokenCiphertext())
	assert.Equal(t, []byte("encrypted:refresh-token"), repository.savedAccount.RefreshTokenCiphertext())
	assert.Equal(t, paymentaccount.StatusConnected, account.Status())
	assert.True(t, account.CanReceivePayments())
	assert.True(t, account.CanSendServiceProposals())
}

func TestCompleteAuthorizationPreservesExternalAccountAlreadyLinkedError(t *testing.T) {
	attempt := paymentaccount.NewAuthorizationAttempt(providerID, paymentaccount.PaymentProvider("mercado_pago"), []byte("stored-state-digest"), []byte("encrypted:pkce-verifier"), fixedNow.Add(10*time.Minute))
	repository := &repositoryStub{
		foundAttempt:   attempt,
		saveAccountErr: paymentaccount.ErrExternalAccountAlreadyLinked,
	}
	oauthConnector := &oauthConnectorStub{credentials: paymentaccount.OAuthCredentials{
		ExternalAccountID:             "mp-already-linked",
		AccessToken:                   "access-token",
		ExpiresOn:                     fixedNow.Add(180 * 24 * time.Hour),
		CanReceiveMarketplacePayments: true,
	}}
	service := paymentaccount.NewService(
		userFinderStub{}, repository, repository, oauthConnector,
		&credentialProtectorStub{}, &secretGeneratorStub{}, clockStub{},
	)

	account, err := service.CompleteAuthorization(context.Background(), "state-secret", "authorization-code")

	require.ErrorIs(t, err, paymentaccount.ErrExternalAccountAlreadyLinked)
	assert.Nil(t, account)
}

func TestCompleteAuthorizationDoesNotProtectInvalidConnectorCredentials(t *testing.T) {
	providerUser := registeredProvider(t)
	attempt := paymentaccount.NewAuthorizationAttempt(providerID, paymentaccount.PaymentProvider("mercado_pago"), []byte("stored-state-digest"), []byte("encrypted:pkce-verifier"), fixedNow.Add(10*time.Minute))
	repository := &repositoryStub{foundAttempt: attempt}
	credentialProtector := &credentialProtectorStub{}
	service := paymentaccount.NewService(
		userFinderStub{found: providerUser},
		repository,
		repository,
		&oauthConnectorStub{credentials: paymentaccount.OAuthCredentials{
			ExternalAccountID:             "mp-juan",
			CanReceiveMarketplacePayments: true,
		}},
		credentialProtector,
		&secretGeneratorStub{},
		clockStub{},
	)

	_, err := service.CompleteAuthorization(context.Background(), "state-secret", "authorization-code")

	require.ErrorIs(t, err, paymentaccount.ErrAccessTokenRequired)
	assert.Empty(t, credentialProtector.encryptedPlaintexts)
	assert.Nil(t, repository.savedAccount)
}

func TestCompleteAuthorizationRejectsAttemptFromDifferentPaymentProvider(t *testing.T) {
	providerUser := registeredProvider(t)
	attempt := paymentaccount.NewAuthorizationAttempt(providerID, paymentaccount.PaymentProvider("uala"), []byte("stored-state-digest"), []byte("encrypted:pkce-verifier"), fixedNow.Add(10*time.Minute))
	repository := &repositoryStub{foundAttempt: attempt}
	credentialProtector := &credentialProtectorStub{}
	oauthConnector := &oauthConnectorStub{provider: paymentaccount.PaymentProvider("mercado_pago")}
	service := paymentaccount.NewService(
		userFinderStub{found: providerUser},
		repository,
		repository,
		oauthConnector,
		credentialProtector,
		&secretGeneratorStub{},
		clockStub{},
	)

	_, err := service.CompleteAuthorization(context.Background(), "state-secret", "authorization-code")

	require.ErrorIs(t, err, paymentaccount.ErrPaymentProviderMismatch)
	assert.Empty(t, credentialProtector.decryptedValues)
	assert.Empty(t, oauthConnector.exchangedCode)
}

func TestGetConnectionReturnsAuthenticatedProviderConnection(t *testing.T) {
	providerUser := registeredProvider(t)
	account, err := paymentaccount.NewPaymentAccount(providerID, paymentaccount.PaymentProvider("mercado_pago"), "mp-juan", []byte("encrypted-access-token"), nil, fixedNow.Add(time.Hour), true)
	require.NoError(t, err)
	repository := &repositoryStub{foundAccount: account}
	service := paymentaccount.NewService(
		userFinderStub{found: providerUser},
		repository,
		repository,
		&oauthConnectorStub{},
		&credentialProtectorStub{},
		&secretGeneratorStub{},
		clockStub{},
	)

	found, err := service.GetConnection(context.Background(), providerAuthID)

	require.NoError(t, err)
	assert.Same(t, account, found)
}

func registeredProvider(t *testing.T) *provider.Provider {
	t.Helper()
	providerUser, err := provider.NewProvider(
		providerAuthID,
		"juan.plomero@example.com",
		"Juan",
		"Gómez",
		&category.Category{ID: 1, Name: "Plomería"},
		nil,
	)
	require.NoError(t, err)
	providerUser.SetPersistenceID(providerID)
	return providerUser
}
