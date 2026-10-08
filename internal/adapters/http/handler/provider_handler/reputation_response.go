package provider_handler

import (
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type reputationRatingCountResponse struct {
	Rating int   `json:"rating"`
	Count  int64 `json:"count"`
}

type reputationReviewResponse struct {
	WorkOrderID int    `json:"work_order_id"`
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}

type reputationResponse struct {
	CalculatedAt       time.Time                        `json:"calculated_at"`
	AverageRating      *float64                         `json:"average_rating"`
	ReviewCount        int64                            `json:"review_count"`
	VisibleReviewCount int64                            `json:"visible_review_count"`
	RatingDistribution [5]reputationRatingCountResponse `json:"rating_distribution"`
	EligiblePaidOrders int64                            `json:"eligible_paid_orders"`
	ReviewedPaidOrders int64                            `json:"reviewed_paid_orders"`
	CoveragePercentage *float64                         `json:"coverage_percentage"`
	Reviews            []reputationReviewResponse       `json:"reviews"`
	NextCursor         *string                          `json:"next_cursor"`
}

func reputationResponseFromDomain(result *readmodel.Reputation) reputationResponse {
	response := reputationResponse{
		CalculatedAt:       result.CalculatedAt.UTC(),
		AverageRating:      result.AverageRating,
		ReviewCount:        result.ReviewedPaidOrders,
		VisibleReviewCount: result.VisibleReviews,
		EligiblePaidOrders: result.EligiblePaidOrders,
		ReviewedPaidOrders: result.ReviewedPaidOrders,
		CoveragePercentage: result.CoveragePercentage,
		Reviews:            make([]reputationReviewResponse, 0, len(result.Reviews)),
	}
	for index, count := range result.RatingDistribution {
		response.RatingDistribution[index] = reputationRatingCountResponse{Rating: index + 1, Count: count}
	}
	for _, review := range result.Reviews {
		response.Reviews = append(response.Reviews, reputationReviewResponse{WorkOrderID: review.WorkOrderID, Rating: review.Rating, Description: review.Description})
	}
	return response
}
