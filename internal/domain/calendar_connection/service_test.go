package calendarconnection_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const consumerAuthID = "auth0|consumer-calendar-connection-test"

var calendarConnectionNow = time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)

func registeredConsumer(t *testing.T) *consumer.Consumer {
	t.Helper()
	foundUser, err := consumer.NewConsumer(consumerAuthID, "ana@example.com", "Ana", "Pérez", nil)
	require.NoError(t, err)
	foundUser.SetPersistenceID(42)
	return foundUser
}

func TestStartAuthorizationCreatesConsumerBoundPKCEAttempt(t *testing.T) {
	attemptRepository := &authorizationAttemptRepositoryStub{}
	connector := &oauthConnectorStub{authorizationURL: "https://calendar-provider.test/authorization"}
	protector := &credentialProtectorStub{}
	service := calendarconnection.NewService(
		&userRepositoryStub{user: registeredConsumer(t)},
		attemptRepository,
		&connectionRepositoryStub{},
		connector,
		protector,
		&secretGeneratorStub{values: []string{"state-secret", "pkce-verifier"}},
		clockStub{now: calendarConnectionNow},
	)

	authorization, err := service.StartAuthorization(context.Background(), consumerAuthID)

	require.NoError(t, err)
	require.NotNil(t, attemptRepository.savedAttempt)
	assert.Equal(t, 42, attemptRepository.savedAttempt.UserID)
	assert.NotEqual(t, []byte("state-secret"), attemptRepository.savedAttempt.StateDigest)
	assert.Equal(t, []byte("encrypted:pkce-verifier"), attemptRepository.savedAttempt.CodeVerifierCiphertext)
	assert.Equal(t, calendarConnectionNow.Add(10*time.Minute), attemptRepository.savedAttempt.ExpiresOn)
	assert.Equal(t, "pkce-verifier", connector.verifier)
	assert.Equal(t, []string{"pkce-verifier"}, protector.encryptedPlaintexts)
	assert.Equal(t, "state-secret", authorization.State)
	assert.Contains(t, authorization.URL, "state=state-secret")
}

func TestCompleteAuthorizationPersistsOnlyEncryptedRefreshToken(t *testing.T) {
	state := "state-secret"
	digest := sha256.Sum256([]byte(state))
	attempt := calendarconnection.RehydrateAuthorizationAttempt(
		17,
		42,
		digest[:],
		[]byte("encrypted:pkce-verifier"),
		calendarConnectionNow.Add(10*time.Minute),
		nil,
	)
	attemptRepository := &authorizationAttemptRepositoryStub{foundAttempt: attempt}
	connectionRepository := &connectionRepositoryStub{}
	connector := &oauthConnectorStub{
		credentials: calendarconnection.AuthorizationCredentials{CalendarID: "calendar-42", RefreshToken: "calendar-refresh-token"},
	}
	protector := &credentialProtectorStub{decryptedPlaintext: "pkce-verifier"}
	service := calendarconnection.NewService(
		&userRepositoryStub{user: registeredConsumer(t)},
		attemptRepository,
		connectionRepository,
		connector,
		protector,
		&secretGeneratorStub{},
		clockStub{now: calendarConnectionNow},
	)

	connection, err := service.CompleteAuthorization(context.Background(), state, "authorization-code")

	require.NoError(t, err)
	require.NotNil(t, connectionRepository.savedConnection)
	assert.Equal(t, 17, connectionRepository.savedAttemptID)
	assert.Equal(t, "authorization-code", connector.code)
	assert.Equal(t, "pkce-verifier", connector.verifier)
	assert.Equal(t, []string{"calendar-refresh-token"}, protector.encryptedPlaintexts)
	assert.Equal(t, []byte("encrypted:calendar-refresh-token"), connection.RefreshTokenCiphertext())
	assert.Equal(t, "calendar-42", connection.CalendarID())
	assert.Equal(t, calendarconnection.StatusConnected, connection.Status())
}

func TestRejectAuthorizationConsumesActiveAttempt(t *testing.T) {
	state := "state-secret"
	digest := sha256.Sum256([]byte(state))
	attempt := calendarconnection.RehydrateAuthorizationAttempt(
		17,
		42,
		digest[:],
		[]byte("encrypted:pkce-verifier"),
		calendarConnectionNow.Add(10*time.Minute),
		nil,
	)
	attemptRepository := &authorizationAttemptRepositoryStub{foundAttempt: attempt}
	service := calendarconnection.NewService(
		&userRepositoryStub{user: registeredConsumer(t)},
		attemptRepository,
		&connectionRepositoryStub{},
		&oauthConnectorStub{},
		&credentialProtectorStub{},
		&secretGeneratorStub{},
		clockStub{now: calendarConnectionNow},
	)

	err := service.RejectAuthorization(context.Background(), state)

	require.NoError(t, err)
	assert.Same(t, attempt, attemptRepository.consumedAttempt)
}

func TestGetConnectionStatusReturnsDisconnectedWhenNoConnectionExists(t *testing.T) {
	service := calendarconnection.NewService(
		&userRepositoryStub{user: registeredConsumer(t)},
		&authorizationAttemptRepositoryStub{},
		&connectionRepositoryStub{foundErr: calendarconnection.ErrConnectionNotFound},
		&oauthConnectorStub{},
		&credentialProtectorStub{},
		&secretGeneratorStub{},
		clockStub{now: calendarConnectionNow},
	)

	status, err := service.GetConnectionStatus(context.Background(), consumerAuthID)

	require.NoError(t, err)
	assert.Equal(t, calendarconnection.StatusDisconnected, status)
}

func TestCompleteAuthorizationConsumesAttemptWhenRefreshTokenIsMissing(t *testing.T) {
	state := "state-secret"
	digest := sha256.Sum256([]byte(state))
	attempt := calendarconnection.RehydrateAuthorizationAttempt(
		17,
		42,
		digest[:],
		[]byte("encrypted:pkce-verifier"),
		calendarConnectionNow.Add(10*time.Minute),
		nil,
	)
	attemptRepository := &authorizationAttemptRepositoryStub{foundAttempt: attempt}
	service := calendarconnection.NewService(
		&userRepositoryStub{user: registeredConsumer(t)},
		attemptRepository,
		&connectionRepositoryStub{},
		&oauthConnectorStub{credentials: calendarconnection.AuthorizationCredentials{CalendarID: "calendar-42"}},
		&credentialProtectorStub{decryptedPlaintext: "pkce-verifier"},
		&secretGeneratorStub{},
		clockStub{now: calendarConnectionNow},
	)

	connection, err := service.CompleteAuthorization(context.Background(), state, "authorization-code")

	assert.Nil(t, connection)
	assert.ErrorIs(t, err, calendarconnection.ErrRefreshTokenRequired)
	assert.Same(t, attempt, attemptRepository.consumedAttempt)
}

func TestGetConnectionStatusPropagatesUnexpectedRepositoryError(t *testing.T) {
	expectedErr := errors.New("database unavailable")
	service := calendarconnection.NewService(
		&userRepositoryStub{user: registeredConsumer(t)},
		&authorizationAttemptRepositoryStub{},
		&connectionRepositoryStub{foundErr: expectedErr},
		&oauthConnectorStub{},
		&credentialProtectorStub{},
		&secretGeneratorStub{},
		clockStub{now: calendarConnectionNow},
	)

	_, err := service.GetConnectionStatus(context.Background(), consumerAuthID)

	assert.ErrorIs(t, err, expectedErr)
}
