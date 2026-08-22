package repositories_test

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type googleCalendarRepositoryTestContext struct {
	database        *sql.DB
	attemptStore    *repositories.GoogleCalendarAuthorizationAttemptRepository
	connectionStore *repositories.GoogleCalendarConnectionRepository
	userID          int
}

func newGoogleCalendarRepositoryTest(t *testing.T) googleCalendarRepositoryTestContext {
	t.Helper()
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err)

	attemptStore := repositories.NewGoogleCalendarAuthorizationAttemptRepository(database)
	connectionStore := repositories.NewGoogleCalendarConnectionRepository(database, attemptStore)
	context := googleCalendarRepositoryTestContext{
		database:        database,
		attemptStore:    attemptStore,
		connectionStore: connectionStore,
	}
	context.userID = saveCalendarConnectionTestConsumer(t, database)
	t.Cleanup(func() {
		cleanGoogleCalendarRepositoryTestDatabase(t, context)
		database.Close()
	})
	return context
}

func saveCalendarConnectionTestConsumer(t *testing.T, database *sql.DB) int {
	t.Helper()
	userRepository := repositories.NewUserRepository(database)
	consumerToSave, err := consumer.NewConsumer(
		"auth0|calendar-connection-repository-test",
		"calendar.connection.repository@example.com",
		"Ana",
		"Perez",
		nil,
	)
	require.NoError(t, err)
	_, err = userRepository.Save(context.Background(), consumerToSave)
	require.NoError(t, err)
	userID, err := userRepository.FindIDByEmail(consumerToSave.Email())
	require.NoError(t, err)
	return userID
}

func cleanGoogleCalendarRepositoryTestDatabase(t *testing.T, testContext googleCalendarRepositoryTestContext) {
	t.Helper()
	_, err := testContext.database.Exec("DELETE FROM google_calendar_connections")
	require.NoError(t, err)
	_, err = testContext.database.Exec("DELETE FROM google_calendar_authorization_attempts")
	require.NoError(t, err)
	_, err = testContext.database.Exec("DELETE FROM users WHERE id = $1", testContext.userID)
	require.NoError(t, err)
}

func calendarAuthorizationAttempt(t *testing.T, userID int, stateByte byte) *calendarconnection.AuthorizationAttempt {
	t.Helper()
	attempt, err := calendarconnection.NewAuthorizationAttempt(
		userID,
		bytes.Repeat([]byte{stateByte}, 32),
		[]byte("opaque-pkce-verifier-ciphertext"),
		time.Now().UTC().Add(10*time.Minute).Truncate(time.Microsecond),
	)
	require.NoError(t, err)
	return attempt
}

func calendarConnection(t *testing.T, userID int, refreshToken string) *calendarconnection.Connection {
	t.Helper()
	connection, err := calendarconnection.NewConnection(
		userID,
		"primary",
		[]byte(refreshToken),
		time.Now().UTC().Truncate(time.Microsecond),
	)
	require.NoError(t, err)
	return connection
}

func TestGoogleCalendarAuthorizationAttemptRepositoryPersistsOpaqueAttempt(t *testing.T) {
	testContext := newGoogleCalendarRepositoryTest(t)
	attempt := calendarAuthorizationAttempt(t, testContext.userID, 1)

	err := testContext.attemptStore.Save(context.Background(), attempt)

	require.NoError(t, err)
	assert.Positive(t, attempt.ID)
	found, err := testContext.attemptStore.FindByStateDigest(context.Background(), attempt.StateDigest)
	require.NoError(t, err)
	assert.Equal(t, attempt.ID, found.ID)
	assert.Equal(t, testContext.userID, found.UserID)
	assert.Equal(t, attempt.StateDigest, found.StateDigest)
	assert.Equal(t, attempt.CodeVerifierCiphertext, found.CodeVerifierCiphertext)
	assert.Equal(t, attempt.ExpiresOn, found.ExpiresOn)
	assert.Nil(t, found.ConsumedOn)
}

func TestGoogleCalendarConnectionRepositoryCompletesAuthorizationAtomically(t *testing.T) {
	testContext := newGoogleCalendarRepositoryTest(t)
	attempt := calendarAuthorizationAttempt(t, testContext.userID, 2)
	require.NoError(t, testContext.attemptStore.Save(context.Background(), attempt))
	connection := calendarConnection(t, testContext.userID, "opaque-refresh-token-ciphertext")

	err := testContext.connectionStore.SaveFromAuthorization(context.Background(), attempt.ID, connection)

	require.NoError(t, err)
	found, err := testContext.connectionStore.FindByUserID(context.Background(), testContext.userID)
	require.NoError(t, err)
	assert.Equal(t, connection.CalendarID(), found.CalendarID())
	assert.Equal(t, connection.RefreshTokenCiphertext(), found.RefreshTokenCiphertext())
	assert.Equal(t, connection.Status(), found.Status())
	foundAttempt, err := testContext.attemptStore.FindByStateDigest(context.Background(), attempt.StateDigest)
	require.NoError(t, err)
	assert.NotNil(t, foundAttempt.ConsumedOn)
}

func TestGoogleCalendarConnectionRepositoryUpsertsOneConnectionPerUser(t *testing.T) {
	testContext := newGoogleCalendarRepositoryTest(t)
	firstAttempt := calendarAuthorizationAttempt(t, testContext.userID, 3)
	require.NoError(t, testContext.attemptStore.Save(context.Background(), firstAttempt))
	require.NoError(t, testContext.connectionStore.SaveFromAuthorization(
		context.Background(),
		firstAttempt.ID,
		calendarConnection(t, testContext.userID, "first-refresh-token"),
	))

	secondAttempt := calendarAuthorizationAttempt(t, testContext.userID, 4)
	require.NoError(t, testContext.attemptStore.Save(context.Background(), secondAttempt))
	secondConnection := calendarConnection(t, testContext.userID, "second-refresh-token")
	require.NoError(t, testContext.connectionStore.SaveFromAuthorization(
		context.Background(), secondAttempt.ID, secondConnection,
	))

	found, err := testContext.connectionStore.FindByUserID(context.Background(), testContext.userID)
	require.NoError(t, err)
	assert.Equal(t, []byte("second-refresh-token"), found.RefreshTokenCiphertext())
	var count int
	require.NoError(t, testContext.database.QueryRow(
		"SELECT COUNT(*) FROM google_calendar_connections WHERE user_id = $1",
		testContext.userID,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestGoogleCalendarConnectionRepositoryRollsBackWhenAttemptCannotBeConsumed(t *testing.T) {
	testContext := newGoogleCalendarRepositoryTest(t)
	connection := calendarConnection(t, testContext.userID, "opaque-refresh-token-ciphertext")

	err := testContext.connectionStore.SaveFromAuthorization(context.Background(), 999999, connection)

	assert.ErrorIs(t, err, calendarconnection.ErrAuthorizationAttemptNotFound)
	_, err = testContext.connectionStore.FindByUserID(context.Background(), testContext.userID)
	assert.ErrorIs(t, err, calendarconnection.ErrConnectionNotFound)
}

func TestGoogleCalendarAuthorizationAttemptRepositoryConsumesAttemptOnce(t *testing.T) {
	testContext := newGoogleCalendarRepositoryTest(t)
	attempt := calendarAuthorizationAttempt(t, testContext.userID, 5)
	require.NoError(t, testContext.attemptStore.Save(context.Background(), attempt))

	require.NoError(t, testContext.attemptStore.Consume(context.Background(), attempt))
	err := testContext.attemptStore.Consume(context.Background(), attempt)

	assert.ErrorIs(t, err, calendarconnection.ErrAuthorizationAttemptConsumed)
	found, err := testContext.attemptStore.FindByStateDigest(context.Background(), attempt.StateDigest)
	require.NoError(t, err)
	assert.NotNil(t, found.ConsumedOn)
}
