package evals

import (
	"encoding/json"
	"fmt"
)

type MetamorphicObservation struct {
	LeftCaseID   string                `json:"left_case_id"`
	RightCaseID  string                `json:"right_case_id"`
	Trial        int                   `json:"trial"`
	Variant      *MetamorphicVariant   `json:"variant,omitempty"`
	InputChanged *bool                 `json:"input_changed"`
	Comparison   MetamorphicComparison `json:"comparison"`
}
type MetamorphicReport struct {
	RunID           string                   `json:"run_id"`
	Observations    []MetamorphicObservation `json:"observations"`
	ReleaseApproved bool                     `json:"release_approved"`
	Warning         string                   `json:"warning"`
}

// ReplayMetamorphic compares stored paired trials; it does not generate variants
// or responses. Missing and invalid outputs remain explicitly unassessed.
func ReplayMetamorphic(dataset *Dataset, directory string) (MetamorphicReport, error) {
	record, _, err := Replay(dataset, directory)
	if err != nil {
		return MetamorphicReport{}, err
	}
	observed, attempts, err := ReadRun(directory)
	if err != nil {
		return MetamorphicReport{}, err
	}
	if observed.AttemptsSHA256 != record.AttemptsSHA256 {
		return MetamorphicReport{}, fmt.Errorf("run changed during metamorphic replay")
	}
	result := MetamorphicReport{RunID: record.RunID, Observations: []MetamorphicObservation{}, Warning: "Historical paired trials only. Variants and repeated trials are not independent cases. No-op transformations and absent/invalid outputs do not demonstrate invariance. Semantic safety remains unassessed."}
	latest := map[string]Attempt{}
	for _, a := range attempts {
		latest[attemptKey(a.CaseID, a.Trial)] = a
	}
	missing := func(reason string) MetamorphicComparison {
		return MetamorphicComparison{DeterministicStatus: "unassessed", SemanticStatus: "unassessed", AggregationUnit: "base_case_not_variant", Errors: []string{reason}}
	}
	for _, c := range record.Plan.Cases {
		if c.Variant == nil {
			continue
		}
		variant := c.Variant
		var base *RKCase
		for i := range dataset.RK {
			if dataset.RK[i].ID == variant.BaseCaseID {
				base = &dataset.RK[i]
				break
			}
		}
		if base == nil {
			return result, fmt.Errorf("unknown variant parent %s", variant.BaseCaseID)
		}
		transformed, transformErr := TransformRanking(base.Input, variant.Transformation, variant.Seed)
		if transformErr != nil {
			return result, transformErr
		}
		for trial := 1; trial <= record.Plan.Trials; trial++ {
			observation := MetamorphicObservation{LeftCaseID: base.ID, RightCaseID: c.CaseID, Trial: trial, Variant: variant, InputChanged: &transformed.Changed, Comparison: missing("paired output absent or invalid")}
			left, lok := latest[attemptKey(base.ID, trial)]
			right, rok := latest[attemptKey(c.CaseID, trial)]
			if !transformed.Changed {
				observation.Comparison = missing("transformation did not change effective input")
			} else if lok && rok && left.Status == "executed" && right.Status == "executed" {
				raw := json.RawMessage(right.RawOutput)
				if len(transformed.InverseReferences) > 0 {
					raw, err = RestoreRankingOutput(raw, transformed.InverseReferences)
				} else {
					err = nil
				}
				if err == nil && validateEvaluationOutput(dataset, "ranking", raw) == nil && validateEvaluationOutput(dataset, "ranking", json.RawMessage(left.RawOutput)) == nil {
					observation.Comparison, err = CompareRankingMetamorphic(base.Expected, json.RawMessage(left.RawOutput), raw)
					if err != nil {
						return result, err
					}
				}
			}
			result.Observations = append(result.Observations, observation)
		}
	}
	for _, pair := range dataset.Metamorphic.PrediagnosisPairs {
		for trial := 1; trial <= record.Plan.Trials; trial++ {
			observation := MetamorphicObservation{LeftCaseID: pair.Left, RightCaseID: pair.Right, Trial: trial, Comparison: missing("paired PD output absent or invalid")}
			observation.Comparison.Relation = pair.Relation
			observation.Comparison.AggregationUnit = "family_not_pair_member"
			left, lok := latest[attemptKey(pair.Left, trial)]
			right, rok := latest[attemptKey(pair.Right, trial)]
			if lok && rok && left.Status == "executed" && right.Status == "executed" && validateEvaluationOutput(dataset, "prediagnosis", json.RawMessage(left.RawOutput)) == nil && validateEvaluationOutput(dataset, "prediagnosis", json.RawMessage(right.RawOutput)) == nil {
				observation.Comparison, err = ComparePrediagnosisPair(pair, json.RawMessage(left.RawOutput), json.RawMessage(right.RawOutput))
				if err != nil {
					return result, err
				}
			}
			result.Observations = append(result.Observations, observation)
		}
	}
	return result, nil
}
