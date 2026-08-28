package identityverification

type VerificationStatus string

const (
	StatusUnverified   VerificationStatus = "unverified"
	StatusNotStarted   VerificationStatus = "not_started"
	StatusInProgress   VerificationStatus = "in_progress"
	StatusAwaitingUser VerificationStatus = "awaiting_user"
	StatusInReview     VerificationStatus = "in_review"
	StatusApproved     VerificationStatus = "approved"
)

func (status VerificationStatus) CanApplyResult() bool {
	return status == StatusInProgress || status == StatusAwaitingUser || status == StatusInReview
}
