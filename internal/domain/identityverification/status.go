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

func (status VerificationStatus) CanApplyResult() bool {
	switch status {
	case StatusInProgress, StatusAwaitingUser, StatusInReview, StatusApproved,
		StatusDeclined, StatusResubmitted, StatusAbandoned, StatusExpired, StatusKYCExpired:
		return true
	default:
		return false
	}
}
