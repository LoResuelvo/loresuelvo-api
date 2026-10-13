package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRecoveryDispatchPreservesSuccessAnd429WithPendingTarget(t *testing.T) {
	prep, candidates, success, originalPath, output := cliRecoveryEvidenceFixture(t)
	executor := &recoveryExecutorMock{}
	executor.On("Execute", mock.Anything, "first").Return(success, nil).Once()
	executor.On("Execute", mock.Anything, "second").Return(evals.ExecutionOutput{RequestCount: 1}, &evals.ExecutionError{Kind: "rate_limited", LaneStop: true}).Once()
	works := make([]recoveryWork, len(candidates))
	for i, c := range candidates {
		works[i] = recoveryWork{candidate: c, executors: []evals.CaseExecutor{executor}}
	}
	byRun, count, reason, err := executeRecoveryAttempts(t.Context(), works, evals.ExecutionLimits{AttemptTimeout: time.Second, MinInterval: time.Nanosecond}, campaignRecoveryAddendum{}, output, nil)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, "rate_limited", reason)
	result, err := finalizeRecoveryEvidence(campaignRecoveryResult{Status: "partial", StopReason: reason, AdditionalAttempts: count}, prep, byRun, originalPath, output)
	require.NoError(t, err)
	require.Len(t, result.RecoveryEvidencePaths, 1)
	saved, err := evals.ReadCampaignRecovery(result.RecoveryEvidencePaths[0])
	require.NoError(t, err)
	require.Len(t, saved.Attempts, 2)
	require.Len(t, saved.Manifest.Targets, 3)
	require.Equal(t, "rate_limited", saved.Manifest.StopReason)
	require.Equal(t, success.RawOutput, saved.Attempts[0].Attempt.RawOutput)
	require.Equal(t, 3, saved.Manifest.Budget.PriorRequests)
	for _, a := range saved.Attempts {
		require.Equal(t, 1, a.Attempt.RequestCount)
	}
	_, err = os.Stat(result.EvidenceWithRecovery)
	require.NoError(t, err)
	executor.AssertNotCalled(t, "Execute", mock.Anything, "third")
	executor.AssertExpectations(t)
}

func TestRecoveryDispatchZeroRPCRefusalWritesEmptyPartialOverlay(t *testing.T) {
	prep, candidates, _, originalPath, output := cliRecoveryEvidenceFixture(t)
	executor := &recoveryExecutorMock{}
	executor.On("Execute", mock.Anything, "first").Return(evals.ExecutionOutput{}, &evals.ExecutionError{Kind: "budget_exhausted", Stop: true, NotExecuted: true}).Once()
	works := []recoveryWork{{candidate: candidates[0], executors: []evals.CaseExecutor{executor}}, {candidate: candidates[1], executors: []evals.CaseExecutor{executor}}}
	byRun, count, reason, err := executeRecoveryAttempts(t.Context(), works, evals.ExecutionLimits{AttemptTimeout: time.Second}, campaignRecoveryAddendum{}, output, nil)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Empty(t, byRun)
	require.Equal(t, "budget_exhausted", reason)
	result, err := finalizeRecoveryEvidence(campaignRecoveryResult{Status: "partial", StopReason: reason}, prep, byRun, originalPath, output)
	require.NoError(t, err)
	saved, err := evals.ReadCampaignRecovery(result.RecoveryEvidencePaths[0])
	require.NoError(t, err)
	require.Empty(t, saved.Attempts)
	require.Equal(t, reason, saved.Manifest.StopReason)
	executor.AssertNotCalled(t, "Execute", mock.Anything, "second")
	executor.AssertExpectations(t)
}

func TestRecoveryCancellationDuringPacingFlushesPhysicalPrefix(t *testing.T) {
	prep, candidates, success, originalPath, output := cliRecoveryEvidenceFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	executor := &recoveryExecutorMock{}
	executor.On("Execute", mock.Anything, "first").Run(func(mock.Arguments) { go func() { time.Sleep(5 * time.Millisecond); cancel() }() }).Return(success, nil).Once()
	works := []recoveryWork{{candidate: candidates[0], executors: []evals.CaseExecutor{executor}}, {candidate: candidates[1], executors: []evals.CaseExecutor{executor}}}
	byRun, count, reason, err := executeRecoveryAttempts(ctx, works, evals.ExecutionLimits{AttemptTimeout: time.Second, MinInterval: time.Hour}, campaignRecoveryAddendum{}, output, nil)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "canceled", reason)
	result, err := finalizeRecoveryEvidence(campaignRecoveryResult{Status: "partial", StopReason: reason, AdditionalAttempts: count}, prep, byRun, originalPath, output)
	require.NoError(t, err)
	saved, err := evals.ReadCampaignRecovery(result.RecoveryEvidencePaths[0])
	require.NoError(t, err)
	require.Len(t, saved.Attempts, 1)
	require.Equal(t, success.RawOutput, saved.Attempts[0].Attempt.RawOutput)
	data, err := os.ReadFile(result.EvidencePath)
	require.NoError(t, err)
	var summary campaignRecoveryResult
	require.NoError(t, json.Unmarshal(data, &summary))
	require.Equal(t, "partial", summary.Status)
	executor.AssertNotCalled(t, "Execute", mock.Anything, "second")
	executor.AssertExpectations(t)
}

func TestRecoveryCancellationDuringBackoffFlushesPhysicalPrefix(t *testing.T) {
	prep, candidates, _, originalPath, output := cliRecoveryEvidenceFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	executor := &recoveryExecutorMock{}
	executor.On("Execute", mock.Anything, "first").Run(func(mock.Arguments) { go func() { time.Sleep(5 * time.Millisecond); cancel() }() }).Return(evals.ExecutionOutput{RequestCount: 1}, &evals.ExecutionError{Kind: "execution_error", Retryable: true, Cause: fmt.Errorf("503 UNAVAILABLE")}).Once()
	works := []recoveryWork{{candidate: candidates[0], executors: []evals.CaseExecutor{executor, executor}}}
	var addendum campaignRecoveryAddendum
	addendum.Execution.Backoff.InitialSeconds = 30
	addendum.Execution.Backoff.MaxSeconds = 300
	byRun, count, reason, err := executeRecoveryAttempts(ctx, works, evals.ExecutionLimits{AttemptTimeout: time.Second}, addendum, output, nil)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "canceled", reason)
	result, err := finalizeRecoveryEvidence(campaignRecoveryResult{Status: "partial", StopReason: reason, AdditionalAttempts: count}, prep, byRun, originalPath, output)
	require.NoError(t, err)
	saved, err := evals.ReadCampaignRecovery(result.RecoveryEvidencePaths[0])
	require.NoError(t, err)
	require.Len(t, saved.Attempts, 1)
	require.Equal(t, 1, saved.Attempts[0].Attempt.RequestCount)
	executor.AssertExpectations(t)
}
