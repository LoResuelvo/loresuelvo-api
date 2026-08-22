package googlecalendar

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFakeOAuthClientExchangesServerAuthorizationCode(t *testing.T) {
	credentials, err := NewFakeOAuthClient().ExchangeAuthorizationCode(context.Background(), "android-calendar-code", "")

	require.NoError(t, err)
	require.Equal(t, primaryCalendarID, credentials.CalendarID)
	require.Equal(t, "fake-refresh-token-for-android", credentials.RefreshToken)
}
