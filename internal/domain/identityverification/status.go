package identityverification

type VerificationStatus string

const (
	StatusNotStarted VerificationStatus = "not_started"
	StatusInProgress VerificationStatus = "in_progress"
	StatusApproved   VerificationStatus = "approved"
)
