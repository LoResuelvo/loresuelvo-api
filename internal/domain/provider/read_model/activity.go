package readmodel

import "time"

// ActivityCounts contains the persisted facts used to calculate period results.
type ActivityCounts struct {
	Confirmed          int64
	Reported           int64
	Paid               int64
	Customers          int64
	NewCustomers       int64
	ReturningCustomers int64
	ContractValueCents int64
}

// ActivityBucket is a sparse calendar bucket returned by the reader. From is
// the untrimmed bucket start, even when the requested period starts midway.
type ActivityBucket struct {
	From      time.Time
	To        time.Time
	Confirmed int64
	Reported  int64
	Paid      int64
}

type ActivityPending struct {
	Requests        int64
	Scheduled       int64
	AwaitingPayment int64
}

// ActivitySnapshot must be read from one consistent database snapshot.
type ActivitySnapshot struct {
	Current  ActivityCounts
	Previous *ActivityCounts
	Series   []ActivityBucket
	Pending  ActivityPending
}

type ActivityPeriod struct {
	From time.Time
	To   time.Time
}

type ActivityResults struct {
	ActivityCounts
	AverageCents *int64
}

type ActivityChange struct {
	Confirmed          int64
	Reported           int64
	Paid               int64
	Customers          int64
	NewCustomers       int64
	ReturningCustomers int64
	ContractValueCents int64
	AverageCents       *int64
}

type ActivityPercentChange struct {
	Confirmed          *float64
	Reported           *float64
	Paid               *float64
	Customers          *float64
	NewCustomers       *float64
	ReturningCustomers *float64
	ContractValueCents *float64
	AverageCents       *float64
}

type ActivityComparison struct {
	Period        ActivityPeriod
	Results       ActivityResults
	Change        ActivityChange
	PercentChange ActivityPercentChange
}

type Activity struct {
	Period       ActivityPeriod
	Granularity  string
	TimeZone     string
	CalculatedAt time.Time
	Results      ActivityResults
	Series       []ActivityBucket
	Pending      ActivityPending
	Comparison   *ActivityComparison
}
