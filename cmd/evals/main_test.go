package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRejectsUnsupportedExecution(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "not-a-live-authorization")
	for _, command := range []string{"live", "contract", "replay", "compare"} {
		t.Run(command, func(t *testing.T) {
			var out, err bytes.Buffer
			require.Equal(t, 2, run([]string{command, "--allow-live"}, &out, &err))
			require.Empty(t, out.String())
			require.Contains(t, err.String(), "no model calls")
		})
	}
}
func TestHelpNeedsNoDataset(t *testing.T) {
	var out, err bytes.Buffer
	require.Zero(t, run([]string{"--help"}, &out, &err))
	require.Contains(t, out.String(), "Offline preparation only")
	require.Empty(t, err.String())
}
func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"plan", "--unknown"}, {"validate", "extra"}, {"validate", "--dataset", t.TempDir()}} {
		var out, err bytes.Buffer
		require.Equal(t, 2, run(args, &out, &err))
		require.NotEmpty(t, err.String())
	}
}

func TestValidateSyntheticDataset(t *testing.T) {
	var out, err bytes.Buffer
	require.Zero(t, run([]string{"validate", "--dataset", cliDataset(t)}, &out, &err), err.String())
	require.Contains(t, out.String(), `"status": "passed"`)
	require.Contains(t, out.String(), `"live_model_calls": 0`)
}

func TestPlanReadsTrialDefaults(t *testing.T) {
	root := cliDataset(t)
	for _, tc := range []struct {
		suite    string
		requests string
		trials   string
	}{{"smoke", "1", "1"}, {"development", "3", "3"}} {
		t.Run(tc.suite, func(t *testing.T) {
			var out, err bytes.Buffer
			require.Zero(t, run([]string{"plan", "--dataset", root, "--suite", tc.suite, "--model", "requested", "--max-requests", tc.requests}, &out, &err), err.String())
			require.Contains(t, out.String(), `"trials": `+tc.trials)
			require.Contains(t, out.String(), `"estimated_cost": null`)
		})
	}
	var out, err bytes.Buffer
	require.Equal(t, 2, run([]string{"plan", "--dataset", root, "--suite", "smoke", "--model", "requested", "--trials", "0", "--max-requests", "1"}, &out, &err))
}
