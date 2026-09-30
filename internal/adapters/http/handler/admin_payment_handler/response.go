package admin_payment_handler

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
)

type response struct {
	Payments []paymentResponse `json:"payments"`
	Page     pageResponse      `json:"page"`
}

type pageResponse struct {
	Limit      int     `json:"limit"`
	NextCursor *string `json:"next_cursor"`
}

type paymentResponse struct {
	ID                string                `json:"id"`
	ServiceProposalID int                   `json:"service_proposal_id"`
	WorkOrderID       *int                  `json:"work_order_id"`
	ConsumerID        int                   `json:"consumer_id"`
	ProviderID        int                   `json:"provider_id"`
	Purpose           string                `json:"purpose"`
	IntentStatus      string                `json:"intent_status"`
	Currency          string                `json:"currency"`
	SellerAmountCents int64                 `json:"seller_amount_cents"`
	PlatformFeeCents  int64                 `json:"platform_fee_cents"`
	TotalAmountCents  int64                 `json:"total_amount_cents"`
	CreatedOn         time.Time             `json:"created_on"`
	UpdatedOn         time.Time             `json:"updated_on"`
	Transactions      []transactionResponse `json:"transactions"`
	Breakdown         breakdownResponse     `json:"breakdown"`
	Summary           summaryResponse       `json:"summary"`
	Anomalies         []string              `json:"anomalies"`
}

type transactionResponse struct {
	ID                 int64      `json:"id"`
	Processor          string     `json:"processor"`
	ExternalPaymentID  string     `json:"external_payment_id"`
	Status             string     `json:"status"`
	Currency           string     `json:"currency"`
	AmountCents        int64      `json:"amount_cents"`
	VerifiedOn         *time.Time `json:"verified_on"`
	CreatedOn          time.Time  `json:"created_on"`
	UpdatedOn          time.Time  `json:"updated_on"`
	ProcessorFeeCents  *int64     `json:"processor_fee_cents"`
	NetSettlementCents *int64     `json:"net_settlement_cents"`
}

type breakdownResponse struct {
	Currency                     string    `json:"currency"`
	ServiceTotalCents            int64     `json:"service_total_cents"`
	DepositCents                 int64     `json:"deposit_cents"`
	PlatformFeeTotalCents        int64     `json:"platform_fee_total_cents"`
	PlatformFeeDueNowCents       int64     `json:"platform_fee_due_now_cents"`
	BookingPaymentDeadline       time.Time `json:"booking_payment_deadline"`
	AmountDueNowCents            *int64    `json:"amount_due_now_cents"`
	RemainingServiceBalanceCents *int64    `json:"remaining_service_balance_cents"`
	RemainingPlatformFeeCents    *int64    `json:"remaining_platform_fee_cents"`
	RemainingAmountDueCents      *int64    `json:"remaining_amount_due_cents"`
}

type amountResponse struct {
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amount_cents"`
}

type summaryResponse struct {
	ApprovedAmounts []amountResponse `json:"approved_amounts"`
	PendingAmount   *amountResponse  `json:"pending_amount"`
	Anomalies       []string         `json:"anomalies"`
}

func responseFromPage(page *readmodel.AdminPaymentPage, codec *signedcursor.Codec, query payment.AdminPaymentQuery) (response, error) {
	result := response{Payments: make([]paymentResponse, 0), Page: pageResponse{Limit: query.Limit}}
	if page == nil {
		return result, payment.ErrInvalidAdminPaymentQuery
	}
	result.Page.Limit = page.Limit
	for _, item := range page.Payments {
		result.Payments = append(result.Payments, paymentResponseFromModel(item))
	}
	if page.Next != nil && len(page.Payments) > 0 {
		token, err := codec.Encode(cursorFromQuery(query, *page.Next))
		if err != nil {
			return response{}, err
		}
		result.Page.NextCursor = &token
	}
	return result, nil
}

func paymentResponseFromModel(item readmodel.AdminPayment) paymentResponse {
	transactions := make([]transactionResponse, 0, len(item.Transactions))
	for _, transaction := range item.Transactions {
		transactions = append(transactions, transactionResponse{
			ID: transaction.ID, Processor: transaction.Processor,
			ExternalPaymentID: transaction.ExternalPaymentID, Status: transaction.Status,
			Currency: transaction.Currency, AmountCents: transaction.AmountCents,
			VerifiedOn: optionalUTC(transaction.VerifiedOn), CreatedOn: transaction.CreatedOn.UTC(),
			UpdatedOn:         transaction.UpdatedOn.UTC(),
			ProcessorFeeCents: nil, NetSettlementCents: nil,
		})
	}
	approvedAmounts := make([]amountResponse, 0, len(item.Summary.ApprovedAmounts))
	for _, amount := range item.Summary.ApprovedAmounts {
		approvedAmounts = append(approvedAmounts, amountResponse{Currency: amount.Currency, AmountCents: amount.AmountCents})
	}
	var pendingAmount *amountResponse
	if item.Summary.PendingAmount != nil {
		pendingAmount = &amountResponse{Currency: item.Summary.PendingAmount.Currency, AmountCents: item.Summary.PendingAmount.AmountCents}
	}

	return paymentResponse{
		ID: item.ID, ServiceProposalID: item.ServiceProposalID, WorkOrderID: item.WorkOrderID,
		ConsumerID: item.ConsumerID, ProviderID: item.ProviderID,
		Purpose: item.Purpose, IntentStatus: item.IntentStatus, Currency: item.Currency,
		SellerAmountCents: item.SellerAmountCents, PlatformFeeCents: item.PlatformFeeCents,
		TotalAmountCents: item.TotalAmountCents, CreatedOn: item.CreatedOn.UTC(), UpdatedOn: item.UpdatedOn.UTC(),
		Transactions: transactions,
		Breakdown: breakdownResponse{
			Currency: item.Breakdown.Currency, ServiceTotalCents: item.Breakdown.ServiceTotalCents,
			DepositCents: item.Breakdown.DepositCents, PlatformFeeTotalCents: item.Breakdown.PlatformFeeTotalCents,
			PlatformFeeDueNowCents:       item.Breakdown.PlatformFeeDueNowCents,
			BookingPaymentDeadline:       item.Breakdown.BookingPaymentDeadline.UTC(),
			AmountDueNowCents:            item.Breakdown.AmountDueNowCents,
			RemainingServiceBalanceCents: item.Breakdown.RemainingServiceBalanceCents,
			RemainingPlatformFeeCents:    item.Breakdown.RemainingPlatformFeeCents,
			RemainingAmountDueCents:      item.Breakdown.RemainingAmountDueCents,
		},
		Summary:   summaryResponse{ApprovedAmounts: approvedAmounts, PendingAmount: pendingAmount, Anomalies: nonNilStrings(item.Summary.Anomalies)},
		Anomalies: nonNilStrings(item.Anomalies),
	}
}

func optionalUTC(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
