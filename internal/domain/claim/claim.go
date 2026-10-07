package claim

import (
	"slices"
	"time"

	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

type Party string

const (
	PartyConsumer Party = "consumer"
	PartyProvider Party = "provider"
)

type Status string

const (
	StatusOpen      Status = "open"
	StatusInReview  Status = "in_review"
	StatusResolved  Status = "resolved"
	StatusDismissed Status = "dismissed"
)

func (s Status) Valid() bool {
	return s == StatusOpen || s == StatusInReview || s == StatusResolved || s == StatusDismissed
}

type ResolutionType string

const (
	ResolutionTypeConsumerFavor ResolutionType = "consumer_favor"
	ResolutionTypeProviderFavor ResolutionType = "provider_favor"
	ResolutionTypeAgreement     ResolutionType = "agreement"
	ResolutionTypeWithoutMerit  ResolutionType = "without_merit"
)

type SuggestedCompensation struct {
	AmountMinor int64
	Currency    string
	Unit        string
}
type Resolution struct {
	Type                  ResolutionType
	Reasoning             string
	ResolvedOn            time.Time
	SuggestedCompensation *SuggestedCompensation
}
type Action struct {
	ID         int
	Type       string
	ActorID    int
	ActorParty Party
	CreatedOn  time.Time
}
type Claimant struct {
	ID    int
	Party Party
}
type Claim struct {
	ID                    int
	ClaimantID            int
	ClaimantParty         Party
	OperationID           operationmodel.ID
	Reference             Reference
	Reason                Reason
	Description           string
	Status                Status
	ImageFileIDs          []string
	CreatedOn             time.Time
	ReviewStartedOn       *time.Time
	ClosedOn              *time.Time
	Resolution            *Resolution
	Actions               []Action
	submissionKey         string
	submissionFingerprint string
}

func New(claimant Claimant, operationID operationmodel.ID, key string, input Submission, now time.Time) (*Claim, error) {
	if claimant.ID <= 0 || (claimant.Party != PartyConsumer && claimant.Party != PartyProvider) {
		return nil, ErrForbidden
	}
	if (operationID.Kind != operationmodel.KindJobRequest && operationID.Kind != operationmodel.KindServiceProposal) || operationID.ResourceID <= 0 || now.IsZero() {
		return nil, ErrInvalidSubmission
	}
	now = now.UTC().Truncate(time.Microsecond)
	normalized, err := input.Normalize()
	if err != nil {
		return nil, err
	}
	key, err = NormalizeIdempotencyKey(key)
	if err != nil {
		return nil, err
	}
	return &Claim{ClaimantID: claimant.ID, ClaimantParty: claimant.Party, OperationID: operationID, Reference: normalized.Reference, Reason: normalized.Reason, Description: normalized.Description, Status: StatusOpen, ImageFileIDs: slices.Clone(normalized.ImageFileIDs), CreatedOn: now, Actions: []Action{{Type: "submitted", ActorID: claimant.ID, ActorParty: claimant.Party, CreatedOn: now}}, submissionKey: key, submissionFingerprint: normalized.Fingerprint()}, nil
}
func Rehydrate(found Claim, key, fingerprint string) *Claim {
	found.submissionKey = key
	found.submissionFingerprint = fingerprint
	return &found
}
func (c *Claim) SubmissionKey() string         { return c.submissionKey }
func (c *Claim) SubmissionFingerprint() string { return c.submissionFingerprint }
func (c *Claim) MatchesSubmission(input Submission) bool {
	return c.submissionFingerprint == input.Fingerprint()
}
