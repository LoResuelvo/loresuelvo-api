package readmodel

import (
	"errors"
	"math/big"
	"slices"
	"time"
)

type ReputationPosition struct {
	WorkOrderID int
}

type ReputationReview struct {
	WorkOrderID int
	Rating      int
	Description string
}

// ReputationSnapshot contains global persisted facts and one page from the same snapshot.
type ReputationSnapshot struct {
	EligiblePaidOrders int64
	ReviewedPaidOrders int64
	VisibleReviews     int64
	RatingDistribution [5]int64
	Reviews            []ReputationReview
	Next               *ReputationPosition
}

type Reputation struct {
	CalculatedAt       time.Time
	AverageRating      *float64
	CoveragePercentage *float64
	EligiblePaidOrders int64
	ReviewedPaidOrders int64
	VisibleReviews     int64
	RatingDistribution [5]int64
	Reviews            []ReputationReview
	Next               *ReputationPosition
}

// Calculate validates the persisted facts before deriving the global reputation.
// afterID is zero for the first page, otherwise the exclusive order-ID bound.
func (s ReputationSnapshot) Calculate(now time.Time, limit, afterID int) (*Reputation, error) {
	if err := s.validate(limit, afterID); err != nil {
		return nil, err
	}
	result := &Reputation{
		CalculatedAt: now.UTC(), EligiblePaidOrders: s.EligiblePaidOrders,
		ReviewedPaidOrders: s.ReviewedPaidOrders, VisibleReviews: s.VisibleReviews, RatingDistribution: s.RatingDistribution,
		Reviews: slices.Clone(s.Reviews),
	}
	if result.Reviews == nil {
		result.Reviews = make([]ReputationReview, 0)
	}
	if s.Next != nil {
		position := *s.Next
		result.Next = &position
	}
	if s.ReviewedPaidOrders > 0 {
		weighted := new(big.Int)
		for index, count := range s.RatingDistribution {
			weighted.Add(weighted, new(big.Int).Mul(big.NewInt(count), big.NewInt(int64(index+1))))
		}
		result.AverageRating = reputationRatio(weighted, s.ReviewedPaidOrders, 100)
	}
	if s.EligiblePaidOrders > 0 {
		result.CoveragePercentage = reputationRatio(big.NewInt(s.ReviewedPaidOrders), s.EligiblePaidOrders, 10000)
	}
	return result, nil
}

func (s ReputationSnapshot) validate(limit, afterID int) error {
	if limit <= 0 || afterID < 0 || s.EligiblePaidOrders < 0 || s.ReviewedPaidOrders < 0 || s.ReviewedPaidOrders > s.EligiblePaidOrders ||
		s.VisibleReviews < 0 || s.VisibleReviews > s.ReviewedPaidOrders {
		return errors.New("reputation snapshot contains invalid totals")
	}
	var counted int64
	for _, count := range s.RatingDistribution {
		// Subtraction against the remaining total also prevents addition overflow.
		if count < 0 || count > s.ReviewedPaidOrders-counted {
			return errors.New("reputation distribution exceeds reviewed orders")
		}
		counted += count
	}
	if counted != s.ReviewedPaidOrders {
		return errors.New("reputation distribution differs from reviewed orders")
	}
	if len(s.Reviews) > limit || int64(len(s.Reviews)) > s.VisibleReviews {
		return errors.New("reputation page exceeds its limit or global total")
	}
	if afterID == 0 && int64(len(s.Reviews)) != min(s.VisibleReviews, int64(limit)) {
		return errors.New("reputation snapshot has an incomplete first page")
	}
	var pageDistribution [5]int64
	previousID := afterID
	for _, review := range s.Reviews {
		if review.WorkOrderID <= 0 || review.Rating < 1 || review.Rating > 5 || (previousID != 0 && review.WorkOrderID >= previousID) {
			return errors.New("reputation page contains an invalid or unordered review")
		}
		previousID = review.WorkOrderID
		pageDistribution[review.Rating-1]++
		if pageDistribution[review.Rating-1] > s.RatingDistribution[review.Rating-1] {
			return errors.New("reputation page distribution exceeds global distribution")
		}
	}
	if s.Next != nil && (len(s.Reviews) != limit || s.Next.WorkOrderID != previousID || s.Next.WorkOrderID <= 0 || s.VisibleReviews <= int64(len(s.Reviews))) {
		return errors.New("reputation snapshot has an invalid continuation position")
	}
	if afterID == 0 && s.VisibleReviews > int64(limit) && s.Next == nil {
		return errors.New("reputation snapshot is missing its continuation position")
	}
	return nil
}

// reputationRatio rounds a nonnegative ratio to two decimals, with exact HALF_UP
// arithmetic even when weighted ratings or percentage numerators exceed int64.
func reputationRatio(numerator *big.Int, denominator, scale int64) *float64 {
	numerator = new(big.Int).Mul(numerator, big.NewInt(scale))
	base := big.NewInt(denominator)
	quotient, remainder := new(big.Int).QuoRem(numerator, base, new(big.Int))
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(base) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	value, _ := new(big.Rat).SetFrac(quotient, big.NewInt(100)).Float64()
	return &value
}
