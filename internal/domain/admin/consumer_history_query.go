package admin

import (
	"errors"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"time"
)

var ErrInvalidConsumerHistoryQuery = errors.New("invalid consumer history query")
var ErrConsumerHistoryNotFound = errors.New("consumer not found")

const DefaultConsumerHistoryLimit = 20
const MaxConsumerHistoryLimit = 100

type ConsumerHistoryQuery struct {
	Type, Status string
	ProviderID   int
	From, To     *time.Time
	Limit        int
	After        *readmodel.ConsumerHistoryPosition
}

func (q ConsumerHistoryQuery) Validate() error {
	if q.Limit < 1 || q.Limit > MaxConsumerHistoryLimit || q.ProviderID < 0 || int64(q.ProviderID) > 2147483647 {
		return ErrInvalidConsumerHistoryQuery
	}
	switch q.Type {
	case "", "job_request", "service_proposal", "work_order":
	default:
		return ErrInvalidConsumerHistoryQuery
	}
	if q.Status != "" {
		valid := false
		switch q.Type {
		case "job_request":
			valid = q.Status == "pending" || q.Status == "accepted"
		case "service_proposal":
			valid = q.Status == "pending" || q.Status == "accepted" || q.Status == "rejected"
		case "work_order":
			valid = q.Status == "scheduled" || q.Status == "awaiting_payment" || q.Status == "paid"
		}
		if !valid {
			return ErrInvalidConsumerHistoryQuery
		}
	}
	if (q.From != nil && q.From.IsZero()) || (q.To != nil && q.To.IsZero()) || (q.From != nil && q.To != nil && !q.From.Before(*q.To)) {
		return ErrInvalidConsumerHistoryQuery
	}
	if q.After != nil {
		p := q.After
		if p.ID < 1 || int64(p.ID) > 2147483647 || p.OccurredOn.IsZero() {
			return ErrInvalidConsumerHistoryQuery
		}
		switch p.Type {
		case "job_request", "service_proposal", "work_order":
		default:
			return ErrInvalidConsumerHistoryQuery
		}
		if q.Type != "" && p.Type != q.Type {
			return ErrInvalidConsumerHistoryQuery
		}
		if q.From != nil && p.OccurredOn.Before(*q.From) || q.To != nil && !p.OccurredOn.Before(*q.To) {
			return ErrInvalidConsumerHistoryQuery
		}
	}
	return nil
}
