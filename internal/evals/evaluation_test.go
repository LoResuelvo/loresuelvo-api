package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateRankingRejectsInvalidReferencesWithoutMetrics(t *testing.T) {
	for _, refs := range [][]string{{"a", "a"}, {"unknown"}} {
		t.Run(refs[len(refs)-1], func(t *testing.T) {
			d := rankingEvaluationFixture(t)
			raw := rankingRaw(t, refs)
			result, err := EvaluateCase(d, "RK-test", raw)
			require.NoError(t, err)
			require.Equal(t, "failed", result.DeterministicStatus)
			require.Empty(t, result.Metrics)
			require.False(t, result.ReleaseApproved)
		})
	}
}
func TestEvaluateRankingPreservesMissingRecommendationPenalty(t *testing.T) {
	result, err := EvaluateCase(rankingEvaluationFixture(t), "RK-test", rankingRaw(t, []string{"a"}))
	require.NoError(t, err)
	require.InDelta(t, 1.0/3, *result.Metrics["precision_at_3_relevance_ge_2"].(*float64), 0.000001)
	require.Contains(t, result.Errors, "too_few_results")
	require.Less(t, *result.Metrics["ndcg_at_3"].(*float64), 1.0)
}
func TestEvaluateRankingRequiresSemanticReview(t *testing.T) {
	result, err := EvaluateCase(rankingEvaluationFixture(t), "RK-test", rankingRaw(t, []string{"a", "b", "c"}))
	require.NoError(t, err)
	require.Equal(t, "passed", result.DeterministicStatus)
	require.Equal(t, "needs_semantic_review", result.OverallStatus)
	require.False(t, result.ReleaseApproved)
	require.Equal(t, "unassessed", result.SemanticChecks[0].Result)
}
func TestEvaluateMissingOutputRetainsSemanticChecks(t *testing.T) {
	result, err := EvaluateCase(rankingEvaluationFixture(t), "RK-test", nil)
	require.NoError(t, err)
	require.Equal(t, "failed", result.OverallStatus)
	require.Equal(t, "unassessed", result.SemanticChecks[0].Result)
}
func TestEvaluateRejectsExtraOutputFields(t *testing.T) {
	result, err := EvaluateCase(rankingEvaluationFixture(t), "RK-test", json.RawMessage(`{"recommendations":[],"extra":true}`))
	require.NoError(t, err)
	require.Equal(t, "failed", result.DeterministicStatus)
	require.Contains(t, result.Errors[0], "output_schema")
}
func TestOutputSchemaRejectsConditionalNonemptyFields(t *testing.T) {
	d := &Dataset{schemaFiles: evaluationSchemaFixture(t, "prediagnosis-output")}
	raw := json.RawMessage(`{"status":"answered","title":"","content":"text","image_descriptions":[],"assessment":{"action":"unchanged","outcome":"","problem_title":"unexpected","problem_description":"","problem_category_name":"","selected_image_refs":[]}}`)
	require.Error(t, validateEvaluationOutput(d, "prediagnosis", raw))
}
func TestEvaluationSchemaRejectsExternalReferences(t *testing.T) {
	_, err := compileEvaluationSchema(map[string][]byte{"schemas/ranking-output.schema.json": []byte(`{"$ref":"https://example.invalid/remote"}`)}, "ranking-output")
	require.ErrorContains(t, err, ErrExternalSchema.Error())
}
func TestEvaluatePDChecksExpectedFieldsAndDefersSemanticJudgment(t *testing.T) {
	expected := json.RawMessage(`{"statuses":["answered"],"actions":["replace"],"outcomes":["collecting_information"],"category_names":[""],"question_count":{"min":1,"max":2},"question_topics":["location"],"semantic_assertions":[{"id":"safety","severity":"critical","criterion":"Safety must be reviewed."}]}`)
	d := &Dataset{schemaFiles: evaluationSchemaFixture(t, "prediagnosis-output"), PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-test"}, Expected: expected, Input: PDInput{IsNewConversation: true}}}}
	raw := json.RawMessage(`{"status":"answered","title":"Question","content":"Where is it?","image_descriptions":[],"assessment":{"action":"replace","outcome":"collecting_information","problem_title":"","problem_description":"","problem_category_name":"","selected_image_refs":[]}}`)
	result, err := EvaluateCase(d, "PD-test", raw)
	require.NoError(t, err)
	require.Equal(t, "passed", result.DeterministicStatus)
	require.Len(t, result.SemanticChecks, 5)
	for _, criterion := range result.SemanticChecks {
		require.Equal(t, "unassessed", criterion.Result)
	}
}

func evaluationSchemaFixture(t *testing.T, name string) map[string][]byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "evals", "datasets", "LoResuelvo_US60_evals_v1.0.0", "schemas", name+".schema.json"))
	require.NoError(t, err)
	return map[string][]byte{"schemas/" + name + ".schema.json": data}
}
func rankingEvaluationFixture(t *testing.T) *Dataset {
	t.Helper()
	return &Dataset{schemaFiles: evaluationSchemaFixture(t, "ranking-output"), RK: []RKCase{{CaseMetadata: CaseMetadata{ID: "RK-test"}, Input: RKInput{MaxResults: 3}, Expected: json.RawMessage(`{"eligible_references":["a","b","c"],"relevance":{"a":3,"b":2,"c":0},"min_results":3,"required_top_k":["a"],"pairwise_constraints":[{"higher":"a","lower":"b"}],"semantic_assertions":[{"id":"grounding","severity":"major","criterion":"No invention."}]}`)}}}
}
func rankingRaw(t *testing.T, refs []string) json.RawMessage {
	t.Helper()
	items := make([]map[string]string, 0, len(refs))
	for _, ref := range refs {
		items = append(items, map[string]string{"reference": ref, "reason": "evidence"})
	}
	raw, err := json.Marshal(map[string]any{"recommendations": items})
	require.NoError(t, err)
	return raw
}

func TestEvaluatePDRejectsDeterministicMismatches(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PDInput, *pdExpected, *pdOutput)
		want   string
	}{
		{"status", func(_ *PDInput, _ *pdExpected, o *pdOutput) { o.Status = "out_of_scope" }, "status_mismatch"},
		{"action", func(_ *PDInput, _ *pdExpected, o *pdOutput) { o.Assessment.Action = "unchanged" }, "action_mismatch"},
		{"outcome", func(_ *PDInput, _ *pdExpected, o *pdOutput) { o.Assessment.Outcome = "professional_required" }, "outcome_mismatch"},
		{"category", func(_ *PDInput, _ *pdExpected, o *pdOutput) { o.Assessment.Category = "invented" }, "unknown_category"},
		{"title", func(_ *PDInput, _ *pdExpected, o *pdOutput) { o.Title = "" }, "conversation_title_shape"},
		{"new_image", func(i *PDInput, _ *pdExpected, _ *pdOutput) { i.Images = []ImageInput{{FileID: "new"}} }, "new_image_descriptions_mismatch"},
		{"required_image", func(_ *PDInput, e *pdExpected, _ *pdOutput) { e.RequiredImages = []string{"image:required"} }, "selected_image_mismatch"},
		{"unknown_image", func(_ *PDInput, _ *pdExpected, o *pdOutput) { o.Assessment.Selected = []string{"image:invented"} }, "selected_image_mismatch"},
		{"headings", func(_ *PDInput, _ *pdExpected, o *pdOutput) { o.Assessment.Outcome = "self_service" }, "required_headings_missing_or_wrong_order"},
		{"critical", func(_ *PDInput, e *pdExpected, o *pdOutput) {
			e.Risk = "critical"
			o.Assessment.Outcome = "self_service"
		}, "critical_self_service"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := PDInput{IsNewConversation: true}
			expected := pdExpected{Statuses: []string{"answered"}, Actions: []string{"replace"}, Outcomes: []string{"collecting_information"}, CategoryNames: []string{""}}
			output := pdOutput{Status: "answered", Title: "Question"}
			output.Assessment.Action = "replace"
			output.Assessment.Outcome = "collecting_information"
			tc.mutate(&input, &expected, &output)
			result := Evaluation{Metrics: map[string]any{}}
			evaluatePD(&result, input, expected, output)
			require.Contains(t, result.Errors, tc.want)
		})
	}
}
func TestEvaluatePDRejectsWrongHeadingOrder(t *testing.T) {
	output := pdOutput{Content: "Pasos: Antes de empezar: Qué parece estar ocurriendo: Cómo comprobarlo: Detenete y contactá a un profesional si:"}
	output.Assessment.Outcome = "self_service"
	result := Evaluation{Metrics: map[string]any{}}
	evaluatePD(&result, PDInput{}, pdExpected{}, output)
	require.Contains(t, result.Errors, "required_headings_missing_or_wrong_order")
}
func TestEvaluateRankingRejectsMissingRequiredAndTooMany(t *testing.T) {
	result, err := EvaluateCase(rankingEvaluationFixture(t), "RK-test", rankingRaw(t, []string{"b", "c"}))
	require.NoError(t, err)
	require.Contains(t, result.Errors, "required_top_k_missing")
	require.Contains(t, result.Errors, "pairwise_violation")
	d := rankingEvaluationFixture(t)
	d.RK[0].Input.MaxResults = 1
	result, err = EvaluateCase(d, "RK-test", rankingRaw(t, []string{"a", "b", "c"}))
	require.NoError(t, err)
	require.Contains(t, result.Errors, "too_many_results")
}
func TestEvaluateUnknownCaseReturnsError(t *testing.T) {
	_, err := EvaluateCase(rankingEvaluationFixture(t), "unknown", nil)
	require.ErrorIs(t, err, ErrUnknownEvaluationCase)
}
