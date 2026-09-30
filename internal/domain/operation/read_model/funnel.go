package readmodel

import "time"

type FunnelSnapshot struct{ AI, Manual FunnelCohortSnapshot }
type FunnelCohortSnapshot struct {
	Counts FunnelCounts
	Delays FunnelDelayTotals
}
type FunnelCounts struct{ Origins, Requested, Proposed, Hired, Completed, Paid, Reviewed int64 }
type FunnelDelayAggregate struct {
	Observations      int64
	TotalMicroseconds string
}
type FunnelDelayTotals struct {
	RequestToFirstProposal, ProposalToConfirmedHiring, ConfirmedHiringToReportedCompletion, ReportedCompletionToFullPayment FunnelDelayAggregate
}
type FunnelMetrics struct {
	From, To, ObservedAt time.Time
	CategoryID           *int
	TimeZone, Rounding   string
	DecimalPlaces        int
	AI, Manual           FunnelCohort
}
type FunnelCohort struct {
	Stages                               []FunnelStageMetric
	GlobalCompletionConversionHundredths *int64
	Delays                               FunnelDelays
}
type FunnelStage string

const (
	FunnelStageProfessionalAssessment FunnelStage = "professional_assessment"
	FunnelStageRequest                FunnelStage = "request"
	FunnelStageProposal               FunnelStage = "proposal"
	FunnelStageConfirmedHiring        FunnelStage = "confirmed_hiring"
	FunnelStageReportedCompletion     FunnelStage = "reported_completion"
	FunnelStageFullPayment            FunnelStage = "full_payment"
	FunnelStageReview                 FunnelStage = "review"
)

type FunnelStageMetric struct {
	Stage                FunnelStage
	Count                int64
	ConversionHundredths *int64
}
type FunnelDelays struct {
	RequestToFirstProposal, ProposalToConfirmedHiring, ConfirmedHiringToReportedCompletion, ReportedCompletionToFullPayment FunnelDelayMetric
}
type FunnelDelayMetric struct {
	Observations          int64
	MeanSecondsHundredths *int64
}
