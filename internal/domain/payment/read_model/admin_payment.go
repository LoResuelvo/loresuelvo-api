package readmodel

import "time"

type AdminPaymentPosition struct {
	CreatedOn time.Time
	ID        string
}
type AdminPaymentAmount struct {
	Currency    string
	AmountCents int64
}
type AdminPaymentTransaction struct {
	ID                                             int64
	Processor, ExternalPaymentID, Status, Currency string
	AmountCents                                    int64
	VerifiedOn, CreatedOn, UpdatedOn               time.Time
}
type AdminPaymentBreakdown struct {
	Currency                                                                                            string
	ServiceTotalCents, DepositCents, PlatformFeeTotalCents, PlatformFeeDueNowCents                      int64
	BookingPaymentDeadline                                                                              time.Time
	AmountDueNowCents, RemainingServiceBalanceCents, RemainingPlatformFeeCents, RemainingAmountDueCents *int64
}
type AdminPaymentSummary struct {
	ApprovedAmounts []AdminPaymentAmount
	PendingAmount   *AdminPaymentAmount
	Anomalies       []string
}
type AdminPayment struct {
	ID                                                    string
	ServiceProposalID                                     int
	WorkOrderID                                           *int
	ConsumerID, ProviderID                                int
	Purpose, IntentStatus, Currency                       string
	SellerAmountCents, PlatformFeeCents, TotalAmountCents int64
	CreatedOn, UpdatedOn                                  time.Time
	Transactions                                          []AdminPaymentTransaction
	Breakdown                                             AdminPaymentBreakdown
	Summary                                               AdminPaymentSummary
	Anomalies                                             []string
}
type AdminPaymentProposal struct {
	ID              int
	Status          string
	WorkOrderID     *int
	WorkOrderStatus string
	Breakdown       AdminPaymentBreakdown
	Intents         []AdminPayment
}
type AdminPaymentSnapshot struct {
	Payments  []AdminPayment
	Proposals []AdminPaymentProposal
	HasMore   bool
}
type AdminPaymentPage struct {
	Payments []AdminPayment
	Limit    int
	Next     *AdminPaymentPosition
}
