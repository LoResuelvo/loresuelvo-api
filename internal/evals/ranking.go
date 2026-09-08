package evals

import (
	"math"
	"slices"
)

type PairwiseConstraint struct {
	Higher string `json:"higher"`
	Lower  string `json:"lower"`
}
type PairwiseMetrics struct {
	Passed       int      `json:"passed"`
	Failed       int      `json:"failed"`
	Unassessed   int      `json:"unassessed"`
	Satisfaction *float64 `json:"satisfaction"`
	Coverage     *float64 `json:"coverage"`
}

func rankingNDCG(refs []string, relevance map[string]int) *float64 {
	grades := make([]int, 0, len(relevance))
	for _, grade := range relevance {
		grades = append(grades, grade)
	}
	slices.SortFunc(grades, func(a, b int) int { return b - a })
	dcg := func(values []int) float64 {
		sum := 0.0
		for i, g := range values[:min(3, len(values))] {
			sum += (math.Exp2(float64(g)) - 1) / math.Log2(float64(i+2))
		}
		return sum
	}
	ideal := dcg(grades)
	if ideal == 0 {
		return nil
	}
	actual := make([]int, len(refs))
	for i, r := range refs {
		actual[i] = relevance[r]
	}
	value := dcg(actual) / ideal
	return &value
}
func rankingPairwise(refs []string, constraints []PairwiseConstraint) PairwiseMetrics {
	positions := make(map[string]int, len(refs))
	for i, r := range refs {
		positions[r] = i
	}
	var result PairwiseMetrics
	for _, pair := range constraints {
		a, hasA := positions[pair.Higher]
		b, hasB := positions[pair.Lower]
		switch {
		case !hasA && !hasB:
			result.Unassessed++
		case hasA && (!hasB || a < b):
			result.Passed++
		default:
			result.Failed++
		}
	}
	tested := result.Passed + result.Failed
	if tested > 0 {
		v := float64(result.Passed) / float64(tested)
		result.Satisfaction = &v
	}
	if len(constraints) > 0 {
		v := float64(tested) / float64(len(constraints))
		result.Coverage = &v
	}
	return result
}
