package installation

import (
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRegistrationValidatesBoundaryFields(t *testing.T) {
	for _, field := range []string{"id", "secret", "binding", "previous_binding", "token", "locale", "app", "uuid_version", "uuid_variant"} {
		t.Run(field, func(t *testing.T) {
			r := validRegistration()
			switch field {
			case "id":
				r.ID = "not-uuid"
			case "secret":
				r.Secret = ""
			case "binding":
				r.BindingID = ""
			case "previous_binding":
				r.PreviousBindingID = "old"
			case "token":
				r.Token = " \n"
			case "locale":
				r.Locale = "fr"
			case "app":
				r.App = "admin"
			case "uuid_version":
				r.ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
			case "uuid_variant":
				r.ID = "00000000-0000-4000-0000-000000000001"
			}
			_, err := NewRegistered(10, "consumer", r)
			require.ErrorIs(t, err, ErrInvalidInstallation)
		})
	}
}
func TestRegistrationHashesPossessionProofAndDefaultsSpanish(t *testing.T) {
	r := validRegistration()
	i, err := NewRegistered(10, "consumer", r)
	require.NoError(t, err)
	hash := sha256.Sum256([]byte(r.Secret))
	require.Equal(t, hash[:], i.SecretHash)
	require.Equal(t, "es", i.Locale)
}
func TestRegistrationRequiresCompatibleAppRole(t *testing.T) {
	r := validRegistration()
	_, err := NewRegistered(10, "provider", r)
	require.ErrorIs(t, err, ErrForbidden)
}
func TestRenewalKeepsBindingAndUpdatesTokenLanguage(t *testing.T) {
	r := validRegistration()
	i, err := NewRegistered(10, "consumer", r)
	require.NoError(t, err)
	r.Token = "renewed-token"
	r.Locale = "en"
	require.NoError(t, i.Register(10, "consumer", r))
	require.Equal(t, r.BindingID, i.BindingID)
	require.Equal(t, "renewed-token", i.Token)
	require.Equal(t, "en", i.Locale)
}
func TestRegistrationRejectsIDOnlyTakeover(t *testing.T) {
	r := validRegistration()
	i, err := NewRegistered(10, "consumer", r)
	require.NoError(t, err)
	r.Secret = uuid.NewString()
	r.PreviousBindingID = r.BindingID
	r.BindingID = uuid.NewString()
	require.ErrorIs(t, i.Register(20, "consumer", r), ErrForbidden)
	require.Equal(t, 10, i.UserID)
}
func TestAccountSwitchRejectsOldQueuedRegistrationAndRemoval(t *testing.T) {
	old := validRegistration()
	i, err := NewRegistered(10, "consumer", old)
	require.NoError(t, err)
	next := old
	next.PreviousBindingID = old.BindingID
	next.BindingID = uuid.NewString()
	require.NoError(t, i.Register(20, "consumer", next))
	require.NoError(t, i.Register(20, "consumer", next))
	require.ErrorIs(t, i.Register(10, "consumer", old), ErrConflict)
	require.ErrorIs(t, i.Unregister(10, old.Secret, old.BindingID), ErrForbidden)
	require.ErrorIs(t, i.Unregister(20, old.Secret, old.BindingID), ErrConflict)
	require.Equal(t, 20, i.UserID)
	require.True(t, i.Enabled)
	require.Equal(t, next.BindingID, i.BindingID)
}
func TestNewBindingRequiresCurrentPreviousBinding(t *testing.T) {
	r := validRegistration()
	i, err := NewRegistered(10, "consumer", r)
	require.NoError(t, err)
	r.BindingID = uuid.NewString()
	require.ErrorIs(t, i.Register(20, "consumer", r), ErrConflict)
}
func TestUnregisterIsIdempotentAndRejectsLateSameBindingRenewal(t *testing.T) {
	r := validRegistration()
	i, err := NewRegistered(10, "consumer", r)
	require.NoError(t, err)
	require.NoError(t, i.Unregister(10, r.Secret, r.BindingID))
	require.NoError(t, i.Unregister(10, r.Secret, r.BindingID))
	require.False(t, i.Enabled)
	require.ErrorIs(t, i.Register(10, "consumer", r), ErrConflict)
}

func TestInvalidTokenCanRenewInSameLiveBinding(t *testing.T) {
	r := validRegistration()
	i, err := NewRegistered(10, "consumer", r)
	require.NoError(t, err)
	i.Invalidate()
	require.False(t, i.Enabled)
	require.False(t, i.Revoked)
	require.ErrorIs(t, i.Register(10, "consumer", r), ErrConflict)
	r.Token = "fresh-token"
	require.NoError(t, i.Register(10, "consumer", r))
	require.True(t, i.Enabled)
	require.Equal(t, r.BindingID, i.BindingID)
}
func TestRevokedBindingCannotRenewEvenWithFreshToken(t *testing.T) {
	r := validRegistration()
	i, err := NewRegistered(10, "consumer", r)
	require.NoError(t, err)
	require.NoError(t, i.Unregister(10, r.Secret, r.BindingID))
	r.Token = "fresh-token"
	require.ErrorIs(t, i.Register(10, "consumer", r), ErrConflict)
	require.True(t, i.Revoked)
	require.False(t, i.Enabled)
}
