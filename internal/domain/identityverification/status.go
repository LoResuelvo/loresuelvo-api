package identityverification

import "strings"

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

var validStatuses = map[VerificationStatus]struct{}{
	StatusNotStarted: {}, StatusInProgress: {}, StatusAwaitingUser: {},
	StatusInReview: {}, StatusApproved: {}, StatusDeclined: {},
	StatusResubmitted: {}, StatusAbandoned: {}, StatusExpired: {}, StatusKYCExpired: {},
}

func (status VerificationStatus) Valid() bool {
	_, ok := validStatuses[status]
	return ok
}

func (status VerificationStatus) IsReusable() bool {
	return status == StatusNotStarted || status == StatusInProgress ||
		status == StatusAwaitingUser || status == StatusResubmitted
}

func (status VerificationStatus) BlocksNewSession() bool {
	return status == StatusApproved || status == StatusInReview
}

func ParseStatus(value string) (VerificationStatus, bool) {
	status := VerificationStatus(strings.ToLower(strings.TrimSpace(value)))
	return status, status.Valid()
}
