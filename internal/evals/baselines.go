package evals

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var ErrInvalidBaseline = errors.New("invalid ranking baseline")

// BaselinePolicy freezes the implementation parameters, not a fitted model.
// These rules never inspect labels, expected relevance, split or case metadata.
type BaselinePolicy struct {
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Seed        uint64  `json:"seed"`
	PriorCount  int     `json:"prior_count"`
	PriorMean   float64 `json:"prior_mean"`
	TieBreak    string  `json:"tie_break"`
	LexicalRule string  `json:"lexical_rule"`
}

func RankingBaselinePolicies() []BaselinePolicy {
	names := []string{"seeded_random", "rating_average", "bayesian_rating_with_fixed_prior", "paid_work_count", "frozen_lexical_heuristic"}
	policies := make([]BaselinePolicy, 0, len(names))
	for _, name := range names {
		policies = append(policies, BaselinePolicy{Name: name, Version: "1", Seed: 601, PriorCount: 5, PriorMean: 3, TieBreak: "reference_ascending", LexicalRule: "Jaccard of unique Unicode letter/digit tokens after lowercase and accent folding; query=problem title+description; document=all work descriptions, completion reports and reviews; no stopwords, stemming, weights or relevance labels"})
	}
	return policies
}

type BaselineRecommendation struct {
	Reference string `json:"reference"`
	Reason    string `json:"reason"`
}
type BaselineOutput struct {
	Recommendations []BaselineRecommendation `json:"recommendations"`
}

// RankBaseline accepts model-facing evidence only. Selection and evaluation are
// separate so neither a prior nor a tie-break can depend on expected relevance.
func RankBaseline(input RKInput, name string) (BaselineOutput, error) {
	output := BaselineOutput{Recommendations: []BaselineRecommendation{}}
	var policy *BaselinePolicy
	for _, candidate := range RankingBaselinePolicies() {
		if candidate.Name == name {
			policy = &candidate
			break
		}
	}
	if policy == nil {
		return output, fmt.Errorf("%w: unknown policy %q", ErrInvalidBaseline, name)
	}
	if input.MaxResults < 1 || input.MaxResults > 3 {
		return output, fmt.Errorf("%w: max_results must be between 1 and 3", ErrInvalidBaseline)
	}
	candidates := slices.Clone(input.Candidates)
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		e := candidate.Evidence
		if strings.TrimSpace(candidate.Reference) == "" || seen[candidate.Reference] {
			return output, fmt.Errorf("%w: duplicate or empty reference", ErrInvalidBaseline)
		}
		if math.IsNaN(e.RatingAverage) || math.IsInf(e.RatingAverage, 0) || e.RatingAverage < 0 || e.RatingAverage > 5 || e.RatingCount < 0 || e.PaidWorkCount < 0 {
			return output, fmt.Errorf("%w: invalid rating or work count", ErrInvalidBaseline)
		}
		seen[candidate.Reference] = true
	}
	slices.SortFunc(candidates, func(a, b CandidateInput) int { return strings.Compare(a.Reference, b.Reference) })
	query := baselineTokens(input.ProblemTitle + " " + input.ProblemDescription)
	scores := make(map[string]float64, len(candidates))
	for _, candidate := range candidates {
		e := candidate.Evidence
		switch name {
		case "rating_average":
			scores[candidate.Reference] = e.RatingAverage
		case "paid_work_count":
			scores[candidate.Reference] = float64(e.PaidWorkCount)
		case "bayesian_rating_with_fixed_prior":
			scores[candidate.Reference] = (float64(e.RatingCount)*e.RatingAverage + float64(policy.PriorCount)*policy.PriorMean) / (float64(e.RatingCount) + float64(policy.PriorCount))
		case "frozen_lexical_heuristic":
			var text strings.Builder
			for _, work := range e.WorkHistory {
				text.WriteString(" " + work.Description)
				if work.CompletionReport != nil {
					text.WriteString(" " + work.CompletionReport.Description)
				}
				if work.Review != nil {
					text.WriteString(" " + work.Review.Description)
				}
			}
			scores[candidate.Reference] = tokenJaccard(query, baselineTokens(text.String()))
		}
	}
	if name == "seeded_random" {
		rng := rand.New(rand.NewPCG(policy.Seed, policy.Seed))
		rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	} else {
		slices.SortStableFunc(candidates, func(a, b CandidateInput) int { return cmp.Compare(scores[b.Reference], scores[a.Reference]) })
	}
	for _, candidate := range candidates[:min(input.MaxResults, len(candidates))] {
		reason := fmt.Sprintf("Offline %s score %.6f; tie-break reference ascending.", name, scores[candidate.Reference])
		if name == "seeded_random" {
			reason = fmt.Sprintf("Offline random ordering with fixed seed %d; no claim of provider suitability.", policy.Seed)
		}
		output.Recommendations = append(output.Recommendations, BaselineRecommendation{Reference: candidate.Reference, Reason: reason})
	}
	return output, nil
}
func baselineTokens(text string) map[string]bool {
	folded := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return unicode.ToLower(r)
	}, norm.NFD.String(text))
	result := map[string]bool{}
	for _, token := range strings.FieldsFunc(folded, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		result[token] = true
	}
	return result
}
func tokenJaccard(left, right map[string]bool) float64 {
	overlap := 0
	for token := range left {
		if right[token] {
			overlap++
		}
	}
	union := len(left) + len(right) - overlap
	if union == 0 {
		return 0
	}
	return float64(overlap) / float64(union)
}

type BaselineCaseResult struct {
	CaseID     string         `json:"case_id"`
	FamilyID   string         `json:"family_id"`
	Split      string         `json:"split"`
	Output     BaselineOutput `json:"output"`
	Evaluation Evaluation     `json:"evaluation"`
}
type BaselinePolicyReport struct {
	Policy  BaselinePolicy           `json:"policy"`
	Cases   []BaselineCaseResult     `json:"cases"`
	Metrics map[string]MetricSummary `json:"metrics"`
}
type BaselineReport struct {
	DatasetVersion string                 `json:"dataset_version"`
	DatasetSHA256  string                 `json:"dataset_manifest_sha256"`
	BaseCases      int                    `json:"ranking_base_cases"`
	ModelRequests  int                    `json:"model_requests"`
	Policies       []BaselinePolicyReport `json:"policies"`
	Warning        string                 `json:"warning"`
}

// EvaluateRankingBaselines evaluates each selected RK base case once per policy.
// PD IDs may accompany a mixed suite and are skipped; unknown IDs fail closed.
func EvaluateRankingBaselines(dataset *Dataset, caseIDs []string) (BaselineReport, error) {
	report := BaselineReport{Policies: []BaselinePolicyReport{}, Warning: "Non-LLM baselines; five policies are paired on the same base cases, not independent observations. Semantic checks remain unassessed."}
	if dataset == nil {
		return report, fmt.Errorf("%w: missing dataset", ErrInvalidBaseline)
	}
	report.DatasetVersion, report.DatasetSHA256 = dataset.Version, dataset.ManifestSHA256
	ranking := map[string]RKCase{}
	known := map[string]bool{}
	for _, c := range dataset.RK {
		ranking[c.ID] = c
		known[c.ID] = true
	}
	for _, c := range dataset.PD {
		known[c.ID] = true
	}
	selected := []RKCase{}
	seen := map[string]bool{}
	for _, id := range caseIDs {
		if !known[id] {
			return report, fmt.Errorf("%w: unknown case %s", ErrInvalidBaseline, id)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		if c, ok := ranking[id]; ok {
			selected = append(selected, c)
		}
	}
	report.BaseCases = len(selected)
	for _, policy := range RankingBaselinePolicies() {
		item := BaselinePolicyReport{Policy: policy, Cases: []BaselineCaseResult{}, Metrics: map[string]MetricSummary{}}
		observations := map[string]map[string][]float64{}
		for _, c := range selected {
			output, err := RankBaseline(c.Input, policy.Name)
			if err != nil {
				return report, fmt.Errorf("rank %s: %w", c.ID, err)
			}
			raw, err := json.Marshal(output)
			if err != nil {
				return report, fmt.Errorf("encode baseline %s: %w", c.ID, err)
			}
			evaluation, err := EvaluateCase(dataset, c.ID, raw)
			if err != nil {
				return report, err
			}
			item.Cases = append(item.Cases, BaselineCaseResult{CaseID: c.ID, FamilyID: c.FamilyID, Split: c.Split, Output: output, Evaluation: evaluation})
			collectRankingMetrics(observations, c.ID, evaluation)
		}
		for _, name := range rankingMetricNames() {
			item.Metrics[name] = summarizeMetric(observations[name], len(selected), len(selected))
		}
		report.Policies = append(report.Policies, item)
	}
	return report, nil
}
