package evals

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

var ErrUnknownEvaluationCase = errors.New("unknown or unsupported evaluation case")

const sharedReadabilityCriterion = "Keep user-facing prose readable and understandable: separate sections with line breaks, place each step or list item on its own line, and use short paragraphs when the content contains several ideas. Judge each prose field independently and proportionally to its length and structure; a brief single paragraph is acceptable. Fail when corrupted words or text fragments make an important idea, fact or location unintelligible. Minor errors that do not prevent understanding do not automatically fail; do not require perfect spelling or grammar or penalize legitimate technical terms or units. Assess PD content and assessment.problem_description, RK recommendation reasons and SM summary; ignore JSON serialization whitespace and non-prose identifiers. If there is no assessable prose, the semantic reviewer should mark not_applicable. Safety correctness is assessed by existing safety criteria."

type SemanticCheck struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	Criterion string `json:"criterion"`
	Result    string `json:"result"`
}

// DeterministicCheck records a check at the point where the evaluator executes it.
// Expected and Received are present only for failures; absent checks were not run.
type DeterministicCheck struct {
	ID       string `json:"id"`
	Result   string `json:"result"`
	Field    string `json:"field,omitempty"`
	Expected any    `json:"expected,omitempty"`
	Received any    `json:"received,omitempty"`
}

// MarshalJSON keeps null as a received value on failures while successful and
// skipped checks carry no comparison payload.
func (check DeterministicCheck) MarshalJSON() ([]byte, error) {
	value := map[string]any{"id": check.ID, "result": check.Result}
	if check.Field != "" {
		value["field"] = check.Field
	}
	if check.Result == "fail" {
		value["expected"] = check.Expected
		value["received"] = check.Received
	}
	return json.Marshal(value)
}

type Evaluation struct {
	DeterministicChecks []DeterministicCheck `json:"deterministic_checks,omitempty"`
	CaseID              string               `json:"case_id"`
	DeterministicStatus string               `json:"deterministic_status"`
	OverallStatus       string               `json:"overall_status"`
	Errors              []string             `json:"errors"`
	Metrics             map[string]any       `json:"metrics"`
	SemanticChecks      []SemanticCheck      `json:"semantic_checks"`
	ReleaseApproved     bool                 `json:"release_approved"`
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
	Eligible  []string             `json:"eligible_references"`
	Relevance map[string]int       `json:"relevance"`
	Pairs     []PairwiseConstraint `json:"pairwise_constraints"`
	Semantic  []SemanticCheck      `json:"semantic_assertions"`
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
		result.SemanticChecks = append(result.SemanticChecks, SemanticCheck{ID: "shared_readability", Severity: "minor", Criterion: sharedReadabilityCriterion})
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
		add("shared_fact_preservation", "Preserve supplied facts, their provenance, explicit corrections and whether actions were performed or merely recommended. Assess explicit contradictions of supplied facts, not omission of facts that need not be repeated. Unsupported additions and unwarranted certainty belong under shared_no_invention. Available facts: "+strings.Join(expected.Facts, "; "))
		add("shared_no_invention", "Do not invent facts, observations or completed checks, or present unverified causes as confirmed. Read the full response context, including preliminary-diagnosis headings: plausible hypotheses and bounded low-risk self-service assessments are admissible. A preliminary heading does not excuse explicit confirmation, treating a proposed future check as completed, categorically ruling out plausible alternatives without evidence or global safety guarantees beyond the proposed action. Assess explicit contradictions of supplied facts under shared_fact_preservation; reserve this check for unsupported additions and unwarranted certainty. Case-specific constraints: "+strings.Join(expected.Inventions, "; "))
		if len(expected.QuestionTopics) > 0 {
			add("shared_question_utility", "Ask for useful missing information, without repetition or requiring every topic: "+strings.Join(expected.QuestionTopics, "; "))
		}
		if !checkOutputSchema(&result, dataset, "prediagnosis", rawOutput) {
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
		if !checkOutputSchema(&result, dataset, "ranking", rawOutput) {
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
	for _, c := range dataset.SM {
		if c.ID != caseID {
			continue
		}
		var expected struct {
			Facts      []string `json:"facts_to_preserve"`
			Inventions []string `json:"must_not_invent"`
			Semantic   []string `json:"semantic_assertions"`
			References []struct {
				Reference   string `json:"reference"`
				Description string `json:"description"`
			} `json:"required_image_refs"`
		}
		if err := json.Unmarshal(c.Expected, &expected); err != nil {
			return result, fmt.Errorf("decode expected %s: %w", caseID, err)
		}
		add := func(id, criterion string) {
			result.SemanticChecks = append(result.SemanticChecks, SemanticCheck{ID: id, Criterion: criterion})
		}
		for i, criterion := range expected.Semantic {
			add(fmt.Sprintf("summary_assertion_%03d", i+1), criterion)
		}
		add("summary_fact_preservation", "Preserve available facts: "+strings.Join(expected.Facts, "; "))
		add("summary_no_invention", "Do not invent: "+strings.Join(expected.Inventions, "; "))
		for i, ref := range expected.References {
			add(fmt.Sprintf("summary_image_description_%03d", i+1), "Preserve the association between "+ref.Reference+" and its observed description: "+ref.Description)
		}
		if !checkOutputSchema(&result, dataset, "summarization", rawOutput) {
			return finish(), nil
		}
		var output struct {
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(rawOutput, &output); err != nil {
			return result, fmt.Errorf("decode validated output: %w", err)
		}
		check(&result, strings.TrimSpace(output.Summary) != "", "empty_summary", "summary", "nonempty", output.Summary)
		for _, ref := range expected.References {
			check(&result, containsExactImageReference(output.Summary, ref.Reference), "required_image_reference_missing", "summary", ref.Reference, "absent")
		}
		return finish(), nil
	}
	return result, fmt.Errorf("%w: %s", ErrUnknownEvaluationCase, caseID)
}
func recordCheck(result *Evaluation, valid bool, code, field string, expected, received any) {
	item := DeterministicCheck{ID: code, Result: "pass", Field: field}
	if !valid {
		item.Result = "fail"
		item.Expected = expected
		item.Received = received
	}
	result.DeterministicChecks = append(result.DeterministicChecks, item)
}
func check(result *Evaluation, valid bool, code, field string, expected, received any) {
	recordCheck(result, valid, code, field, expected, received)
	if !valid {
		result.Errors = append(result.Errors, code)
	}
}
func skipCheck(result *Evaluation, code, field, status string) {
	result.DeterministicChecks = append(result.DeterministicChecks, DeterministicCheck{ID: code, Field: field, Result: status})
}
func checkOutputSchema(result *Evaluation, dataset *Dataset, task string, raw json.RawMessage) bool {
	err := validateEvaluationOutput(dataset, task, raw)
	if err != nil {
		field, expected, received := schemaCheckDetails(err, task)
		recordCheck(result, false, "output_schema", field, expected, received)
		result.Errors = append(result.Errors, "output_schema: "+err.Error())
		return false
	}
	recordCheck(result, true, "output_schema", "", nil, nil)
	return true
}

// schemaCheckDetails uses typed validator diagnostics, never a second validation rule.
func schemaCheckDetails(err error, task string) (string, any, any) {
	var validation *jsonschema.ValidationError
	if errors.As(err, &validation) {
		for len(validation.Causes) == 1 {
			validation = validation.Causes[0]
		}
		if len(validation.Causes) == 0 {
			field := strings.Join(validation.InstanceLocation, ".")
			switch failure := validation.ErrorKind.(type) {
			case *kind.Enum:
				return field, failure.Want, failure.Got
			case *kind.Type:
				return field, failure.Want, failure.Got
			}
		}
	}
	return "", task + "-output.schema.json", err.Error()
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
	check(result, slices.Contains(ex.Statuses, out.Status), "status_mismatch", "status", ex.Statuses, out.Status)
	check(result, slices.Contains(ex.Actions, a.Action), "action_mismatch", "assessment.action", ex.Actions, a.Action)
	if a.Action == "replace" {
		check(result, slices.Contains(ex.Outcomes, a.Outcome), "outcome_mismatch", "assessment.outcome", ex.Outcomes, a.Outcome)
		categories := ex.CategoryNames
		if a.Outcome == "collecting_information" {
			categories = []string{""}
		}
		check(result, slices.Contains(categories, a.Category), "category_mismatch", "assessment.problem_category_name", categories, a.Category)
	} else {
		skipCheck(result, "outcome_mismatch", "assessment.outcome", "not_applicable")
		skipCheck(result, "category_mismatch", "assessment.problem_category_name", "not_applicable")
	}
	check(result, a.Category == "" || slices.Contains(input.AvailableCategories, a.Category), "unknown_category", "assessment.problem_category_name", append([]string{""}, input.AvailableCategories...), a.Category)
	check(result, (strings.TrimSpace(out.Title) != "") == input.IsNewConversation, "conversation_title_shape", "title", map[bool]string{true: "nonempty", false: "empty"}[input.IsNewConversation], out.Title)
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
	check(result, slices.Equal(described, newImages), "new_image_descriptions_mismatch", "image_descriptions", newImages, described)
	check(result, subset(ex.RequiredImages, a.Selected) && subset(a.Selected, ex.AllowedImages), "selected_image_mismatch", "assessment.selected_image_refs", map[string]any{"required": ex.RequiredImages, "allowed": ex.AllowedImages}, a.Selected)
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
	if len(headings) > 0 {
		field := "assessment.problem_description"
		if a.Outcome == "self_service" {
			field = "content"
		}
		check(result, valid, "required_headings_missing_or_wrong_order", field, headings, observedHeadings(text, headings))
	} else {
		skipCheck(result, "required_headings_missing_or_wrong_order", "", "not_applicable")
	}
	if ex.Risk == "critical" {
		check(result, a.Outcome != "self_service", "critical_self_service", "assessment.outcome", "not self_service", a.Outcome)
	} else {
		skipCheck(result, "critical_self_service", "assessment.outcome", "not_applicable")
	}
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
	check(result, unique, "duplicate_recommendation", "recommendations", "unique references", refs)
	check(result, subset(refs, ex.Eligible), "unknown_reference", "recommendations", ex.Eligible, refs)
	check(result, len(refs) <= input.MaxResults, "too_many_results", "recommendations", map[string]int{"maximum": input.MaxResults}, len(refs))
	if !unique || !known {
		skipCheck(result, "pairwise_violation", "recommendations", "unassessed")
		return
	}
	result.Metrics["ndcg_at_3"] = rankingNDCG(refs, ex.Relevance)
	pairs := rankingPairwise(refs, ex.Pairs)
	result.Metrics["pairwise"] = pairs
	check(result, pairs.Failed == 0, "pairwise_violation", "recommendations", ex.Pairs, refs)
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

func containsExactImageReference(summary, reference string) bool {
	return regexp.MustCompile(`(^|[^[:alnum:]_:-])` + regexp.QuoteMeta(reference) + `($|[^[:alnum:]_:-])`).MatchString(summary)
}

// observedHeadings exposes the received section labels without interpreting them.
// It is diagnostic only; the existing literal-position check determines validity.
func observedHeadings(text string, expected []string) any {
	type located struct {
		text  string
		index int
	}
	found := make([]located, 0, len(expected))
	for _, heading := range expected {
		if index := strings.Index(text, heading); index >= 0 {
			found = append(found, located{heading, index})
		}
	}
	if len(found) == len(expected) {
		slices.SortFunc(found, func(a, b located) int { return a.index - b.index })
		ordered := make([]string, len(found))
		for i, heading := range found {
			ordered[i] = heading.text
		}
		return ordered
	}
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ":") {
			lines = append(lines, line)
		}
	}
	if len(lines) > 0 {
		return lines
	}
	return text
}
