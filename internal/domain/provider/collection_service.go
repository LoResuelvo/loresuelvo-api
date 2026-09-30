package provider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

var ErrInvalidCollectionQuery = errors.New("invalid collection query")

const DefaultCollectionPageSize = 20
const MaxCollectionPageSize = 100

type CollectionPurpose string

const (
	CollectionBookingDeposit CollectionPurpose = "booking_deposit"
	CollectionServiceBalance CollectionPurpose = "service_balance"
)

type CollectionDetailInput struct {
	Period           ActivityQueryInput
	Purpose          string
	Limit            int
	After            *readmodel.CollectionPosition
	CursorProviderID int
}

type CollectionDetailQuery struct {
	Period  ActivityQuery
	Purpose CollectionPurpose
	Limit   int
	After   *readmodel.CollectionPosition
}

type CollectionReader interface {
	ReadSummary(ctx context.Context, providerID int, query ActivityQuery) (*readmodel.CollectionSnapshot, error)
	ReadDetail(ctx context.Context, providerID int, query CollectionDetailQuery) (*readmodel.CollectionDetailSnapshot, error)
}

type CollectionService struct {
	reader CollectionReader
	actors ProviderActorFinder
	clock  clock.Clock
}

func NewCollectionService(reader CollectionReader, actors ProviderActorFinder, clock clock.Clock) *CollectionService {
	return &CollectionService{reader: reader, actors: actors, clock: clock}
}

func (s *CollectionService) Summary(ctx context.Context, authID string, input ActivityQueryInput) (*readmodel.Collections, error) {
	now := s.clock.Now().UTC()
	query, err := input.resolve(now)
	if err != nil {
		return nil, ErrInvalidCollectionQuery
	}
	providerID, err := s.providerID(ctx, authID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.reader.ReadSummary(ctx, providerID, query)
	if err != nil {
		return nil, fmt.Errorf("reading provider collections: %w", err)
	}
	if snapshot == nil || (query.ComparePrevious && snapshot.Previous == nil) {
		return nil, errors.New("collection reader returned incomplete snapshot")
	}
	location, err := time.LoadLocation(activityTimeZone)
	if err != nil {
		return nil, fmt.Errorf("loading collection timezone: %w", err)
	}
	current, err := completeCollectionAmounts(snapshot.Current)
	if err != nil {
		return nil, err
	}
	series, err := collectionSeries(query, snapshot.Series, current, location)
	if err != nil {
		return nil, err
	}
	if err := validateCollectionPending(snapshot.Pending); err != nil {
		return nil, err
	}
	result := &readmodel.Collections{
		Period:       readmodel.ActivityPeriod{From: query.From, To: query.To},
		Granularity:  string(query.Granularity),
		TimeZone:     activityTimeZone,
		CalculatedAt: now,
		Results:      current,
		Series:       series,
		Pending:      snapshot.Pending,
	}
	if query.ComparePrevious {
		previous, err := completeCollectionAmounts(*snapshot.Previous)
		if err != nil {
			return nil, err
		}
		changes := readmodel.CollectionChanges{
			Absolute: readmodel.CollectionAmounts{
				BookingDepositCents: current.BookingDepositCents - previous.BookingDepositCents,
				ServiceBalanceCents: current.ServiceBalanceCents - previous.ServiceBalanceCents,
				TotalCents:          current.TotalCents - previous.TotalCents,
			},
		}
		changes.Percentage.BookingDepositCents = percentChange(current.BookingDepositCents, previous.BookingDepositCents)
		changes.Percentage.ServiceBalanceCents = percentChange(current.ServiceBalanceCents, previous.ServiceBalanceCents)
		changes.Percentage.TotalCents = percentChange(current.TotalCents, previous.TotalCents)
		result.Comparison = &readmodel.CollectionComparison{Period: query.PreviousPeriod(), Results: previous, Changes: changes}
	}
	return result, nil
}

func (s *CollectionService) Detail(ctx context.Context, authID string, input CollectionDetailInput) (*readmodel.CollectionDetail, int, error) {
	now := s.clock.Now().UTC()
	query, err := input.Period.resolve(now)
	if err != nil || input.Period.ComparePrevious || input.Period.Granularity != "" {
		return nil, 0, ErrInvalidCollectionQuery
	}
	if input.Purpose != "" && input.Purpose != string(CollectionBookingDeposit) && input.Purpose != string(CollectionServiceBalance) {
		return nil, 0, ErrInvalidCollectionQuery
	}
	limit := input.Limit
	if limit == 0 {
		limit = DefaultCollectionPageSize
	}
	if limit < 1 || limit > MaxCollectionPageSize || input.CursorProviderID < 0 {
		return nil, 0, ErrInvalidCollectionQuery
	}
	if input.After != nil && (input.After.ID <= 0 || input.After.VerifiedOn.IsZero() || input.After.VerifiedOn.Before(query.From) || !input.After.VerifiedOn.Before(query.To) || input.CursorProviderID <= 0) {
		return nil, 0, ErrInvalidCollectionQuery
	}
	providerID, err := s.providerID(ctx, authID)
	if err != nil {
		return nil, 0, err
	}
	if input.CursorProviderID != 0 && input.CursorProviderID != providerID {
		return nil, 0, ErrInvalidCollectionQuery
	}
	page, err := s.reader.ReadDetail(ctx, providerID, CollectionDetailQuery{Period: query, Purpose: CollectionPurpose(input.Purpose), Limit: limit, After: input.After})
	if err != nil {
		return nil, 0, fmt.Errorf("reading provider collection transactions: %w", err)
	}
	if page == nil || page.TotalCount < 0 || page.TotalAmountCents < 0 || len(page.Page) > limit {
		return nil, 0, errors.New("collection reader returned invalid detail")
	}
	for _, transaction := range page.Page {
		if transaction.ID <= 0 || transaction.VerifiedOn.Before(query.From) || !transaction.VerifiedOn.Before(query.To) || transaction.SellerAmountCents <= 0 || transaction.Currency != "ARS" || (transaction.Purpose != string(CollectionBookingDeposit) && transaction.Purpose != string(CollectionServiceBalance)) || transaction.ServiceProposalID <= 0 {
			return nil, 0, errors.New("collection reader returned invalid transaction")
		}
	}
	return &readmodel.CollectionDetail{
		Period:           readmodel.ActivityPeriod{From: query.From, To: query.To},
		TimeZone:         activityTimeZone,
		CalculatedAt:     now,
		TotalCount:       page.TotalCount,
		TotalAmountCents: page.TotalAmountCents,
		Transactions:     page.Page,
		Next:             page.Next,
	}, providerID, nil
}

func (s *CollectionService) providerID(ctx context.Context, authID string) (int, error) {
	providerID, role, err := s.actors.FindByAuthID(ctx, authID)
	if errors.Is(err, user.ErrNotFound) {
		return 0, ErrActivityProviderNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("resolving collection provider: %w", err)
	}
	if role != Role {
		return 0, ErrActivityForbidden
	}
	if providerID <= 0 {
		return 0, ErrActivityProviderNotFound
	}
	return providerID, nil
}

func completeCollectionAmounts(amounts readmodel.CollectionAmounts) (readmodel.CollectionAmounts, error) {
	if amounts.BookingDepositCents < 0 || amounts.ServiceBalanceCents < 0 || amounts.BookingDepositCents > math.MaxInt64-amounts.ServiceBalanceCents {
		return readmodel.CollectionAmounts{}, errors.New("collection amounts exceed the supported range")
	}
	amounts.TotalCents = amounts.BookingDepositCents + amounts.ServiceBalanceCents
	return amounts, nil
}

func validateCollectionPending(pending readmodel.CollectionPending) error {
	if pending.Scheduled.Orders < 0 || pending.Scheduled.AmountCents < 0 || pending.AwaitingPayment.Orders < 0 || pending.AwaitingPayment.AmountCents < 0 {
		return errors.New("collection reader returned invalid pending balances")
	}
	return nil
}

func collectionSeries(query ActivityQuery, sparse []readmodel.CollectionBucket, totals readmodel.CollectionAmounts, location *time.Location) ([]readmodel.CollectionBucket, error) {
	byStart := make(map[time.Time]readmodel.CollectionBucket, len(sparse))
	first := activityBucketStart(query.From, query.Granularity, location)
	for _, bucket := range sparse {
		start := bucket.From.UTC()
		if start.Before(first) || !start.Before(query.To) || !start.Equal(activityBucketStart(start, query.Granularity, location)) {
			return nil, errors.New("collection reader returned invalid series bucket")
		}
		if _, duplicate := byStart[start]; duplicate {
			return nil, errors.New("collection reader returned duplicate series bucket")
		}
		amounts, err := completeCollectionAmounts(bucket.CollectionAmounts)
		if err != nil {
			return nil, errors.New("collection reader returned invalid series amounts")
		}
		bucket.CollectionAmounts = amounts
		byStart[start] = bucket
	}
	series := make([]readmodel.CollectionBucket, 0)
	var deposits, balances int64
	for start := first; start.Before(query.To); start = nextActivityBucket(start, query.Granularity, location) {
		bucket := byStart[start]
		if bucket.BookingDepositCents > totals.BookingDepositCents-deposits || bucket.ServiceBalanceCents > totals.ServiceBalanceCents-balances {
			return nil, errors.New("collection series exceeds totals")
		}
		deposits += bucket.BookingDepositCents
		balances += bucket.ServiceBalanceCents
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
	if deposits != totals.BookingDepositCents || balances != totals.ServiceBalanceCents {
		return nil, errors.New("collection series is inconsistent with totals")
	}
	return series, nil
}
