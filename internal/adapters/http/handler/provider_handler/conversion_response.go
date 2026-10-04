package provider_handler

import (
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type conversionPeriodResponse struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	TimeZone string    `json:"time_zone"`
}
type conversionRatioResponse struct {
	Numerator   int64    `json:"numerator"`
	Denominator int64    `json:"denominator"`
	Percentage  *float64 `json:"percentage"`
}
type conversionStageRatesResponse struct {
	Cohort        conversionRatioResponse `json:"cohort"`
	PreviousStage conversionRatioResponse `json:"previous_stage"`
}
type conversionStagesResponse struct {
	Issued     int64 `json:"issued"`
	Contracted int64 `json:"contracted"`
	Reported   int64 `json:"reported"`
	Paid       int64 `json:"paid"`
}
type conversionRatesResponse struct {
	Contracted conversionStageRatesResponse `json:"contracted"`
	Reported   conversionStageRatesResponse `json:"reported"`
	Paid       conversionStageRatesResponse `json:"paid"`
}
type conversionProposalsResponse struct {
	Stages       conversionStagesResponse `json:"stages"`
	Rates        conversionRatesResponse  `json:"rates"`
	Uncontracted int64                    `json:"uncontracted"`
}
type conversionRequestsResponse struct {
	Received       int64                   `json:"received"`
	Accepted       int64                   `json:"accepted"`
	Pending        int64                   `json:"pending"`
	AcceptanceRate conversionRatioResponse `json:"acceptance_rate"`
}
type conversionResponse struct {
	Period     conversionPeriodResponse    `json:"period"`
	ObservedAt time.Time                   `json:"observed_at"`
	Proposals  conversionProposalsResponse `json:"proposals"`
	Requests   conversionRequestsResponse  `json:"requests"`
}

func conversionResponseFromDomain(result *readmodel.Conversion) conversionResponse {
	stages := result.Proposals.Stages
	rates := result.Proposals.Rates
	return conversionResponse{
		Period:     conversionPeriodResponse{From: result.Period.From.UTC(), To: result.Period.To.UTC(), TimeZone: result.TimeZone},
		ObservedAt: result.ObservedAt.UTC(),
		Proposals: conversionProposalsResponse{
			Stages:       conversionStagesResponse{Issued: stages.Issued, Contracted: stages.Contracted, Reported: stages.Reported, Paid: stages.Paid},
			Rates:        conversionRatesResponse{Contracted: conversionStageRatesFromDomain(rates.Contracted), Reported: conversionStageRatesFromDomain(rates.Reported), Paid: conversionStageRatesFromDomain(rates.Paid)},
			Uncontracted: result.Proposals.Uncontracted,
		},
		Requests: conversionRequestsResponse{Received: result.Requests.Received, Accepted: result.Requests.Accepted, Pending: result.Requests.Pending, AcceptanceRate: conversionRatioFromDomain(result.Requests.AcceptanceRate)},
	}
}
func conversionStageRatesFromDomain(rates readmodel.ConversionStageRates) conversionStageRatesResponse {
	return conversionStageRatesResponse{Cohort: conversionRatioFromDomain(rates.Cohort), PreviousStage: conversionRatioFromDomain(rates.PreviousStage)}
}
func conversionRatioFromDomain(ratio readmodel.ConversionRatio) conversionRatioResponse {
	return conversionRatioResponse{Numerator: ratio.Numerator, Denominator: ratio.Denominator, Percentage: ratio.Percentage}
}
