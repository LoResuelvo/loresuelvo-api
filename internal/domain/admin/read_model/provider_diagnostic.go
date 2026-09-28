package readmodel

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

// ProviderDiagnostic contains only persisted evidence safe for administrative access.
type ProviderDiagnostic struct {
	Activity         []ProviderActivity
	Reviews          []ProviderReview
	OrderSync        []ProviderOrderSync
	RatingStats      provider.RatingStats
	DiagnosticChecks []DiagnosticCheck
	Provider         Provider
	IdentityResultOn *time.Time
	Payment          PaymentEvidence
	Calendar         CalendarEvidence
}
type PaymentEvidence struct{ ConnectedOn, TokenExpiresOn *time.Time }

func (e PaymentEvidence) State() string {
	if e.ConnectedOn != nil {
		return "connected"
	}
	return "disconnected"
}

type CalendarEvidence struct {
	Status                 string
	ConnectedOn, UpdatedOn *time.Time
}

func (e CalendarEvidence) State() string {
	if e.Status == "" {
		return "disconnected"
	}
	return e.Status
}

type DiagnosticCheck struct {
	Control, Result, ReasonCode string
	EvidenceOn                  *time.Time
}

func (d ProviderDiagnostic) Checks(now time.Time) []DiagnosticCheck {
	return []DiagnosticCheck{d.identityCheck(), d.Payment.connectionCheck(), d.Payment.expiryCheck(now), d.Calendar.connectionCheck()}
}
func (d ProviderDiagnostic) identityCheck() DiagnosticCheck {
	status := d.Provider.IdentityVerificationStatus
	if status == "" || status == identityverification.StatusUnverified {
		return DiagnosticCheck{"identity_verification", "warning", "identity_no_session", nil}
	}
	if status == identityverification.StatusApproved {
		return DiagnosticCheck{"identity_verification", "pass", "identity_approved", d.Provider.IdentityVerifiedOn}
	}
	return DiagnosticCheck{"identity_verification", "warning", "identity_" + string(status), d.IdentityResultOn}
}
func (e PaymentEvidence) connectionCheck() DiagnosticCheck {
	if e.ConnectedOn == nil {
		return DiagnosticCheck{"payment_account_connection", "warning", "payment_account_not_connected", nil}
	}
	return DiagnosticCheck{"payment_account_connection", "pass", "payment_account_connected", e.ConnectedOn}
}
func (e PaymentEvidence) expiryCheck(now time.Time) DiagnosticCheck {
	if e.TokenExpiresOn == nil || e.TokenExpiresOn.IsZero() {
		return DiagnosticCheck{"payment_token_expiry", "unknown", "payment_token_expiry_unavailable", nil}
	}
	if !e.TokenExpiresOn.After(now) {
		return DiagnosticCheck{"payment_token_expiry", "warning", "payment_token_expiry_elapsed", e.TokenExpiresOn}
	}
	return DiagnosticCheck{"payment_token_expiry", "pass", "payment_token_expiry_future", e.TokenExpiresOn}
}
func (e CalendarEvidence) connectionCheck() DiagnosticCheck {
	if e.Status == "connected" {
		return DiagnosticCheck{"calendar_connection", "pass", "calendar_connected", e.ConnectedOn}
	}
	if e.Status == "action_required" {
		return DiagnosticCheck{"calendar_connection", "warning", "calendar_authorization_required", e.UpdatedOn}
	}
	return DiagnosticCheck{"calendar_connection", "warning", "calendar_not_connected", nil}
}

// RecordIdentityEvidence keeps verified identity dates meaningful for the current session state.
func (d *ProviderDiagnostic) RecordIdentityEvidence(status identityverification.VerificationStatus, verifiedOn, resultOn *time.Time) {
	d.Provider.IdentityVerificationStatus = status
	d.Provider.IdentityVerifiedOn = nil
	if status == identityverification.StatusApproved {
		d.Provider.IdentityVerifiedOn = verifiedOn
	}
	d.IdentityResultOn = resultOn
}
