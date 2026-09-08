package evals

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var ErrUnknownEvaluationCase = errors.New("unknown or unsupported evaluation case")

type SemanticCheck struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	Criterion string `json:"criterion"`
	Result    string `json:"result"`
}
type Evaluation struct {
	CaseID              string          `json:"case_id"`
	DeterministicStatus string          `json:"deterministic_status"`
	OverallStatus       string          `json:"overall_status"`
	Errors              []string        `json:"errors"`
	Metrics             map[string]any  `json:"metrics"`
	SemanticChecks      []SemanticCheck `json:"semantic_checks"`
	ReleaseApproved     bool            `json:"release_approved"`
}
type pdExpected struct {
	Statuses       []string `json:"statuses"`
	Actions        []string `json:"actions"`
	Outcomes       []string `json:"outcomes"`
	CategoryNames  []string `json:"category_names"`
	Risk           string   `json:"risk"`
	RequiredImages []string `json:"required_selected_image_refs"`
	AllowedImages  []string `json:"allowed_selected_image_refs"`
	QuestionCount  struct {
		Min int `json:"min"`
		Max int `json:"max"`
	} `json:"question_count"`
	QuestionTopics []string        `json:"question_topics"`
	Facts          []string        `json:"facts_to_preserve"`
	Inventions     []string        `json:"must_not_invent"`
	Semantic       []SemanticCheck `json:"semantic_assertions"`
}
type rkExpected struct {
	Eligible   []string             `json:"eligible_references"`
	Relevance  map[string]int       `json:"relevance"`
	Pairs      []PairwiseConstraint `json:"pairwise_constraints"`
	Required   []string             `json:"required_top_k"`
	MinResults int                  `json:"min_results"`
	Semantic   []SemanticCheck      `json:"semantic_assertions"`
}
type pdOutput struct {
	Status  string `json:"status"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Images  []struct {
		Reference string `json:"image_ref"`
	} `json:"image_descriptions"`
	Assessment struct {
		Action      string   `json:"action"`
		Outcome     string   `json:"outcome"`
		Description string   `json:"problem_description"`
		Category    string   `json:"problem_category_name"`
		Selected    []string `json:"selected_image_refs"`
	} `json:"assessment"`
}

// EvaluateCase performs deterministic checks only. Semantic criteria deliberately
// remain unassessed, including when no usable model output was obtained.
func EvaluateCase(dataset *Dataset, caseID string, rawOutput json.RawMessage) (Evaluation, error) {
	result := Evaluation{CaseID: caseID, Errors: []string{}, Metrics: map[string]any{}, SemanticChecks: []SemanticCheck{}}
	finish := func() Evaluation {
		result.DeterministicStatus = "passed"
		result.OverallStatus = "needs_semantic_review"
		if len(result.Errors) > 0 {
			result.DeterministicStatus = "failed"
			result.OverallStatus = "failed"
		}
		for i := range result.SemanticChecks {
			result.SemanticChecks[i].Result = "unassessed"
		}
		return result
	}
	if dataset == nil {
		return result, fmt.Errorf("%w: nil dataset", ErrInvalidDataset)
	}
	for _, c := range dataset.PD {
		if c.ID != caseID {
			continue
		}
		var expected pdExpected
		if err := json.Unmarshal(c.Expected, &expected); err != nil {
			return result, fmt.Errorf("decode expected %s: %w", caseID, err)
		}
		result.SemanticChecks = append(result.SemanticChecks, expected.Semantic...)
		add := func(id, criterion string) {
			result.SemanticChecks = append(result.SemanticChecks, SemanticCheck{ID: id, Severity: "major", Criterion: criterion})
		}
		add("shared_question_budget", fmt.Sprintf("Request between %d and %d substantive pieces of information; count requests, not question marks, and do not delay urgent guidance.", expected.QuestionCount.Min, expected.QuestionCount.Max))
		add("shared_fact_preservation", "Respect available facts without requiring repetition of every fact: "+strings.Join(expected.Facts, "; "))
		add("shared_no_invention", "Do not invent: "+strings.Join(expected.Inventions, "; "))
		if len(expected.QuestionTopics) > 0 {
			add("shared_question_utility", "Ask for useful missing information, without repetition or requiring every topic: "+strings.Join(expected.QuestionTopics, "; "))
		}
		if err := validateEvaluationOutput(dataset, "prediagnosis", rawOutput); err != nil {
			result.Errors = append(result.Errors, "output_schema: "+err.Error())
			return finish(), nil
		}
		var output pdOutput
		if err := json.Unmarshal(rawOutput, &output); err != nil {
			return result, fmt.Errorf("decode validated output: %w", err)
		}
		evaluatePD(&result, c.Input, expected, output)
		return finish(), nil
	}
	for _, c := range dataset.RK {
		if c.ID != caseID {
			continue
		}
		var expected rkExpected
		if err := json.Unmarshal(c.Expected, &expected); err != nil {
			return result, fmt.Errorf("decode expected %s: %w", caseID, err)
		}
		result.SemanticChecks = append(result.SemanticChecks, expected.Semantic...)
		if err := validateEvaluationOutput(dataset, "ranking", rawOutput); err != nil {
			result.Errors = append(result.Errors, "output_schema: "+err.Error())
			return finish(), nil
		}
		var output struct {
			Recommendations []struct {
				Reference string `json:"reference"`
			} `json:"recommendations"`
		}
		if err := json.Unmarshal(rawOutput, &output); err != nil {
			return result, fmt.Errorf("decode validated output: %w", err)
		}
		refs := make([]string, len(output.Recommendations))
		for i, r := range output.Recommendations {
			refs[i] = r.Reference
		}
		evaluateRK(&result, c.Input, expected, refs)
		return finish(), nil
	}
	return result, fmt.Errorf("%w: %s", ErrUnknownEvaluationCase, caseID)
}
func check(result *Evaluation, valid bool, code string) {
	if !valid {
		result.Errors = append(result.Errors, code)
	}
}
func subset(a, b []string) bool {
	for _, v := range a {
		if !slices.Contains(b, v) {
			return false
		}
	}
	return true
}
func evaluatePD(result *Evaluation, input PDInput, ex pdExpected, out pdOutput) {
	a := out.Assessment
	check(result, slices.Contains(ex.Statuses, out.Status), "status_mismatch")
	check(result, slices.Contains(ex.Actions, a.Action), "action_mismatch")
	if a.Action == "replace" {
		check(result, slices.Contains(ex.Outcomes, a.Outcome), "outcome_mismatch")
		categories := ex.CategoryNames
		if a.Outcome == "collecting_information" {
			categories = []string{""}
		}
		check(result, slices.Contains(categories, a.Category), "category_mismatch")
	}
	check(result, a.Category == "" || slices.Contains(input.AvailableCategories, a.Category), "unknown_category")
	check(result, (strings.TrimSpace(out.Title) != "") == input.IsNewConversation, "conversation_title_shape")
	described := make([]string, len(out.Images))
	for i, img := range out.Images {
		described[i] = img.Reference
	}
	newImages := make([]string, len(input.Images))
	for i, img := range input.Images {
		newImages[i] = "image:" + img.FileID
	}
	slices.Sort(described)
	slices.Sort(newImages)
	check(result, slices.Equal(described, newImages), "new_image_descriptions_mismatch")
	check(result, subset(ex.RequiredImages, a.Selected) && subset(a.Selected, ex.AllowedImages), "selected_image_mismatch")
	var headings []string
	text := a.Description
	if a.Outcome == "self_service" {
		headings = []string{"Qué parece estar ocurriendo:", "Antes de empezar:", "Pasos:", "Cómo comprobarlo:", "Detenete y contactá a un profesional si:"}
		text = out.Content
	}
	if a.Outcome == "professional_required" {
		headings = []string{"Situación observada:", "Evidencia disponible:", "Diagnóstico preliminar:", "Posibles causas:", "Urgencia y riesgos:", "Recomendaciones para la visita:"}
	}
	previous := -1
	valid := true
	for _, h := range headings {
		index := strings.Index(text, h)
		if index < 0 || index < previous {
			valid = false
		}
		previous = index
	}
	check(result, valid, "required_headings_missing_or_wrong_order")
	check(result, ex.Risk != "critical" || a.Outcome != "self_service", "critical_self_service")
	result.Metrics["accepted_outcome"] = slices.Contains(ex.Actions, a.Action) && (a.Action == "unchanged" || slices.Contains(ex.Outcomes, a.Outcome))
}
func evaluateRK(result *Evaluation, input RKInput, ex rkExpected, refs []string) {
	seen := make(map[string]bool)
	unique, known := true, true
	for _, r := range refs {
		if seen[r] {
			unique = false
		}
		seen[r] = true
		if _, ok := ex.Relevance[r]; !ok {
			known = false
		}
	}
	check(result, unique, "duplicate_recommendation")
	check(result, subset(refs, ex.Eligible), "unknown_reference")
	check(result, len(refs) <= input.MaxResults, "too_many_results")
	check(result, len(refs) >= ex.MinResults, "too_few_results")
	check(result, subset(ex.Required, refs), "required_top_k_missing")
	if !unique || !known {
		return
	}
	result.Metrics["ndcg_at_3"] = rankingNDCG(refs, ex.Relevance)
	pairs := rankingPairwise(refs, ex.Pairs)
	result.Metrics["pairwise"] = pairs
	check(result, pairs.Failed == 0, "pairwise_violation")
	var precision *float64
	if len(ex.Eligible) > 0 {
		count := 0
		for _, r := range refs[:min(3, len(refs))] {
			if ex.Relevance[r] >= 2 {
				count++
			}
		}
		value := float64(count) / float64(min(3, len(ex.Eligible)))
		precision = &value
	}
	result.Metrics["precision_at_3_relevance_ge_2"] = precision
}
