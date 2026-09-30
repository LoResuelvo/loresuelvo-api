package provider_handler

import (
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type activityPeriodResponse struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Granularity string    `json:"granularity,omitempty"`
	TimeZone    string    `json:"time_zone,omitempty"`
}

type activityResultsResponse struct {
	ConfirmedBookings   int64  `json:"confirmed_bookings"`
	ReportedCompletions int64  `json:"reported_completions"`
	FullyPaidWorkOrders int64  `json:"fully_paid_work_orders"`
	ClientsServed       int64  `json:"clients_served"`
	NewClients          int64  `json:"new_clients"`
	ReturningClients    int64  `json:"returning_clients"`
	AgreedValueCents    int64  `json:"agreed_value_cents"`
	AverageValueCents   *int64 `json:"average_value_cents"`
	Currency            string `json:"currency"`
}

type activityBucketResponse struct {
	From                time.Time `json:"from"`
	To                  time.Time `json:"to"`
	ConfirmedBookings   int64     `json:"confirmed_bookings"`
	ReportedCompletions int64     `json:"reported_completions"`
	FullyPaidWorkOrders int64     `json:"fully_paid_work_orders"`
}

type activityPendingResponse struct {
	Requests              int64 `json:"requests"`
	ScheduledOrders       int64 `json:"scheduled_orders"`
	AwaitingPaymentOrders int64 `json:"awaiting_payment_orders"`
}

type activityMetricChangeResponse struct {
	Absolute   *int64   `json:"absolute"`
	Percentage *float64 `json:"percentage"`
}

type activityChangesResponse struct {
	ConfirmedBookings   activityMetricChangeResponse `json:"confirmed_bookings"`
	ReportedCompletions activityMetricChangeResponse `json:"reported_completions"`
	FullyPaidWorkOrders activityMetricChangeResponse `json:"fully_paid_work_orders"`
	ClientsServed       activityMetricChangeResponse `json:"clients_served"`
	NewClients          activityMetricChangeResponse `json:"new_clients"`
	ReturningClients    activityMetricChangeResponse `json:"returning_clients"`
	AgreedValueCents    activityMetricChangeResponse `json:"agreed_value_cents"`
	AverageValueCents   activityMetricChangeResponse `json:"average_value_cents"`
}

type activityComparisonResponse struct {
	Period  activityPeriodResponse  `json:"period"`
	Results activityResultsResponse `json:"results"`
	Changes activityChangesResponse `json:"changes"`
}

type activityResponse struct {
	Period         activityPeriodResponse      `json:"period"`
	CalculatedAt   time.Time                   `json:"calculated_at"`
	Results        activityResultsResponse     `json:"results"`
	Evolution      []activityBucketResponse    `json:"evolution"`
	CurrentPending activityPendingResponse     `json:"current_pending"`
	Comparison     *activityComparisonResponse `json:"comparison,omitempty"`
}

func activityResponseFromDomain(activity *readmodel.Activity) activityResponse {
	response := activityResponse{
		Period:         activityPeriodResponse{From: activity.Period.From, To: activity.Period.To, Granularity: activity.Granularity, TimeZone: activity.TimeZone},
		CalculatedAt:   activity.CalculatedAt,
		Results:        activityResultsFromDomain(activity.Results),
		Evolution:      make([]activityBucketResponse, 0, len(activity.Series)),
		CurrentPending: activityPendingResponse{Requests: activity.Pending.Requests, ScheduledOrders: activity.Pending.Scheduled, AwaitingPaymentOrders: activity.Pending.AwaitingPayment},
	}
	for _, bucket := range activity.Series {
		response.Evolution = append(response.Evolution, activityBucketResponse{From: bucket.From, To: bucket.To, ConfirmedBookings: bucket.Confirmed, ReportedCompletions: bucket.Reported, FullyPaidWorkOrders: bucket.Paid})
	}
	if comparison := activity.Comparison; comparison != nil {
		change, percentage := comparison.Change, comparison.PercentChange
		response.Comparison = &activityComparisonResponse{
			Period:  activityPeriodResponse{From: comparison.Period.From, To: comparison.Period.To, Granularity: activity.Granularity, TimeZone: activity.TimeZone},
			Results: activityResultsFromDomain(comparison.Results),
			Changes: activityChangesResponse{
				ConfirmedBookings:   activityMetricChangeResponse{Absolute: int64Pointer(change.Confirmed), Percentage: percentage.Confirmed},
				ReportedCompletions: activityMetricChangeResponse{Absolute: int64Pointer(change.Reported), Percentage: percentage.Reported},
				FullyPaidWorkOrders: activityMetricChangeResponse{Absolute: int64Pointer(change.Paid), Percentage: percentage.Paid},
				ClientsServed:       activityMetricChangeResponse{Absolute: int64Pointer(change.Customers), Percentage: percentage.Customers},
				NewClients:          activityMetricChangeResponse{Absolute: int64Pointer(change.NewCustomers), Percentage: percentage.NewCustomers},
				ReturningClients:    activityMetricChangeResponse{Absolute: int64Pointer(change.ReturningCustomers), Percentage: percentage.ReturningCustomers},
				AgreedValueCents:    activityMetricChangeResponse{Absolute: int64Pointer(change.ContractValueCents), Percentage: percentage.ContractValueCents},
				AverageValueCents:   activityMetricChangeResponse{Absolute: change.AverageCents, Percentage: percentage.AverageCents},
			},
		}
	}
	return response
}

func activityResultsFromDomain(result readmodel.ActivityResults) activityResultsResponse {
	return activityResultsResponse{ConfirmedBookings: result.Confirmed, ReportedCompletions: result.Reported, FullyPaidWorkOrders: result.Paid, ClientsServed: result.Customers, NewClients: result.NewCustomers, ReturningClients: result.ReturningCustomers, AgreedValueCents: result.ContractValueCents, AverageValueCents: result.AverageCents, Currency: "ARS"}
}

func int64Pointer(value int64) *int64 { return &value }
