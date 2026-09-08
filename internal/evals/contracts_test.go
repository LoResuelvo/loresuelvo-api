package evals

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExecuteContractsNoCandidates(t *testing.T) {
	results, err := ExecuteContracts(context.Background(), &Dataset{CT: []CTCase{{CaseMetadata: CaseMetadata{ID: "CT-01"}, Input: []byte(`{"eligible_candidates":[]}`)}}})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "pass", results[0].Status, results[0].Evidence)
}
func TestExecuteContractsTimeout(t *testing.T) {
	result, err := executeContract(context.Background(), &Dataset{}, CTCase{CaseMetadata: CaseMetadata{ID: "CT-09"}, Input: []byte(`{"model_error":"timeout"}`)})
	require.NoError(t, err)
	require.Equal(t, "pass", result.Status, result.Evidence)
	require.Equal(t, "unassessed", result.SemanticStatus)
}
func TestExecuteContractsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err := ExecuteContracts(ctx, &Dataset{CT: []CTCase{{CaseMetadata: CaseMetadata{ID: "CT-01"}}}})
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, results)
}
func TestExecuteContractsUnknownCase(t *testing.T) {
	results, err := ExecuteContracts(context.Background(), &Dataset{CT: []CTCase{{CaseMetadata: CaseMetadata{ID: "CT-unknown"}, Input: []byte(`{}`)}}})
	require.NoError(t, err)
	require.Equal(t, "not_implemented", results[0].Status)
}

func TestRankingLimitContractDoesNotPassOnValidOutput(t *testing.T) {
	result, err := executeContract(context.Background(), &Dataset{}, CTCase{CaseMetadata: CaseMetadata{ID: "CT-05"}, Input: []byte(`{"max_results":4,"returned_references":["a","b","c","d"]}`)})
	require.NoError(t, err)
	require.Equal(t, "fail", result.Status, result.Evidence)
}

func TestImageReferenceContractDoesNotPassOnKnownReference(t *testing.T) {
	result, err := executeContract(context.Background(), &Dataset{}, CTCase{CaseMetadata: CaseMetadata{ID: "CT-06"}, Input: []byte(`{"known_image_refs":["image:known"],"selected_image_refs":["image:known"]}`)})
	require.NoError(t, err)
	require.Equal(t, "fail", result.Status, result.Evidence)
}

func TestImageLoadingContractRequiresMissingAssetError(t *testing.T) {
	dataset, err := LoadDataset(writeDatasetFixture(t))
	require.NoError(t, err)
	result, err := executeContract(context.Background(), dataset, CTCase{CaseMetadata: CaseMetadata{ID: "CT-11"}, Input: []byte(`{"path":"manifest.json"}`)})
	require.NoError(t, err)
	require.Equal(t, "fail", result.Status)
}

func TestServiceContractsRejectInvalidOutput(t *testing.T) {
	cases := []struct{ id, input string }{
		{"CT-03", `{"eligible":["known"],"raw_output":{"recommendations":[{"reference":"unknown","reason":"reason"}]}}`},
		{"CT-04", `{"eligible":["known"],"raw_output":{"recommendations":[{"reference":"known","reason":"one"},{"reference":"known","reason":"two"}]}}`},
		{"CT-05", `{"max_results":3,"returned_references":["one","two","three","four"]}`},
		{"CT-06", `{"known_image_refs":["image:known"],"selected_image_refs":["image:unknown"]}`},
		{"CT-07", `{"available_categories":["Plumbing"],"outcome":"professional_required","problem_category_name":"Unknown"}`},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			result, err := executeContract(context.Background(), &Dataset{}, CTCase{CaseMetadata: CaseMetadata{ID: tc.id}, Input: []byte(tc.input)})
			require.NoError(t, err)
			require.Equal(t, "pass", result.Status, result.Evidence)
		})
	}
}

func TestSemanticContractsRemainUnassessed(t *testing.T) {
	for _, tc := range []struct{ id, input string }{
		{"CT-08", `{"available_categories":["Plumbing"],"user_message":"Urgent risk"}`},
		{"CT-10", `{"previous_summary":"Old context","messages":[{"sender_role":"consumer","content":"New risk"}]}`},
	} {
		t.Run(tc.id, func(t *testing.T) {
			result, err := executeContract(context.Background(), &Dataset{}, CTCase{CaseMetadata: CaseMetadata{ID: tc.id}, Input: []byte(tc.input)})
			require.NoError(t, err)
			require.Equal(t, "unassessed", result.Status, result.Evidence)
			require.Equal(t, "unassessed", result.SemanticStatus)
		})
	}
}
