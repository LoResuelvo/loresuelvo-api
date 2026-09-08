package evals

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaselinePoliciesAreFrozenAndComplete(t *testing.T) {
	policies := RankingBaselinePolicies()
	require.Len(t, policies, 5)
	for _, policy := range policies {
		require.Equal(t, "1", policy.Version)
		require.Equal(t, uint64(601), policy.Seed)
		require.Equal(t, 5, policy.PriorCount)
		require.Equal(t, 3.0, policy.PriorMean)
		require.NotEmpty(t, policy.LexicalRule)
		require.Equal(t, "reference_ascending", policy.TieBreak)
	}
	policies[0].Seed = 100
	require.Equal(t, uint64(601), RankingBaselinePolicies()[0].Seed)
}
func TestRatingAndPaidWorkBaselinesUseTheirOwnEvidence(t *testing.T) {
	input := RKInput{MaxResults: 1, Candidates: []CandidateInput{{Reference: "experienced", Evidence: EvidenceInput{RatingAverage: 4, RatingCount: 30, PaidWorkCount: 40}}, {Reference: "new", Evidence: EvidenceInput{RatingAverage: 5, RatingCount: 1, PaidWorkCount: 1}}}}
	for _, tc := range []struct{ policy, want string }{{"rating_average", "new"}, {"paid_work_count", "experienced"}, {"bayesian_rating_with_fixed_prior", "experienced"}} {
		t.Run(tc.policy, func(t *testing.T) {
			output, err := RankBaseline(input, tc.policy)
			require.NoError(t, err)
			require.Equal(t, tc.want, output.Recommendations[0].Reference)
		})
	}
}
func TestBaselineTiesIgnoreInputOrder(t *testing.T) {
	input := RKInput{MaxResults: 3, Candidates: []CandidateInput{{Reference: "c"}, {Reference: "a"}, {Reference: "b"}}}
	before := slices.Clone(input.Candidates)
	output, err := RankBaseline(input, "rating_average")
	require.NoError(t, err)
	require.Equal(t, "a", output.Recommendations[0].Reference)
	require.Equal(t, before, input.Candidates)
	slices.Reverse(input.Candidates)
	reversed, err := RankBaseline(input, "rating_average")
	require.NoError(t, err)
	require.Equal(t, output, reversed)
}
func TestRandomBaselineReproducesWithoutMutatingInput(t *testing.T) {
	input := RKInput{MaxResults: 3, Candidates: []CandidateInput{{Reference: "c"}, {Reference: "a"}, {Reference: "b"}, {Reference: "d"}}}
	first, err := RankBaseline(input, "seeded_random")
	require.NoError(t, err)
	slices.Reverse(input.Candidates)
	second, err := RankBaseline(input, "seeded_random")
	require.NoError(t, err)
	require.Equal(t, first, second)
}
func TestLexicalBaselineUsesAccentFoldedEvidenceOnly(t *testing.T) {
	input := RKInput{MaxResults: 1, ProblemTitle: "Conexión eléctrica", Candidates: []CandidateInput{{Reference: "a", Evidence: EvidenceInput{WorkHistory: []WorkInput{{Description: "pintura"}}}}, {Reference: "z", Evidence: EvidenceInput{WorkHistory: []WorkInput{{CompletionReport: &CompletionInput{Description: "CONEXION ELECTRICA"}}}}}}}
	output, err := RankBaseline(input, "frozen_lexical_heuristic")
	require.NoError(t, err)
	require.Equal(t, "z", output.Recommendations[0].Reference)
	require.Equal(t, 1.0, tokenJaccard(baselineTokens("CONEXIÓN"), baselineTokens("conexion")))
}
func TestBaselineRejectsUnknownPolicyAndInvalidCandidates(t *testing.T) {
	_, err := RankBaseline(RKInput{MaxResults: 3}, "oracle")
	require.ErrorIs(t, err, ErrInvalidBaseline)
	_, err = RankBaseline(RKInput{MaxResults: 3, Candidates: []CandidateInput{{Reference: "a"}, {Reference: "a"}}}, "rating_average")
	require.ErrorIs(t, err, ErrInvalidBaseline)
	_, err = RankBaseline(RKInput{MaxResults: 4}, "rating_average")
	require.ErrorIs(t, err, ErrInvalidBaseline)
}
func TestBaselineHasNoCandidateSuitabilityClaimOnEmptyEvidence(t *testing.T) {
	output, err := RankBaseline(RKInput{MaxResults: 3}, "frozen_lexical_heuristic")
	require.NoError(t, err)
	require.Empty(t, output.Recommendations)
}
func TestEvaluateBaselinesDeduplicatesCasesAndPreservesEvidence(t *testing.T) {
	dataset := rankingEvaluationFixture(t)
	dataset.RK[0].Input = RKInput{MaxResults: 3, Candidates: []CandidateInput{{Reference: "a", Evidence: EvidenceInput{RatingAverage: 5, RatingCount: 2}}, {Reference: "b", Evidence: EvidenceInput{RatingAverage: 4, RatingCount: 1}}, {Reference: "c"}}}
	report, err := EvaluateRankingBaselines(dataset, []string{"RK-test", "RK-test"})
	require.NoError(t, err)
	require.Equal(t, 1, report.BaseCases)
	require.Len(t, report.Policies, 5)
	require.Zero(t, report.ModelRequests)
	for _, policy := range report.Policies {
		require.Len(t, policy.Cases, 1)
		require.False(t, policy.Cases[0].Evaluation.ReleaseApproved)
		require.Equal(t, "unassessed", policy.Cases[0].Evaluation.SemanticChecks[0].Result)
		require.Equal(t, 1, policy.Metrics["ndcg_at_3"].EvaluatedBaseCases)
	}
}
