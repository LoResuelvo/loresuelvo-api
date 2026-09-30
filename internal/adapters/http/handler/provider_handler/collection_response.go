package provider_handler

import (
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type collectionPeriodResponse struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Granularity string    `json:"granularity,omitempty"`
	TimeZone    string    `json:"time_zone"`
}

type collectionAmountsResponse struct {
	BookingDepositCents int64 `json:"booking_deposit_cents"`
	ServiceBalanceCents int64 `json:"service_balance_cents"`
	TotalCents          int64 `json:"total_cents"`
}

type collectionBucketResponse struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	collectionAmountsResponse
}

type collectionPendingBalanceResponse struct {
	Orders      int64 `json:"orders"`
	AmountCents int64 `json:"amount_cents"`
}

type collectionPendingResponse struct {
	Scheduled       collectionPendingBalanceResponse `json:"scheduled"`
	AwaitingPayment collectionPendingBalanceResponse `json:"awaiting_payment"`
}

type collectionMetricChangeResponse struct {
	Absolute   int64    `json:"absolute"`
	Percentage *float64 `json:"percentage"`
}

type collectionChangesResponse struct {
	BookingDepositCents collectionMetricChangeResponse `json:"booking_deposit_cents"`
	ServiceBalanceCents collectionMetricChangeResponse `json:"service_balance_cents"`
	TotalCents          collectionMetricChangeResponse `json:"total_cents"`
}

type collectionComparisonResponse struct {
	Period  collectionPeriodResponse  `json:"period"`
	Results collectionAmountsResponse `json:"results"`
	Changes collectionChangesResponse `json:"changes"`
}

type collectionSummaryResponse struct {
	Period         collectionPeriodResponse      `json:"period"`
	CalculatedAt   time.Time                     `json:"calculated_at"`
	Currency       string                        `json:"currency"`
	Results        collectionAmountsResponse     `json:"results"`
	Evolution      []collectionBucketResponse    `json:"evolution"`
	CurrentPending collectionPendingResponse     `json:"current_pending"`
	Comparison     *collectionComparisonResponse `json:"comparison,omitempty"`
}

func collectionSummaryResponseFromDomain(result *readmodel.Collections) collectionSummaryResponse {
	response := collectionSummaryResponse{
		Period:       collectionPeriodResponse{From: result.Period.From, To: result.Period.To, Granularity: result.Granularity, TimeZone: result.TimeZone},
		CalculatedAt: result.CalculatedAt,
		Currency:     "ARS",
		Results:      collectionAmountsFromDomain(result.Results),
		Evolution:    make([]collectionBucketResponse, 0, len(result.Series)),
		CurrentPending: collectionPendingResponse{
			Scheduled:       collectionPendingBalanceResponse{Orders: result.Pending.Scheduled.Orders, AmountCents: result.Pending.Scheduled.AmountCents},
			AwaitingPayment: collectionPendingBalanceResponse{Orders: result.Pending.AwaitingPayment.Orders, AmountCents: result.Pending.AwaitingPayment.AmountCents},
		},
	}
	for _, bucket := range result.Series {
		response.Evolution = append(response.Evolution, collectionBucketResponse{From: bucket.From, To: bucket.To, collectionAmountsResponse: collectionAmountsFromDomain(bucket.CollectionAmounts)})
	}
	if comparison := result.Comparison; comparison != nil {
		response.Comparison = &collectionComparisonResponse{
			Period:  collectionPeriodResponse{From: comparison.Period.From, To: comparison.Period.To, Granularity: result.Granularity, TimeZone: result.TimeZone},
			Results: collectionAmountsFromDomain(comparison.Results),
			Changes: collectionChangesResponse{
				BookingDepositCents: collectionMetricChangeResponse{Absolute: comparison.Changes.Absolute.BookingDepositCents, Percentage: comparison.Changes.Percentage.BookingDepositCents},
				ServiceBalanceCents: collectionMetricChangeResponse{Absolute: comparison.Changes.Absolute.ServiceBalanceCents, Percentage: comparison.Changes.Percentage.ServiceBalanceCents},
				TotalCents:          collectionMetricChangeResponse{Absolute: comparison.Changes.Absolute.TotalCents, Percentage: comparison.Changes.Percentage.TotalCents},
			},
		}
	}
	return response
}

func collectionAmountsFromDomain(amounts readmodel.CollectionAmounts) collectionAmountsResponse {
	return collectionAmountsResponse{BookingDepositCents: amounts.BookingDepositCents, ServiceBalanceCents: amounts.ServiceBalanceCents, TotalCents: amounts.TotalCents}
}

type collectionTransactionResponse struct {
	ID                int64     `json:"id"`
	VerifiedOn        time.Time `json:"verified_on"`
	Purpose           string    `json:"purpose"`
	SellerAmountCents int64     `json:"seller_amount_cents"`
	Currency          string    `json:"currency"`
	ServiceProposalID int       `json:"service_proposal_id"`
	WorkOrderID       *int      `json:"work_order_id"`
}

type collectionDetailResponse struct {
	Period           collectionPeriodResponse        `json:"period"`
	CalculatedAt     time.Time                       `json:"calculated_at"`
	Currency         string                          `json:"currency"`
	TotalCount       int64                           `json:"total_count"`
	TotalAmountCents int64                           `json:"total_amount_cents"`
	Transactions     []collectionTransactionResponse `json:"transactions"`
	NextCursor       *string                         `json:"next_cursor"`
}

func collectionDetailResponseFromDomain(detail *readmodel.CollectionDetail) collectionDetailResponse {
	response := collectionDetailResponse{
		Period:           collectionPeriodResponse{From: detail.Period.From, To: detail.Period.To, TimeZone: detail.TimeZone},
		CalculatedAt:     detail.CalculatedAt,
		Currency:         "ARS",
		TotalCount:       detail.TotalCount,
		TotalAmountCents: detail.TotalAmountCents,
		Transactions:     make([]collectionTransactionResponse, 0, len(detail.Transactions)),
	}
	for _, row := range detail.Transactions {
		response.Transactions = append(response.Transactions, collectionTransactionResponse{
			ID: row.ID, VerifiedOn: row.VerifiedOn, Purpose: row.Purpose,
			SellerAmountCents: row.SellerAmountCents, Currency: row.Currency,
			ServiceProposalID: row.ServiceProposalID, WorkOrderID: row.WorkOrderID,
		})
	}
	return response
}
