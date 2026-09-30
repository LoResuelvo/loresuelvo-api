package operation_funnel_handler

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

const (
	aiCategorySource     = "assessment.problem_category_id"
	manualCategorySource = "provider.current_category_id"
)

type funnelResponse struct {
	Period        periodResponse  `json:"period"`
	TimeZone      string          `json:"timezone"`
	ObservedAt    time.Time       `json:"observed_at"`
	CategoryID    *int            `json:"category_id"`
	Rounding      string          `json:"rounding"`
	DecimalPlaces int             `json:"decimal_places"`
	Cohorts       cohortsResponse `json:"cohorts"`
}

type periodResponse struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type cohortsResponse struct {
	AI     cohortResponse `json:"ai"`
	Manual cohortResponse `json:"manual"`
}

type cohortResponse struct {
	CategorySource                    string          `json:"category_source"`
	Stages                            []stageResponse `json:"stages"`
	GlobalCompletionConversionPercent *json.Number    `json:"global_completion_conversion_percentage"`
	Delays                            delaysResponse  `json:"delays"`
}

type stageResponse struct {
	Stage                string       `json:"stage"`
	Count                int64        `json:"count"`
	ConversionPercentage *json.Number `json:"conversion_percentage"`
}

type delaysResponse struct {
	RequestToFirstProposal              delayResponse `json:"request_to_first_proposal"`
	ProposalToConfirmedHiring           delayResponse `json:"proposal_to_confirmed_hiring"`
	ConfirmedHiringToReportedCompletion delayResponse `json:"confirmed_hiring_to_reported_completion"`
	ReportedCompletionToFullPayment     delayResponse `json:"reported_completion_to_full_payment"`
}

type delayResponse struct {
	Observations int64        `json:"observations"`
	MeanSeconds  *json.Number `json:"mean_seconds"`
}

func responseFromMetrics(metrics readmodel.FunnelMetrics) funnelResponse {
	return funnelResponse{
		Period:   periodResponse{From: metrics.From.UTC(), To: metrics.To.UTC()},
		TimeZone: metrics.TimeZone, ObservedAt: metrics.ObservedAt.UTC(),
		CategoryID: metrics.CategoryID, Rounding: metrics.Rounding, DecimalPlaces: metrics.DecimalPlaces,
		Cohorts: cohortsResponse{
			AI:     cohortResponseFromModel(metrics.AI, aiCategorySource),
			Manual: cohortResponseFromModel(metrics.Manual, manualCategorySource),
		},
	}
}

func cohortResponseFromModel(cohort readmodel.FunnelCohort, categorySource string) cohortResponse {
	stages := make([]stageResponse, 0, len(cohort.Stages))
	for _, stage := range cohort.Stages {
		stages = append(stages, stageResponse{
			Stage: string(stage.Stage), Count: stage.Count,
			ConversionPercentage: decimalHundredths(stage.ConversionHundredths),
		})
	}
	return cohortResponse{
		CategorySource:                    categorySource,
		Stages:                            stages,
		GlobalCompletionConversionPercent: decimalHundredths(cohort.GlobalCompletionConversionHundredths),
		Delays: delaysResponse{
			RequestToFirstProposal:              delayResponseFromModel(cohort.Delays.RequestToFirstProposal),
			ProposalToConfirmedHiring:           delayResponseFromModel(cohort.Delays.ProposalToConfirmedHiring),
			ConfirmedHiringToReportedCompletion: delayResponseFromModel(cohort.Delays.ConfirmedHiringToReportedCompletion),
			ReportedCompletionToFullPayment:     delayResponseFromModel(cohort.Delays.ReportedCompletionToFullPayment),
		},
	}
}

func delayResponseFromModel(delay readmodel.FunnelDelayMetric) delayResponse {
	return delayResponse{Observations: delay.Observations, MeanSeconds: decimalHundredths(delay.MeanSecondsHundredths)}
}

func decimalHundredths(value *int64) *json.Number {
	if value == nil {
		return nil
	}
	scaled := new(big.Int).SetInt64(*value)
	negative := scaled.Sign() < 0
	whole, remainder := new(big.Int), new(big.Int)
	whole.QuoRem(scaled, big.NewInt(100), remainder)
	whole.Abs(whole)
	remainder.Abs(remainder)
	decimal := fmt.Sprintf("%s.%02d", whole.String(), remainder.Int64())
	if negative {
		decimal = "-" + decimal
	}
	formatted := json.Number(decimal)
	return &formatted
}
