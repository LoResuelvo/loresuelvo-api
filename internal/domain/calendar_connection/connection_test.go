package calendarconnection_test

import (
	"testing"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionOwnsRefreshTokenCiphertextAndCalendarIdentity(t *testing.T) {
	ciphertext := []byte("encrypted-refresh-token")
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)

	connection, err := calendarconnection.NewConnection(42, "calendar-42", ciphertext, now)

	require.NoError(t, err)
	ciphertext[0] = 'x'
	returnedCiphertext := connection.RefreshTokenCiphertext()
	returnedCiphertext[0] = 'y'
	assert.Equal(t, 42, connection.UserID())
	assert.Equal(t, []byte("encrypted-refresh-token"), connection.RefreshTokenCiphertext())
	assert.Equal(t, "calendar-42", connection.CalendarID())
	assert.Equal(t, calendarconnection.StatusConnected, connection.Status())
	assert.True(t, connection.IsConnected())
	assert.Equal(t, now, connection.ConnectedOn())
	assert.Equal(t, now, connection.UpdatedOn())
}

func TestRehydrateConnectionRequiresCalendarIdentity(t *testing.T) {
	connection, err := calendarconnection.RehydrateConnection(
		42,
		[]byte("encrypted-refresh-token"),
		" ",
		calendarconnection.StatusConnected,
		time.Now(),
		time.Now(),
	)

	assert.Nil(t, connection)
	assert.ErrorIs(t, err, calendarconnection.ErrCalendarIDRequired)
}

func TestConnectionCanRequireAttention(t *testing.T) {
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	connection, err := calendarconnection.NewConnection(42, "calendar-42", []byte("encrypted-refresh-token"), now)
	require.NoError(t, err)

	updated := now.Add(time.Hour)
	connection.MarkActionRequired(updated)

	assert.Equal(t, calendarconnection.StatusActionRequired, connection.Status())
	assert.False(t, connection.IsConnected())
	assert.Equal(t, updated, connection.UpdatedOn())
}
