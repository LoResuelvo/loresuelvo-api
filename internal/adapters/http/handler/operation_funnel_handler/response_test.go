package operation_funnel_handler

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/require"
)

func TestFunnelResponseUsesExactDecimalJSONNumbersAndPreservesNulls(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	observedAt := to.Add(time.Hour)
	zero := int64(0)
	percent := int64(3334)
	mean := int64(3334)
	metrics := readmodel.FunnelMetrics{
		From: from, To: to, ObservedAt: observedAt, TimeZone: "America/Argentina/Buenos_Aires",
		Rounding: "half_up", DecimalPlaces: 2,
		AI: readmodel.FunnelCohort{
			Stages: []readmodel.FunnelStageMetric{
				{Stage: readmodel.FunnelStage("professional_assessment"), Count: 3},
				{Stage: readmodel.FunnelStage("request"), Count: 1, ConversionHundredths: &percent},
			},
			GlobalCompletionConversionHundredths: &zero,
			Delays:                               readmodel.FunnelDelays{RequestToFirstProposal: readmodel.FunnelDelayMetric{Observations: 1, MeanSecondsHundredths: &mean}},
		},
	}
	encoded, err := json.Marshal(responseFromMetrics(metrics))
	require.NoError(t, err)
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var body map[string]any
	require.NoError(t, decoder.Decode(&body))
	require.Equal(t, "2026-09-01T00:00:00Z", body["period"].(map[string]any)["from"])
	require.Equal(t, "2026-09-02T01:00:00Z", body["observed_at"])
	require.Nil(t, body["category_id"])
	cohorts := body["cohorts"].(map[string]any)
	ai := cohorts["ai"].(map[string]any)
	stages := ai["stages"].([]any)
	require.Nil(t, stages[0].(map[string]any)["conversion_percentage"])
	require.Equal(t, json.Number("33.34"), stages[1].(map[string]any)["conversion_percentage"])
	require.Equal(t, json.Number("0.00"), ai["global_completion_conversion_percentage"])
	delays := ai["delays"].(map[string]any)
	require.Equal(t, json.Number("33.34"), delays["request_to_first_proposal"].(map[string]any)["mean_seconds"])
	manual := cohorts["manual"].(map[string]any)
	require.Equal(t, []any{}, manual["stages"])
	require.Nil(t, manual["global_completion_conversion_percentage"])
	require.Nil(t, manual["delays"].(map[string]any)["request_to_first_proposal"].(map[string]any)["mean_seconds"])
	require.Equal(t, json.Number("0"), manual["delays"].(map[string]any)["request_to_first_proposal"].(map[string]any)["observations"])
	require.Equal(t, "assessment.problem_category_id", ai["category_source"])
	require.Equal(t, "provider.current_category_id", manual["category_source"])
}

func TestFunnelDecimalFormattingPreservesTinyNegativeMagnitude(t *testing.T) {
	negativeHundredth := int64(-1)
	formatted := decimalHundredths(&negativeHundredth)
	require.NotNil(t, formatted)
	require.Equal(t, json.Number("-0.01"), *formatted)
}
