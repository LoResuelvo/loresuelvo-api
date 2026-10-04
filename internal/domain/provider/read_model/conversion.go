package readmodel

import (
	"errors"
	"math/big"
	"time"
)

type ConversionStages struct{ Issued, Contracted, Reported, Paid int64 }
type ConversionRequestCounts struct{ Received, Accepted, Pending int64 }
type ConversionSnapshot struct {
	Stages   ConversionStages
	Requests ConversionRequestCounts
}
type ConversionPeriod struct{ From, To time.Time }
type ConversionRatio struct {
	Numerator, Denominator int64
	Percentage             *float64
}
type ConversionStageRates struct{ Cohort, PreviousStage ConversionRatio }
type ConversionRates struct{ Contracted, Reported, Paid ConversionStageRates }
type ConversionProposals struct {
	Stages       ConversionStages
	Rates        ConversionRates
	Uncontracted int64
}
type ConversionRequests struct {
	ConversionRequestCounts
	AcceptanceRate ConversionRatio
}
type Conversion struct {
	Period     ConversionPeriod
	TimeZone   string
	ObservedAt time.Time
	Proposals  ConversionProposals
	Requests   ConversionRequests
}

// Calculate retains the distinct cohort and request denominators and rejects
// incomplete or inconsistent persisted facts rather than reporting empty results.
func (s ConversionSnapshot) Calculate(period ConversionPeriod, observedAt time.Time) (*Conversion, error) {
	stages, requests := s.Stages, s.Requests
	if stages.Paid < 0 || stages.Reported < stages.Paid || stages.Contracted < stages.Reported || stages.Issued < stages.Contracted || requests.Received < 0 || requests.Accepted < 0 || requests.Pending < 0 || requests.Accepted > requests.Received || requests.Pending != requests.Received-requests.Accepted {
		return nil, errors.New("conversion snapshot contains inconsistent counts")
	}
	rates := ConversionRates{
		Contracted: ConversionStageRates{conversionRatio(stages.Contracted, stages.Issued), conversionRatio(stages.Contracted, stages.Issued)},
		Reported:   ConversionStageRates{conversionRatio(stages.Reported, stages.Issued), conversionRatio(stages.Reported, stages.Contracted)},
		Paid:       ConversionStageRates{conversionRatio(stages.Paid, stages.Issued), conversionRatio(stages.Paid, stages.Reported)},
	}
	period.From, period.To = period.From.UTC(), period.To.UTC()
	return &Conversion{Period: period, TimeZone: "America/Argentina/Buenos_Aires", ObservedAt: observedAt.UTC(), Proposals: ConversionProposals{stages, rates, stages.Issued - stages.Contracted}, Requests: ConversionRequests{requests, conversionRatio(requests.Accepted, requests.Received)}}, nil
}

// conversionRatio uses exact HALF_UP rounding before conversion to float64.
// Multiplication uses arbitrary precision so even MaxInt64 counts are safe.
func conversionRatio(numerator, denominator int64) ConversionRatio {
	ratio := ConversionRatio{Numerator: numerator, Denominator: denominator}
	if denominator == 0 {
		return ratio
	}
	scaled := new(big.Int).Mul(big.NewInt(numerator), big.NewInt(10000))
	base := big.NewInt(denominator)
	quotient, remainder := new(big.Int).QuoRem(scaled, base, new(big.Int))
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(base) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	value, _ := new(big.Rat).SetFrac(quotient, big.NewInt(100)).Float64()
	ratio.Percentage = &value
	return ratio
}
