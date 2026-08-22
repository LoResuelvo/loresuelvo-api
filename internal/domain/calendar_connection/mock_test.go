package calendarconnection_test

import (
	"context"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type userRepositoryStub struct {
	user user.User
	err  error
}

func (stub *userRepositoryStub) FindByAuthID(string) (user.User, error) {
	return stub.user, stub.err
}

type secretGeneratorStub struct {
	values []string
}

func (stub *secretGeneratorStub) Generate() (string, error) {
	value := stub.values[0]
	stub.values = stub.values[1:]
	return value, nil
}

type clockStub struct {
	now time.Time
}

func (stub clockStub) Now() time.Time { return stub.now }

type credentialProtectorStub struct {
	encryptedPlaintexts []string
	decryptedPlaintext  string
	decryptErr          error
}

func (stub *credentialProtectorStub) Encrypt(plaintext string) ([]byte, error) {
	stub.encryptedPlaintexts = append(stub.encryptedPlaintexts, plaintext)
	return []byte("encrypted:" + plaintext), nil
}

func (stub *credentialProtectorStub) Decrypt([]byte) (string, error) {
	return stub.decryptedPlaintext, stub.decryptErr
}

type oauthConnectorStub struct {
	authorizationURL string
	credentials      calendarconnection.AuthorizationCredentials
	exchangeErr      error
	verifier         string
	code             string
}

func (stub *oauthConnectorStub) AuthorizationURL(state, verifier string) (string, error) {
	stub.verifier = verifier
	return stub.authorizationURL + "?state=" + state, nil
}

func (stub *oauthConnectorStub) ExchangeAuthorizationCode(_ context.Context, code, verifier string) (calendarconnection.AuthorizationCredentials, error) {
	stub.code = code
	stub.verifier = verifier
	return stub.credentials, stub.exchangeErr
}

type connectionRepositoryStub struct {
	savedAttemptID  int
	savedConnection *calendarconnection.Connection
	foundConnection *calendarconnection.Connection
	foundErr        error
}

func (stub *connectionRepositoryStub) SaveFromAuthorization(_ context.Context, attemptID int, connection *calendarconnection.Connection) error {
	stub.savedAttemptID = attemptID
	stub.savedConnection = connection
	return nil
}

func (stub *connectionRepositoryStub) FindByUserID(_ context.Context, _ int) (*calendarconnection.Connection, error) {
	return stub.foundConnection, stub.foundErr
}

type authorizationAttemptRepositoryStub struct {
	savedAttempt    *calendarconnection.AuthorizationAttempt
	foundAttempt    *calendarconnection.AuthorizationAttempt
	consumedAttempt *calendarconnection.AuthorizationAttempt
}

func (stub *authorizationAttemptRepositoryStub) Save(_ context.Context, attempt *calendarconnection.AuthorizationAttempt) error {
	attempt.ID = 17
	stub.savedAttempt = attempt
	return nil
}

func (stub *authorizationAttemptRepositoryStub) FindByStateDigest(_ context.Context, _ []byte) (*calendarconnection.AuthorizationAttempt, error) {
	return stub.foundAttempt, nil
}

func (stub *authorizationAttemptRepositoryStub) Consume(_ context.Context, attempt *calendarconnection.AuthorizationAttempt) error {
	stub.consumedAttempt = attempt
	return nil
}
