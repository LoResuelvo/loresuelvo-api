package readmodel

import "time"

// CollectionAmounts are the provider's contractual share, in ARS cents.
type CollectionAmounts struct {
	BookingDepositCents int64
	ServiceBalanceCents int64
	TotalCents          int64
}

// CollectionBucket is returned by the reader with its untrimmed calendar start.
type CollectionBucket struct {
	From time.Time
	To   time.Time
	CollectionAmounts
}

type PendingBalance struct {
	Orders      int64
	AmountCents int64
}

type CollectionPending struct {
	Scheduled       PendingBalance
	AwaitingPayment PendingBalance
}

// CollectionSnapshot contains facts from one consistent database snapshot.
type CollectionSnapshot struct {
	Current  CollectionAmounts
	Previous *CollectionAmounts
	Series   []CollectionBucket
	Pending  CollectionPending
}

type CollectionChanges struct {
	Absolute   CollectionAmounts
	Percentage struct {
		BookingDepositCents *float64
		ServiceBalanceCents *float64
		TotalCents          *float64
	}
}

type CollectionComparison struct {
	Period  ActivityPeriod
	Results CollectionAmounts
	Changes CollectionChanges
}

type Collections struct {
	Period       ActivityPeriod
	Granularity  string
	TimeZone     string
	CalculatedAt time.Time
	Results      CollectionAmounts
	Series       []CollectionBucket
	Pending      CollectionPending
	Comparison   *CollectionComparison
}

type CollectionPosition struct {
	VerifiedOn time.Time
	ID         int64
}

type CollectionTransaction struct {
	ID                int64
	VerifiedOn        time.Time
	Purpose           string
	SellerAmountCents int64
	Currency          string
	ServiceProposalID int
	WorkOrderID       *int
}

// CollectionDetailSnapshot includes whole-filter totals independent of Page.
type CollectionDetailSnapshot struct {
	TotalCount       int64
	TotalAmountCents int64
	Page             []CollectionTransaction
	Next             *CollectionPosition
}

type CollectionDetail struct {
	Period           ActivityPeriod
	TimeZone         string
	CalculatedAt     time.Time
	TotalCount       int64
	TotalAmountCents int64
	Transactions     []CollectionTransaction
	Next             *CollectionPosition
}
