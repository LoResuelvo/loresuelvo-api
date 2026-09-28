package readmodel_test

import (
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestDiagnosticChecksDistinguishAbsentEvidence(t *testing.T) {
	d := readmodel.ProviderDiagnostic{}
	checks := d.Checks(time.Now())
	require.Len(t, checks, 4)
	require.Equal(t, "identity_no_session", checks[0].ReasonCode)
	require.Equal(t, "warning", checks[1].Result)
	require.Equal(t, "unknown", checks[2].Result)
	require.Nil(t, checks[2].EvidenceOn)
	require.Equal(t, "calendar_not_connected", checks[3].ReasonCode)
}

func TestDiagnosticChecksTreatExpiryBoundaryAsElapsed(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	d := readmodel.ProviderDiagnostic{Payment: readmodel.PaymentEvidence{ConnectedOn: &now, TokenExpiresOn: &now}}
	require.Equal(t, "pass", d.Checks(now)[1].Result)
	require.Equal(t, "payment_token_expiry_elapsed", d.Checks(now)[2].ReasonCode)
	future := now.Add(time.Second)
	d.Payment.TokenExpiresOn = &future
	require.Equal(t, "pass", d.Checks(now)[2].Result)
}

func TestDiagnosticChecksUseOnlyPersistedIdentityDates(t *testing.T) {
	now := time.Now().UTC()
	for _, status := range []identityverification.VerificationStatus{identityverification.StatusUnverified, identityverification.StatusNotStarted, identityverification.StatusInProgress, identityverification.StatusAwaitingUser, identityverification.StatusInReview, identityverification.StatusApproved, identityverification.StatusDeclined, identityverification.StatusResubmitted, identityverification.StatusAbandoned, identityverification.StatusExpired, identityverification.StatusKYCExpired} {
		t.Run(string(status), func(t *testing.T) {
			d := readmodel.ProviderDiagnostic{Provider: readmodel.Provider{IdentityVerificationStatus: status}}
			check := d.Checks(now)[0]
			require.Nil(t, check.EvidenceOn)
			if status == identityverification.StatusApproved {
				require.Equal(t, "pass", check.Result)
				d.Provider.IdentityVerifiedOn = &now
				require.Equal(t, &now, d.Checks(now)[0].EvidenceOn)
			} else {
				require.Equal(t, "warning", check.Result)
			}
		})
	}
	zero := time.Time{}
	d := readmodel.ProviderDiagnostic{Payment: readmodel.PaymentEvidence{TokenExpiresOn: &zero}}
	require.Equal(t, "unknown", d.Checks(now)[2].Result)
	require.Nil(t, d.Checks(now)[2].EvidenceOn)
}
