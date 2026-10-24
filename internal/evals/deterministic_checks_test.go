package evals

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeterministicChecksRecordOnlyExecutedChecksAfterSchemaFailure(t *testing.T) {
	result, err := EvaluateCase(rankingEvaluationFixture(t), "RK-test", json.RawMessage(`{"recommendations":[],"extra":true}`))
	require.NoError(t, err)
	require.Len(t, result.DeterministicChecks, 1)
	require.Equal(t, "output_schema", result.DeterministicChecks[0].ID)
	require.Equal(t, "fail", result.DeterministicChecks[0].Result)
	require.NotNil(t, result.DeterministicChecks[0].Expected)
	require.NotNil(t, result.DeterministicChecks[0].Received)
}

func TestDeterministicChecksKeepSuccessfulChecksCompact(t *testing.T) {
	result, err := EvaluateCase(rankingEvaluationFixture(t), "RK-test", rankingRaw(t, []string{"a", "b", "c"}))
	require.NoError(t, err)
	require.Len(t, result.DeterministicChecks, 5)
	for _, check := range result.DeterministicChecks {
		require.Equal(t, "pass", check.Result)
		require.Nil(t, check.Expected)
		require.Nil(t, check.Received)
	}
	raw, err := json.Marshal(result.DeterministicChecks)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"expected"`)
	require.NotContains(t, string(raw), `"received"`)
}

func TestDeterministicChecksExposeHeadingDifferenceWithoutChangingVerdict(t *testing.T) {
	output := pdOutput{Content: "Qué parece estar informando:\nRestos en la rejilla.\nAntes de empezar:\nPrecaución.\nPasos:\nRetirar restos.\nCómo comprobarlo:\nObservar.\nDetenete y contactá a un profesional si:\nPersiste."}
	output.Assessment.Action = "replace"
	output.Assessment.Outcome = "self_service"
	expected := pdExpected{Statuses: []string{""}, Actions: []string{"replace"}, Outcomes: []string{"self_service"}, CategoryNames: []string{""}}
	result := Evaluation{Metrics: map[string]any{}}
	evaluatePD(&result, PDInput{}, expected, output)
	require.Equal(t, []string{"required_headings_missing_or_wrong_order"}, result.Errors)
	for _, check := range result.DeterministicChecks {
		if check.ID == "required_headings_missing_or_wrong_order" {
			require.Equal(t, "fail", check.Result)
			require.Equal(t, "content", check.Field)
			require.Contains(t, check.Expected, "Qué parece estar ocurriendo:")
			require.Contains(t, check.Received, "Qué parece estar informando:")
			return
		}
	}
	t.Fatal("missing executed heading check")
}

func TestDeterministicChecksDoNotApprovePairwiseWhenReferencesPreventEvaluation(t *testing.T) {
	result, err := EvaluateCase(rankingEvaluationFixture(t), "RK-test", rankingRaw(t, []string{"a", "a"}))
	require.NoError(t, err)
	for _, check := range result.DeterministicChecks {
		if check.ID == "pairwise_violation" {
			require.Equal(t, "unassessed", check.Result)
			return
		}
	}
	t.Fatal("missing pairwise check state")
}

func TestDeterministicSchemaCheckExposesTypedEnumValues(t *testing.T) {
	d := &Dataset{schemaFiles: evaluationSchemaFixture(t, "prediagnosis-output"), PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-test"}, Expected: json.RawMessage(`{}`)}}}
	raw := json.RawMessage(`{"status":"answered","title":"","content":"Advice","image_descriptions":[],"assessment":{"action":"replace","outcome":"","problem_title":"","problem_description":"","problem_category_name":"","selected_image_refs":[]}}`)
	result, err := EvaluateCase(d, "PD-test", raw)
	require.NoError(t, err)
	require.Len(t, result.DeterministicChecks, 1)
	check := result.DeterministicChecks[0]
	require.Equal(t, "assessment.outcome", check.Field)
	require.Equal(t, []any{"collecting_information", "self_service", "professional_required"}, check.Expected)
	require.Equal(t, "", check.Received)
}

func TestDeterministicChecksExposeMissingReferenceWithoutRepeatingSummary(t *testing.T) {
	d := &Dataset{schemaFiles: evaluationSchemaFixture(t, "summarization-output"), SM: []SMCase{{CaseMetadata: CaseMetadata{ID: "SM-test"}, Expected: json.RawMessage(`{"required_image_refs":[{"reference":"image:required","description":"photo"}]}`)}}}
	result, err := EvaluateCase(d, "SM-test", json.RawMessage(`{"summary":"A readable summary without the required image reference."}`))
	require.NoError(t, err)
	for _, check := range result.DeterministicChecks {
		if check.ID == "required_image_reference_missing" {
			require.Equal(t, "image:required", check.Expected)
			require.Equal(t, "absent", check.Received)
			return
		}
	}
	t.Fatal("missing required image check")
}

func TestDeterministicFailureKeepsReceivedNullDistinctFromAbsence(t *testing.T) {
	d := &Dataset{schemaFiles: evaluationSchemaFixture(t, "prediagnosis-output"), PD: []PDCase{{CaseMetadata: CaseMetadata{ID: "PD-test"}, Expected: json.RawMessage(`{}`)}}}
	raw := json.RawMessage(`{"status":null,"title":"","content":"Advice","image_descriptions":[],"assessment":{"action":"unchanged","outcome":"","problem_title":"","problem_description":"","problem_category_name":"","selected_image_refs":[]}}`)
	result, err := EvaluateCase(d, "PD-test", raw)
	require.NoError(t, err)
	require.Len(t, result.DeterministicChecks, 1)
	encoded, err := json.Marshal(result.DeterministicChecks[0])
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	require.Contains(t, payload, "received")
	require.Nil(t, payload["received"])
}
