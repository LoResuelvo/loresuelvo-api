package identityverification

type VerificationStatus string

const (
	StatusUnverified VerificationStatus = "unverified"
	StatusNotStarted VerificationStatus = "not_started"
	StatusInProgress VerificationStatus = "in_progress"
	StatusApproved   VerificationStatus = "approved"
)
