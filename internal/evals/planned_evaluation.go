package evals

import (
	"encoding/json"
	"fmt"
)

// evaluatePlannedAttempt restores reference identities for scoring only. Original
// provider output and effective transformed input remain unchanged in the journal.
func evaluatePlannedAttempt(dataset *Dataset, plan *Plan, attempt Attempt) (Evaluation, error) {
	for _, planned := range plan.Cases {
		if planned.CaseID != attempt.CaseID {
			continue
		}
		if planned.Variant == nil {
			return EvaluateCase(dataset, attempt.CaseID, json.RawMessage(attempt.RawOutput))
		}
		variant := planned.Variant
		raw := json.RawMessage(attempt.RawOutput)
		for _, base := range dataset.RK {
			if base.ID != variant.BaseCaseID {
				continue
			}
			transformed, err := TransformRanking(base.Input, variant.Transformation, variant.Seed)
			if err != nil {
				return Evaluation{}, err
			}
			if len(raw) > 0 && len(transformed.InverseReferences) > 0 {
				restored, restoreErr := RestoreRankingOutput(raw, transformed.InverseReferences)
				if restoreErr != nil {
					evaluation, evalErr := EvaluateCase(dataset, base.ID, raw)
					if evalErr != nil {
						return evaluation, evalErr
					}
					evaluation.Metrics = map[string]any{}
					evaluation.Errors = append(evaluation.Errors, "reference_restoration: "+restoreErr.Error())
					evaluation.DeterministicStatus = "failed"
					evaluation.OverallStatus = "failed"
					evaluation.CaseID = attempt.CaseID
					return evaluation, nil
				}
				raw = restored
			}
			evaluation, evalErr := EvaluateCase(dataset, base.ID, raw)
			evaluation.CaseID = attempt.CaseID
			return evaluation, evalErr
		}
	}
	return Evaluation{}, fmt.Errorf("attempt %q has no known planned base case", attempt.CaseID)
}
