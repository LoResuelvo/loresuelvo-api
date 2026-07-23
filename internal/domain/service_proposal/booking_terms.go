package serviceproposal

import (
	"fmt"
	"math"
	"time"
)

const (
	bookingCurrencyARS               = "ARS"
	bookingDepositRateBasisPoints    = int64(2000)
	bookingPlatformFeeTotalCents     = int64(500000)
	percentageBasisPointsDenominator = int64(10000)
)

type BookingPolicy struct {
	configured bool
}

func NewBookingPolicy() BookingPolicy {
	return BookingPolicy{configured: true}
}

func (policy BookingPolicy) Calculate(serviceTotalCents int64, scheduledOn time.Time) (BookingTerms, error) {
	if !policy.configured {
		return BookingTerms{}, fmt.Errorf("calculating booking terms: booking policy is required")
	}
	if serviceTotalCents <= 0 {
		return BookingTerms{}, ErrInvalidAmount
	}
	if serviceTotalCents > math.MaxInt64-bookingPlatformFeeTotalCents {
		return BookingTerms{}, ErrInvalidAmount
	}

	depositCents := policy.percentageOf(serviceTotalCents, bookingDepositRateBasisPoints)
	platformFeeDueNowCents := policy.percentageOf(bookingPlatformFeeTotalCents, bookingDepositRateBasisPoints)

	return NewBookingTerms(
		bookingCurrencyARS,
		serviceTotalCents,
		depositCents,
		bookingPlatformFeeTotalCents,
		platformFeeDueNowCents,
		scheduledOn.Add(-minimumBookingLeadTime),
	)
}

func (policy BookingPolicy) percentageOf(amountCents, rateBasisPoints int64) int64 {
	whole := (amountCents / percentageBasisPointsDenominator) * rateBasisPoints
	remainder := amountCents % percentageBasisPointsDenominator
	fraction := (remainder*rateBasisPoints + percentageBasisPointsDenominator/2) /
		percentageBasisPointsDenominator
	return whole + fraction
}

type BookingTerms struct {
	currency                     string
	serviceTotalCents            int64
	depositCents                 int64
	remainingServiceBalanceCents int64
	platformFeeTotalCents        int64
	platformFeeDueNowCents       int64
	remainingPlatformFeeCents    int64
	amountDueNowCents            int64
	remainingAmountDueCents      int64
	contractTotalCents           int64
	bookingPaymentDeadline       time.Time
}

func NewBookingTerms(
	currency string,
	serviceTotalCents int64,
	depositCents int64,
	platformFeeTotalCents int64,
	platformFeeDueNowCents int64,
	bookingPaymentDeadline time.Time,
) (BookingTerms, error) {
	err := validate(
		currency,
		serviceTotalCents,
		depositCents,
		platformFeeTotalCents,
		platformFeeDueNowCents,
		bookingPaymentDeadline,
	)
	if err != nil {
		return BookingTerms{}, err
	}

	remainingServiceBalanceCents := serviceTotalCents - depositCents
	remainingPlatformFeeCents := platformFeeTotalCents - platformFeeDueNowCents

	return BookingTerms{
		currency:                     currency,
		serviceTotalCents:            serviceTotalCents,
		depositCents:                 depositCents,
		remainingServiceBalanceCents: remainingServiceBalanceCents,
		platformFeeTotalCents:        platformFeeTotalCents,
		platformFeeDueNowCents:       platformFeeDueNowCents,
		remainingPlatformFeeCents:    remainingPlatformFeeCents,
		amountDueNowCents:            depositCents + platformFeeDueNowCents,
		remainingAmountDueCents:      remainingServiceBalanceCents + remainingPlatformFeeCents,
		contractTotalCents:           serviceTotalCents + platformFeeTotalCents,
		bookingPaymentDeadline:       bookingPaymentDeadline,
	}, nil
}

func (terms BookingTerms) Currency() string         { return terms.currency }
func (terms BookingTerms) ServiceTotalCents() int64 { return terms.serviceTotalCents }
func (terms BookingTerms) DepositCents() int64      { return terms.depositCents }
func (terms BookingTerms) RemainingServiceBalanceCents() int64 {
	return terms.remainingServiceBalanceCents
}
func (terms BookingTerms) PlatformFeeTotalCents() int64     { return terms.platformFeeTotalCents }
func (terms BookingTerms) PlatformFeeDueNowCents() int64    { return terms.platformFeeDueNowCents }
func (terms BookingTerms) RemainingPlatformFeeCents() int64 { return terms.remainingPlatformFeeCents }
func (terms BookingTerms) AmountDueNowCents() int64         { return terms.amountDueNowCents }
func (terms BookingTerms) RemainingAmountDueCents() int64   { return terms.remainingAmountDueCents }
func (terms BookingTerms) ContractTotalCents() int64        { return terms.contractTotalCents }
func (terms BookingTerms) BookingPaymentDeadline() time.Time {
	return terms.bookingPaymentDeadline
}

func validate(
	currency string,
	serviceTotalCents,
	depositCents,
	platformFeeTotalCents,
	platformFeeDueNowCents int64,
	bookingPaymentDeadline time.Time,
) error {
	if serviceTotalCents <= 0 {
		return ErrInvalidAmount
	}
	if currency != bookingCurrencyARS {
		return fmt.Errorf("creating booking terms: unsupported currency %q", currency)
	}
	if depositCents <= 0 || depositCents > serviceTotalCents {
		return fmt.Errorf("creating booking terms: invalid deposit")
	}
	if platformFeeTotalCents < 0 {
		return fmt.Errorf("creating booking terms: invalid platform fee")
	}
	if serviceTotalCents > math.MaxInt64-platformFeeTotalCents {
		return fmt.Errorf("creating booking terms: total amount exceeds supported range")
	}
	if platformFeeDueNowCents < 0 || platformFeeDueNowCents > platformFeeTotalCents {
		return fmt.Errorf("creating booking terms: invalid platform fee due now")
	}
	if bookingPaymentDeadline.IsZero() {
		return fmt.Errorf("creating booking terms: booking payment deadline is required")
	}

	return nil
}
