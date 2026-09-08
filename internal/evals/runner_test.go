package evals

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunExactBudgetCompletesLastResult(t *testing.T) {
	plan := executionTestPlan("case")
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	executor.On("Execute", mock.Anything, "case").Return(ExecutionOutput{RequestCount: 1, RawOutput: "response"}, nil).Once()
	actual, err := Run(context.Background(), &Dataset{}, plan, executionTestLimits(), executor, journal, record, CredentialRedactor(""))
	require.NoError(t, err)
	require.Equal(t, "completed", actual.Status)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Equal(t, "response", attempts[0].RawOutput)
	executor.AssertExpectations(t)
}
func TestRunBudgetPreservesPartialResults(t *testing.T) {
	plan := executionTestPlan("first", "second")
	plan.RequestLimit = 1
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	executor.On("Execute", mock.Anything, "first").Return(ExecutionOutput{RequestCount: 1, RawOutput: "response"}, nil).Once()
	actual, err := Run(context.Background(), &Dataset{}, plan, executionTestLimits(), executor, journal, record, CredentialRedactor(""))
	require.NoError(t, err)
	require.Equal(t, "partial", actual.Status)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	require.Equal(t, "executed", attempts[0].Status)
	require.Equal(t, "not_executed", attempts[1].Status)
	executor.AssertExpectations(t)
}
func TestRunRetriesAreBounded(t *testing.T) {
	plan := executionTestPlan("case")
	plan.MaxRetries = 2
	plan.RequestLimit = 10
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	executor.On("Execute", mock.Anything, "case").Return(ExecutionOutput{RequestCount: 1}, &ExecutionError{Kind: "execution_error", Retryable: true, Cause: errors.New("transient")}).Times(3)
	_, err := Run(context.Background(), &Dataset{}, plan, executionTestLimits(), executor, journal, record, CredentialRedactor(""))
	require.NoError(t, err)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 3)
	require.Equal(t, 2, attempts[2].Retry)
	executor.AssertExpectations(t)
}
func TestRunCancelledContextSkipsExecutor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	plan := executionTestPlan("case")
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	actual, err := Run(ctx, &Dataset{}, plan, executionTestLimits(), executor, journal, record, CredentialRedactor(""))
	require.NoError(t, err)
	require.Equal(t, "partial", actual.Status)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Equal(t, "not_executed", attempts[0].Status)
	executor.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}
func TestRunGlobalTimeoutPreservesFailedAttempt(t *testing.T) {
	plan := executionTestPlan("first", "second")
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	limits := executionTestLimits()
	limits.GlobalTimeout = time.Millisecond
	limits.AttemptTimeout = time.Millisecond
	executor.On("Execute", mock.Anything, "first").Run(func(args mock.Arguments) { <-args.Get(0).(context.Context).Done() }).Return(ExecutionOutput{RequestCount: 1}, context.DeadlineExceeded).Once()
	actual, err := Run(context.Background(), &Dataset{}, plan, limits, executor, journal, record, CredentialRedactor(""))
	require.NoError(t, err)
	require.Equal(t, "partial", actual.Status)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Equal(t, "execution_error", attempts[0].Status)
	require.Equal(t, "not_executed", attempts[1].Status)
	executor.AssertExpectations(t)
}
func TestRunScrubsCredentialsBeforePersistence(t *testing.T) {
	const secret = "test-secret"
	plan := executionTestPlan("case")
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	executor.On("Execute", mock.Anything, "case").Return(ExecutionOutput{Input: []byte(`[{"parts":[{"text":"test-secret"}]}]`), RawOutput: secret, ProviderResponse: []byte(`{"text":"test-secret"}`)}, errors.New(secret)).Once()
	_, err := Run(context.Background(), &Dataset{}, plan, executionTestLimits(), executor, journal, record, CredentialRedactor(secret))
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(directory, "attempts.jsonl"))
	require.NoError(t, err)
	require.NotContains(t, string(data), secret)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Equal(t, digest(attempts[0].Input), attempts[0].InputSHA256)
	hash, hashErr := promptHash(attempts[0].Input)
	require.NoError(t, hashErr)
	require.Equal(t, hash, attempts[0].PromptSHA256)
	executor.AssertExpectations(t)
}
func TestRunPreservesAssetErrorKind(t *testing.T) {
	plan := executionTestPlan("case")
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	executor.On("Execute", mock.Anything, "case").Return(ExecutionOutput{}, &ExecutionError{Kind: "asset_error", Cause: os.ErrNotExist}).Once()
	_, err := Run(context.Background(), &Dataset{}, plan, executionTestLimits(), executor, journal, record, CredentialRedactor(""))
	require.NoError(t, err)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Equal(t, "asset_error", attempts[0].Status)
	require.Zero(t, attempts[0].RequestCount)
	executor.AssertExpectations(t)
}

func TestRunAttemptTimeoutAllowsNextCase(t *testing.T) {
	plan := executionTestPlan("first", "second")
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	limits := executionTestLimits()
	limits.AttemptTimeout = time.Millisecond
	executor.On("Execute", mock.Anything, "first").Run(func(args mock.Arguments) { <-args.Get(0).(context.Context).Done() }).Return(ExecutionOutput{RequestCount: 1}, context.DeadlineExceeded).Once()
	executor.On("Execute", mock.Anything, "second").Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	_, err := Run(context.Background(), &Dataset{}, plan, limits, executor, journal, record, CredentialRedactor(""))
	require.NoError(t, err)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	require.Equal(t, "execution_error", attempts[0].Status)
	require.Equal(t, "executed", attempts[1].Status)
	executor.AssertExpectations(t)
}

func TestRunRequestBudgetCapsRetries(t *testing.T) {
	plan := executionTestPlan("first", "second")
	plan.MaxRetries = 5
	plan.RequestLimit = 2
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	executor.On("Execute", mock.Anything, "first").Return(ExecutionOutput{RequestCount: 1}, &ExecutionError{Kind: "execution_error", Retryable: true}).Times(2)
	_, err := Run(context.Background(), &Dataset{}, plan, executionTestLimits(), executor, journal, record, CredentialRedactor(""))
	require.NoError(t, err)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 3)
	require.Equal(t, "not_executed", attempts[2].Status)
	executor.AssertExpectations(t)
}

func TestCredentialRedactorMatchesTransmittedTrimmedKey(t *testing.T) {
	require.Equal(t, "provider: [REDACTED]", CredentialRedactor("  secret-key \n")("provider: secret-key"))
}
