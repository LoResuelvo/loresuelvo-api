package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRejectsUnsupportedExecution(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "not-a-live-authorization")
	for _, command := range []string{"unknown", "all"} {
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
	require.Contains(t, out.String(), "Offline modes never call a model")
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

func liveArguments(root string) []string {
	return []string{"live", "--dataset", root, "--suite", "smoke", "--model", "requested", "--max-requests", "1", "--attempt-timeout", "1s", "--global-timeout", "10s", "--min-interval", "1ms", "--max-output-tokens", "128"}
}

func TestLiveDryRunNeedsNoCredential(t *testing.T) {
	t.Setenv("CHATBOT_API_KEY", "")
	var out, stderr bytes.Buffer
	args := append(liveArguments(cliDataset(t)), "--dry-run")
	require.Zero(t, run(args, &out, &stderr), stderr.String())
	require.Contains(t, out.String(), `"live_model_calls": 0`)
}
func TestLiveRequiresOptInAndCredentials(t *testing.T) {
	t.Setenv("CHATBOT_API_KEY", "")
	args := liveArguments(cliDataset(t))
	var out, stderr bytes.Buffer
	require.Equal(t, 2, run(args, &out, &stderr))
	require.Contains(t, stderr.String(), "--allow-live")
	stderr.Reset()
	require.Equal(t, 2, run(append(args, "--allow-live", "--out", t.TempDir()+"/run"), &out, &stderr))
	require.Contains(t, stderr.String(), "CHATBOT_API_KEY")
}
func TestOfflineCommandsRejectLiveFlag(t *testing.T) {
	for _, command := range []string{"validate", "contract", "replay", "compare"} {
		var out, stderr bytes.Buffer
		require.Equal(t, 2, run([]string{command, "--allow-live"}, &out, &stderr))
		require.Contains(t, stderr.String(), "flag provided but not defined")
	}
}
func TestRejectsOutputInsideDatasetWithoutCreatingDirectories(t *testing.T) {
	root := t.TempDir()
	require.Error(t, checkOutputDirectory(root, root+"/nested/run"))
	_, err := os.Stat(root + "/nested")
	require.True(t, os.IsNotExist(err))
}
