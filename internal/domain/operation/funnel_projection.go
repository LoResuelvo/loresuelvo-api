package operation

import (
	"math/big"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

// roundedRatio returns exact nonnegative HALF_UP hundredths without intermediate rounding.
func roundedRatio(numerator, denominator *big.Int) (*int64, error) {
	if denominator.Sign() == 0 {
		return nil, nil
	}
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Lsh(remainder, 1).Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return nil, ErrInvalidFunnelSnapshot
	}
	return new(quotient.Int64()), nil
}
func funnelConversion(current, prior int64) (*int64, error) {
	return roundedRatio(new(big.Int).Mul(big.NewInt(current), big.NewInt(10000)), big.NewInt(prior))
}
func projectFunnelDelay(total readmodel.FunnelDelayAggregate) (readmodel.FunnelDelayMetric, error) {
	sum, ok := new(big.Rat).SetString(total.TotalMicroseconds)
	if !ok || sum.Sign() < 0 || !sum.IsInt() || total.Observations < 0 || (total.Observations == 0 && sum.Sign() != 0) {
		return readmodel.FunnelDelayMetric{}, ErrInvalidFunnelSnapshot
	}
	denominator := new(big.Int).Mul(big.NewInt(total.Observations), big.NewInt(10000))
	mean, err := roundedRatio(sum.Num(), denominator)
	if err != nil {
		return readmodel.FunnelDelayMetric{}, err
	}
	return readmodel.FunnelDelayMetric{Observations: total.Observations, MeanSecondsHundredths: mean}, nil
}
func projectFunnelCohort(snapshot readmodel.FunnelCohortSnapshot, manual bool) (readmodel.FunnelCohort, error) {
	counts := snapshot.Counts
	values := []int64{counts.Origins, counts.Requested, counts.Proposed, counts.Hired, counts.Completed, counts.Paid, counts.Reviewed}
	stages := []readmodel.FunnelStage{readmodel.FunnelStageProfessionalAssessment, readmodel.FunnelStageRequest, readmodel.FunnelStageProposal, readmodel.FunnelStageConfirmedHiring, readmodel.FunnelStageReportedCompletion, readmodel.FunnelStageFullPayment, readmodel.FunnelStageReview}
	for i, value := range values {
		if value < 0 || (i > 0 && value > values[i-1]) {
			return readmodel.FunnelCohort{}, ErrInvalidFunnelSnapshot
		}
	}
	if manual {
		if counts.Requested != counts.Origins {
			return readmodel.FunnelCohort{}, ErrInvalidFunnelSnapshot
		}
		values, stages = values[1:], stages[1:]
	}
	cohort := readmodel.FunnelCohort{Stages: make([]readmodel.FunnelStageMetric, len(stages))}
	for i, stage := range stages {
		metric := readmodel.FunnelStageMetric{Stage: stage, Count: values[i]}
		if i > 0 {
			conversion, err := funnelConversion(values[i], values[i-1])
			if err != nil {
				return readmodel.FunnelCohort{}, err
			}
			metric.ConversionHundredths = conversion
		}
		cohort.Stages[i] = metric
	}
	global, err := funnelConversion(counts.Completed, counts.Origins)
	if err != nil {
		return readmodel.FunnelCohort{}, err
	}
	cohort.GlobalCompletionConversionHundredths = global
	totals := []readmodel.FunnelDelayAggregate{snapshot.Delays.RequestToFirstProposal, snapshot.Delays.ProposalToConfirmedHiring, snapshot.Delays.ConfirmedHiringToReportedCompletion, snapshot.Delays.ReportedCompletionToFullPayment}
	metrics := make([]readmodel.FunnelDelayMetric, 4)
	for i, total := range totals {
		metric, err := projectFunnelDelay(total)
		if err != nil {
			return readmodel.FunnelCohort{}, err
		}
		metrics[i] = metric
	}
	cohort.Delays = readmodel.FunnelDelays{RequestToFirstProposal: metrics[0], ProposalToConfirmedHiring: metrics[1], ConfirmedHiringToReportedCompletion: metrics[2], ReportedCompletionToFullPayment: metrics[3]}
	return cohort, nil
}
