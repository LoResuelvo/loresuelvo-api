package payment

import (
	"errors"
	"time"
	"unicode/utf8"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/google/uuid"
)

var ErrInvalidAdminPaymentQuery = errors.New("invalid administrative payment query")
var ErrAdminPaymentAmountOverflow = errors.New("administrative payment amount exceeds supported range")

const DefaultAdminPaymentLimit = 20
const MaxAdminPaymentLimit = 100

type AdminPaymentQuery struct {
	PaymentIntentID, ExternalReference, ExternalPaymentID string
	ServiceProposalID                                     int
	ConsumerEmail, ProviderEmail                          string
	Purpose                                               Purpose
	IntentStatus                                          string
	CreatedFrom, CreatedTo                                *time.Time
	Limit                                                 int
	After                                                 *readmodel.AdminPaymentPosition
}

func (q AdminPaymentQuery) Validate() error {
	if q.Limit < 1 || q.Limit > MaxAdminPaymentLimit || q.ServiceProposalID < 0 || int64(q.ServiceProposalID) > 2147483647 {
		return ErrInvalidAdminPaymentQuery
	}
	for _, id := range []string{q.PaymentIntentID, q.ExternalReference} {
		if id != "" && !validAdminPaymentID(id) {
			return ErrInvalidAdminPaymentQuery
		}
	}
	if q.Purpose != "" && q.Purpose != PurposeBookingDeposit && q.Purpose != PurposeServiceBalance {
		return ErrInvalidAdminPaymentQuery
	}
	switch q.IntentStatus {
	case "", "requires_checkout", "checkout_ready", "processing", "paid", "rejected", "expired", "cancelled", "refunded", "disputed", "payment_mismatch":
	default:
		return ErrInvalidAdminPaymentQuery
	}
	for _, s := range []string{q.ConsumerEmail, q.ProviderEmail, q.ExternalPaymentID} {
		if len(s) > 255 || !utf8.ValidString(s) {
			return ErrInvalidAdminPaymentQuery
		}
		for _, r := range s {
			if r < 32 || r == 127 {
				return ErrInvalidAdminPaymentQuery
			}
		}
	}
	if q.CreatedFrom != nil && q.CreatedFrom.IsZero() || q.CreatedTo != nil && q.CreatedTo.IsZero() || q.CreatedFrom != nil && q.CreatedTo != nil && !q.CreatedFrom.Before(*q.CreatedTo) {
		return ErrInvalidAdminPaymentQuery
	}
	if q.After != nil && (q.After.CreatedOn.IsZero() || !validAdminPaymentID(q.After.ID)) {
		return ErrInvalidAdminPaymentQuery
	}
	return nil
}
func validAdminPaymentID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}
