package claim

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type ReferenceKind string

const (
	ReferenceKindJobRequest      ReferenceKind = "job_request"
	ReferenceKindServiceProposal ReferenceKind = "service_proposal"
	ReferenceKindWorkOrder       ReferenceKind = "work_order"
	ReferenceKindPaymentIntent   ReferenceKind = "payment_intent"
)

type Reference struct {
	kind ReferenceKind
	id   string
}

func NewReference(kind ReferenceKind, id string) (Reference, error) {
	switch kind {
	case ReferenceKindJobRequest, ReferenceKindServiceProposal, ReferenceKindWorkOrder:
		if id == "" || strings.IndexFunc(id, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return Reference{}, ErrInvalidSubmission
		}
		parsed, err := strconv.ParseInt(id, 10, 32)
		if err != nil || parsed <= 0 {
			return Reference{}, ErrInvalidSubmission
		}
		return Reference{kind: kind, id: strconv.FormatInt(parsed, 10)}, nil
	case ReferenceKindPaymentIntent:
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || len(id) != 36 {
			return Reference{}, ErrInvalidSubmission
		}
		return Reference{kind: kind, id: parsed.String()}, nil
	default:
		return Reference{}, ErrInvalidSubmission
	}
}
func (r Reference) Kind() ReferenceKind { return r.kind }
func (r Reference) ID() string          { return r.id }
