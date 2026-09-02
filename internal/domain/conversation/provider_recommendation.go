package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/google/uuid"
)

const defaultMaxRecommendedProviders = 3

type ProviderRecommendationConfig struct {
	MaxRecommendedProviders int
}

func DefaultProviderRecommendationConfig() ProviderRecommendationConfig {
	return ProviderRecommendationConfig{MaxRecommendedProviders: defaultMaxRecommendedProviders}
}

func (config ProviderRecommendationConfig) Validate() error {
	if config.MaxRecommendedProviders <= 0 {
		return fmt.Errorf("max recommended providers must be positive")
	}
	return nil
}

type ProviderRecommendationCandidate struct {
	Reference string
	// ProviderID is kept for service-side correlation and is never sent to the ranker wire payload.
	ProviderID int
	Evidence   ProviderRecommendationEvidence
}

type ProviderRankingRequest struct {
	ProblemTitle       string
	ProblemDescription string
	MaxResults         int
	Candidates         []ProviderRecommendationCandidate
}

// ProviderRankingRecommendation is one item returned by the AI ranker before
// the service correlates it with a persisted provider.
type ProviderRankingRecommendation struct {
	Reference string
	Reason    string
}

type ProviderRankingResponse struct {
	Recommendations []ProviderRankingRecommendation
}

type WorkOrderReader interface {
	FindRatingStatsByProviderIDs(ctx context.Context, providerIDs []int) (map[int]provider.RatingStats, error)
	FindPaidWorkHistoryByProviderIDs(ctx context.Context, providerIDs []int) (map[int][]providerreadmodel.WorkOrder, error)
}

// ProviderRecommendation is one item in the current persisted ranking.
type ProviderRecommendation struct {
	ProviderID int
	Position   int
	Reason     string
}

type CurrentProviderRecommendation struct {
	AssessmentID         int
	CandidateProviderIDs []int
	Recommendations      []ProviderRecommendation
}

type ProviderRecommendationEvidence struct {
	RatingAverage      float64
	RatingCount        int
	RatingDistribution provider.RatingDistribution
	PaidWorkCount      int
	MostRecentPaidWork time.Time
	WorkHistory        []providerreadmodel.WorkOrder
}

func (evidence ProviderRecommendationEvidence) IsEmpty() bool {
	return evidence.RatingCount == 0 && evidence.PaidWorkCount == 0 && len(evidence.WorkHistory) == 0
}

func NewCurrentProviderRecommendation(assessmentID int, candidateProviderIDs []int, recommendations []ProviderRecommendation, maxResults int) (*CurrentProviderRecommendation, error) {
	if assessmentID < 0 || maxResults <= 0 {
		return nil, ErrProviderRecommendationInvalid
	}

	seenCandidates := make(map[int]struct{}, len(candidateProviderIDs))
	for _, providerID := range candidateProviderIDs {
		if providerID <= 0 {
			return nil, ErrProviderRecommendationInvalid
		}
		if _, exists := seenCandidates[providerID]; exists {
			return nil, ErrProviderRecommendationInvalid
		}
		seenCandidates[providerID] = struct{}{}
	}
	if len(recommendations) > maxResults {
		return nil, ErrProviderRecommendationInvalid
	}

	seenRecommendations := make(map[int]struct{}, len(recommendations))
	for index, recommendation := range recommendations {
		if recommendation.ProviderID <= 0 || recommendation.Position != index+1 || strings.TrimSpace(recommendation.Reason) == "" {
			return nil, ErrProviderRecommendationInvalid
		}
		if _, candidate := seenCandidates[recommendation.ProviderID]; !candidate {
			return nil, ErrProviderRecommendationInvalid
		}
		if _, duplicate := seenRecommendations[recommendation.ProviderID]; duplicate {
			return nil, ErrProviderRecommendationInvalid
		}
		seenRecommendations[recommendation.ProviderID] = struct{}{}
		recommendation.Reason = strings.TrimSpace(recommendation.Reason)
		recommendations[index] = recommendation
	}

	return &CurrentProviderRecommendation{
		AssessmentID:         assessmentID,
		CandidateProviderIDs: append([]int(nil), candidateProviderIDs...),
		Recommendations:      append([]ProviderRecommendation(nil), recommendations...),
	}, nil
}

func (recommendation *CurrentProviderRecommendation) Copy() *CurrentProviderRecommendation {
	if recommendation == nil {
		return nil
	}
	copied := *recommendation
	copied.CandidateProviderIDs = append([]int(nil), recommendation.CandidateProviderIDs...)
	copied.Recommendations = append([]ProviderRecommendation(nil), recommendation.Recommendations...)
	return &copied
}

func newProviderRecommendationReference() string {
	return "candidate-" + uuid.NewString()
}
