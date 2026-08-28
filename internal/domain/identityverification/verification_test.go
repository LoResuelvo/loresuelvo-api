package identityverification

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewIdentityVerificationStartsNotStarted(t *testing.T) {
	verification, err := NewVerification(7, uuid.New(), uuid.New(), "fake", 1, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, StatusNotStarted, verification.Status)
}

func TestProviderVendorDataUsesInternalID(t *testing.T) {
	require.Equal(t, "42", ProviderVendorData(42))
}
