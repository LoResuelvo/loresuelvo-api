package evals

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type caseExecutorMock struct{ mock.Mock }

func (m *caseExecutorMock) Execute(ctx context.Context, id string) (ExecutionOutput, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(ExecutionOutput), args.Error(1)
}

type roundTripperMock struct{ mock.Mock }

func (m *roundTripperMock) RoundTrip(request *http.Request) (*http.Response, error) {
	args := m.Called(request)
	var response *http.Response
	if args.Get(0) != nil {
		response = args.Get(0).(*http.Response)
	}
	return response, args.Error(1)
}
func executionTestLimits() ExecutionLimits {
	return ExecutionLimits{Concurrency: 1, AttemptTimeout: time.Second, GlobalTimeout: 2 * time.Second, MinInterval: time.Nanosecond, MaxOutputTokens: 100}
}
func executionTestJournal(t *testing.T, plan *Plan) (*Journal, RunRecord, string) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "run")
	record := RunRecord{FormatVersion: resultVersion, RunID: "test", Mode: "contract", Plan: plan}
	journal, err := NewJournal(directory, record)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, journal.Close()) })
	return journal, record, directory
}
func executionTestPlan(ids ...string) *Plan {
	plan := &Plan{Model: "test-model", Trials: 1, RequestLimit: len(ids)}
	for _, id := range ids {
		plan.Cases = append(plan.Cases, PlannedCase{CaseID: id})
	}
	return plan
}
