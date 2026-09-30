package admin_payment_handler

import (
	"net/url"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
)

const cursorPurpose = "admin-payments:v1"

type paymentCursor struct {
	Version           int                            `json:"v"`
	PaymentIntentID   string                         `json:"payment_intent_id"`
	ExternalReference string                         `json:"external_reference"`
	ExternalPaymentID string                         `json:"external_payment_id"`
	ServiceProposalID int                            `json:"service_proposal_id"`
	ConsumerEmail     string                         `json:"consumer_email"`
	ProviderEmail     string                         `json:"provider_email"`
	Purpose           payment.Purpose                `json:"purpose"`
	IntentStatus      string                         `json:"intent_status"`
	CreatedFrom       *time.Time                     `json:"created_from"`
	CreatedTo         *time.Time                     `json:"created_to"`
	Limit             int                            `json:"limit"`
	After             readmodel.AdminPaymentPosition `json:"after"`
}

func parseQuery(raw string, codec *signedcursor.Codec) (payment.AdminPaymentQuery, error) {
	query := payment.AdminPaymentQuery{Limit: payment.DefaultAdminPaymentLimit}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return query, payment.ErrInvalidAdminPaymentQuery
	}
	for key, entries := range values {
		switch key {
		case "payment_intent_id", "external_reference", "external_payment_id", "service_proposal_id", "consumer_email", "provider_email", "purpose", "intent_status", "created_from", "created_to", "limit", "cursor":
		default:
			return query, payment.ErrInvalidAdminPaymentQuery
		}
		if len(entries) != 1 || entries[0] == "" {
			return query, payment.ErrInvalidAdminPaymentQuery
		}
	}

	if token := values.Get("cursor"); token != "" {
		var cursor paymentCursor
		if codec == nil || codec.Decode(token, &cursor) != nil || cursor.Version != 1 {
			return query, payment.ErrInvalidAdminPaymentQuery
		}
		query = queryFromCursor(cursor)
		if err := query.Validate(); err != nil {
			return query, payment.ErrInvalidAdminPaymentQuery
		}
	}

	for key, entries := range values {
		if key == "cursor" {
			continue
		}
		previous := query
		value := entries[0]
		switch key {
		case "payment_intent_id":
			query.PaymentIntentID = value
		case "external_reference":
			query.ExternalReference = value
		case "external_payment_id":
			query.ExternalPaymentID = value
		case "service_proposal_id":
			query.ServiceProposalID, err = parsePositiveInt(value)
		case "consumer_email":
			query.ConsumerEmail = value
		case "provider_email":
			query.ProviderEmail = value
		case "purpose":
			query.Purpose = payment.Purpose(value)
		case "intent_status":
			query.IntentStatus = value
		case "created_from", "created_to":
			var parsed time.Time
			parsed, err = time.Parse(time.RFC3339Nano, value)
			if err == nil {
				parsed = parsed.UTC()
				if key == "created_from" {
					query.CreatedFrom = &parsed
				} else {
					query.CreatedTo = &parsed
				}
			}
		case "limit":
			query.Limit, err = parsePositiveInt(value)
		}
		if err != nil {
			return query, payment.ErrInvalidAdminPaymentQuery
		}
		if values.Get("cursor") != "" && !sameQuery(previous, query) {
			return query, payment.ErrInvalidAdminPaymentQuery
		}
	}
	if err := query.Validate(); err != nil {
		return query, payment.ErrInvalidAdminPaymentQuery
	}
	return query, nil
}

func queryFromCursor(cursor paymentCursor) payment.AdminPaymentQuery {
	return payment.AdminPaymentQuery{
		PaymentIntentID: cursor.PaymentIntentID, ExternalReference: cursor.ExternalReference,
		ExternalPaymentID: cursor.ExternalPaymentID, ServiceProposalID: cursor.ServiceProposalID,
		ConsumerEmail: cursor.ConsumerEmail, ProviderEmail: cursor.ProviderEmail,
		Purpose: cursor.Purpose, IntentStatus: cursor.IntentStatus,
		CreatedFrom: cursor.CreatedFrom, CreatedTo: cursor.CreatedTo,
		Limit: cursor.Limit, After: &cursor.After,
	}
}

func cursorFromQuery(query payment.AdminPaymentQuery, position readmodel.AdminPaymentPosition) paymentCursor {
	return paymentCursor{
		Version: 1, PaymentIntentID: query.PaymentIntentID,
		ExternalReference: query.ExternalReference, ExternalPaymentID: query.ExternalPaymentID,
		ServiceProposalID: query.ServiceProposalID, ConsumerEmail: query.ConsumerEmail,
		ProviderEmail: query.ProviderEmail, Purpose: query.Purpose, IntentStatus: query.IntentStatus,
		CreatedFrom: query.CreatedFrom, CreatedTo: query.CreatedTo, Limit: query.Limit, After: position,
	}
}

func sameQuery(left, right payment.AdminPaymentQuery) bool {
	return left.PaymentIntentID == right.PaymentIntentID &&
		left.ExternalReference == right.ExternalReference &&
		left.ExternalPaymentID == right.ExternalPaymentID &&
		left.ServiceProposalID == right.ServiceProposalID &&
		left.ConsumerEmail == right.ConsumerEmail &&
		left.ProviderEmail == right.ProviderEmail &&
		left.Purpose == right.Purpose &&
		left.IntentStatus == right.IntentStatus &&
		left.Limit == right.Limit &&
		sameTime(left.CreatedFrom, right.CreatedFrom) &&
		sameTime(left.CreatedTo, right.CreatedTo)
}

func sameTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func parsePositiveInt(value string) (int, error) {
	if value == "" {
		return 0, payment.ErrInvalidAdminPaymentQuery
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, payment.ErrInvalidAdminPaymentQuery
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 1 {
		return 0, payment.ErrInvalidAdminPaymentQuery
	}
	return int(parsed), nil
}
