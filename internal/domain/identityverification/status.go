package identityverification

type VerificationStatus string

const (
	StatusUnverified   VerificationStatus = "unverified"
	StatusNotStarted   VerificationStatus = "not_started"
	StatusInProgress   VerificationStatus = "in_progress"
	StatusAwaitingUser VerificationStatus = "awaiting_user"
	StatusInReview     VerificationStatus = "in_review"
	StatusApproved     VerificationStatus = "approved"
	StatusDeclined     VerificationStatus = "declined"
	StatusResubmitted  VerificationStatus = "resubmitted"
	StatusAbandoned    VerificationStatus = "abandoned"
	StatusExpired      VerificationStatus = "expired"
	StatusKYCExpired   VerificationStatus = "kyc_expired"
)

// IsValid reports whether the status is one of the application's known
// verification states, including the absence of a verification session.
func (status VerificationStatus) IsValid() bool {
	switch status {
	case StatusUnverified, StatusNotStarted, StatusInProgress, StatusAwaitingUser,
		StatusInReview, StatusApproved, StatusDeclined, StatusResubmitted,
		StatusAbandoned, StatusExpired, StatusKYCExpired:
		return true
	default:
		return false
	}
}

func (status VerificationStatus) CanApplyResult() bool {
	switch status {
	case StatusInProgress, StatusAwaitingUser, StatusInReview, StatusApproved,
		StatusDeclined, StatusResubmitted, StatusAbandoned, StatusExpired, StatusKYCExpired:
		return true
	default:
		return false
	}
}
