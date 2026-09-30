package provider

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	read_model "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

var ErrActivityForbidden = errors.New("activity is only available to providers")
var ErrActivityProviderNotFound = errors.New("activity provider not found")

type ProviderActorFinder interface {
	FindByAuthID(ctx context.Context, authID string) (providerID int, role string, err error)
}

// ActivityReader obtains all facts from one coherent read-only snapshot.
type ActivityReader interface {
	Read(ctx context.Context, providerID int, query ActivityQuery) (*read_model.ActivitySnapshot, error)
}

type ActivityService struct {
	reader ActivityReader
	actors ProviderActorFinder
	clock  clock.Clock
}

func NewActivityService(reader ActivityReader, actors ProviderActorFinder, clock clock.Clock) *ActivityService {
	return &ActivityService{reader: reader, actors: actors, clock: clock}
}

func (s *ActivityService) Query(ctx context.Context, authID string, input ActivityQueryInput) (*read_model.Activity, error) {
	now := s.clock.Now().UTC()
	query, err := input.resolve(now)
	if err != nil {
		return nil, err
	}
	providerID, role, err := s.actors.FindByAuthID(ctx, authID)
	if errors.Is(err, user.ErrNotFound) {
		return nil, ErrActivityProviderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolving activity provider: %w", err)
	}
	if role != Role {
		return nil, ErrActivityForbidden
	}
	if providerID <= 0 {
		return nil, ErrActivityProviderNotFound
	}
	snapshot, err := s.reader.Read(ctx, providerID, query)
	if err != nil {
		return nil, fmt.Errorf("reading provider activity: %w", err)
	}
	if snapshot == nil || (query.ComparePrevious && snapshot.Previous == nil) {
		return nil, errors.New("activity reader returned incomplete snapshot")
	}
	location, err := time.LoadLocation(activityTimeZone)
	if err != nil {
		return nil, fmt.Errorf("loading activity timezone: %w", err)
	}
	series, err := activitySeries(query, snapshot.Series, snapshot.Current, location)
	if err != nil {
		return nil, fmt.Errorf("reading provider activity: %w", err)
	}

	period := read_model.ActivityPeriod{From: query.From, To: query.To}
	activity := &read_model.Activity{
		Period:       period,
		Granularity:  string(query.Granularity),
		TimeZone:     activityTimeZone,
		CalculatedAt: now,
		Results:      activityResults(snapshot.Current),
		Series:       series,
		Pending:      snapshot.Pending,
	}
	if query.ComparePrevious {
		previous := activityResults(*snapshot.Previous)
		activity.Comparison = &read_model.ActivityComparison{
			Period:        query.PreviousPeriod(),
			Results:       previous,
			Change:        activityChange(activity.Results, previous),
			PercentChange: activityPercentChange(activity.Results, previous),
		}
	}
	return activity, nil
}

func activityResults(counts read_model.ActivityCounts) read_model.ActivityResults {
	result := read_model.ActivityResults{ActivityCounts: counts}
	if counts.Reported > 0 {
		average := counts.ContractValueCents / counts.Reported
		if counts.ContractValueCents%counts.Reported >= counts.Reported/2+counts.Reported%2 {
			average++
		}
		result.AverageCents = &average
	}
	return result
}

func activitySeries(query ActivityQuery, sparse []read_model.ActivityBucket, counts read_model.ActivityCounts, location *time.Location) ([]read_model.ActivityBucket, error) {
	if counts.Confirmed < 0 || counts.Reported < 0 || counts.Paid < 0 {
		return nil, errors.New("activity reader returned negative totals")
	}
	byStart := make(map[time.Time]read_model.ActivityBucket, len(sparse))
	first := activityBucketStart(query.From, query.Granularity, location)
	for _, bucket := range sparse {
		start := bucket.From.UTC()
		if start.Before(first) || !start.Before(query.To) || !start.Equal(activityBucketStart(start, query.Granularity, location)) || bucket.Confirmed < 0 || bucket.Reported < 0 || bucket.Paid < 0 {
			return nil, errors.New("activity reader returned invalid series bucket")
		}
		if _, duplicate := byStart[start]; duplicate {
			return nil, errors.New("activity reader returned duplicate series bucket")
		}
		byStart[start] = bucket
	}
	series := make([]read_model.ActivityBucket, 0)
	var confirmed, reported, paid int64
	for start := first; start.Before(query.To); start = nextActivityBucket(start, query.Granularity, location) {
		bucket := byStart[start]
		if bucket.Confirmed > counts.Confirmed-confirmed || bucket.Reported > counts.Reported-reported || bucket.Paid > counts.Paid-paid {
			return nil, errors.New("activity reader returned series exceeding totals")
		}
		confirmed += bucket.Confirmed
		reported += bucket.Reported
		paid += bucket.Paid
		bucket.From = start
		bucket.To = nextActivityBucket(start, query.Granularity, location)
		if bucket.From.Before(query.From) {
			bucket.From = query.From
		}
		if bucket.To.After(query.To) {
			bucket.To = query.To
		}
		series = append(series, bucket)
	}
	if confirmed != counts.Confirmed || reported != counts.Reported || paid != counts.Paid {
		return nil, errors.New("activity reader returned series inconsistent with totals")
	}
	return series, nil
}

func activityChange(current, previous read_model.ActivityResults) read_model.ActivityChange {
	change := read_model.ActivityChange{
		Confirmed:          current.Confirmed - previous.Confirmed,
		Reported:           current.Reported - previous.Reported,
		Paid:               current.Paid - previous.Paid,
		Customers:          current.Customers - previous.Customers,
		NewCustomers:       current.NewCustomers - previous.NewCustomers,
		ReturningCustomers: current.ReturningCustomers - previous.ReturningCustomers,
		ContractValueCents: current.ContractValueCents - previous.ContractValueCents,
	}
	if current.AverageCents != nil && previous.AverageCents != nil {
		average := *current.AverageCents - *previous.AverageCents
		change.AverageCents = &average
	}
	return change
}

func activityPercentChange(current, previous read_model.ActivityResults) read_model.ActivityPercentChange {
	change := read_model.ActivityPercentChange{
		Confirmed:          percentChange(current.Confirmed, previous.Confirmed),
		Reported:           percentChange(current.Reported, previous.Reported),
		Paid:               percentChange(current.Paid, previous.Paid),
		Customers:          percentChange(current.Customers, previous.Customers),
		NewCustomers:       percentChange(current.NewCustomers, previous.NewCustomers),
		ReturningCustomers: percentChange(current.ReturningCustomers, previous.ReturningCustomers),
		ContractValueCents: percentChange(current.ContractValueCents, previous.ContractValueCents),
	}
	if current.AverageCents != nil && previous.AverageCents != nil {
		change.AverageCents = percentChange(*current.AverageCents, *previous.AverageCents)
	}
	return change
}

// percentChange rounds to two decimals, halves away from zero. Arithmetic is
// performed with integers to avoid float drift and intermediate overflow.
func percentChange(current, previous int64) *float64 {
	if previous == 0 {
		return nil
	}
	delta := new(big.Int).Sub(big.NewInt(current), big.NewInt(previous))
	negative := delta.Sign() < 0
	delta.Abs(delta).Mul(delta, big.NewInt(10000))
	base := big.NewInt(previous)
	base.Abs(base)
	quotient, remainder := new(big.Int).QuoRem(delta, base, new(big.Int))
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(base) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	value, _ := new(big.Rat).SetFrac(quotient, big.NewInt(100)).Float64()
	if negative {
		value = -value
	}
	return &value
}
