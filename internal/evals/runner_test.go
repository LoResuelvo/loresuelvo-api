package evals

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
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

func TestConcurrentRunStopsRateLimitedLaneWithoutStoppingOtherProject(t *testing.T) {
	plan := executionTestPlan("first", "second", "third")
	limits := executionTestLimits()
	limits.Concurrency = 2
	journal, record, directory := executionTestJournal(t, plan)
	executor := &laneExecutorMock{}
	executor.On("LaneCount").Return(2).Once()
	executor.On("ExecuteLane", mock.Anything, "first", 0).Return(ExecutionOutput{RequestCount: 1}, &ExecutionError{Kind: "rate_limited", LaneStop: true}).Once()
	executor.On("ExecuteLane", mock.Anything, "second", 1).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	thirdLane := -1
	executor.On("ExecuteLane", mock.Anything, "third", mock.Anything).Run(func(args mock.Arguments) { thirdLane = args.Int(2) }).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	actual, err := Run(t.Context(), &Dataset{}, plan, limits, executor, journal, record, CredentialsRedactor(nil))
	require.NoError(t, err)
	require.Equal(t, "completed", actual.Status)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 3)
	require.Equal(t, 1, thirdLane)
	executor.AssertExpectations(t)
}

func TestConcurrentRunRetriesUseSameLaneAndPacePerProject(t *testing.T) {
	plan := executionTestPlan("first", "second")
	plan.MaxRetries = 1
	plan.RequestLimit = 4
	limits := executionTestLimits()
	limits.Concurrency = 2
	limits.MinInterval = 20 * time.Millisecond
	journal, record, directory := executionTestJournal(t, plan)
	executor := &laneExecutorMock{}
	executor.On("LaneCount").Return(2).Once()
	executor.On("ExecuteLane", mock.Anything, "first", 0).Return(ExecutionOutput{RequestCount: 1}, &ExecutionError{Kind: "execution_error", Retryable: true}).Once()
	executor.On("ExecuteLane", mock.Anything, "first", 0).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	executor.On("ExecuteLane", mock.Anything, "second", 1).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	_, err := Run(t.Context(), &Dataset{}, plan, limits, executor, journal, record, CredentialsRedactor(nil))
	require.NoError(t, err)
	// Measure the scheduler's recorded starts, not delayed mock callbacks.
	_, attempts, readErr := ReadRun(directory)
	require.NoError(t, readErr)
	var starts [2]time.Time
	for _, attempt := range attempts {
		if attempt.CaseID == "first" {
			starts[attempt.Retry] = attempt.StartedOn
		}
	}
	require.False(t, starts[0].IsZero())
	require.False(t, starts[1].IsZero())
	require.GreaterOrEqual(t, starts[1].Sub(starts[0]), limits.MinInterval)

	executor.AssertExpectations(t)
}

func TestConcurrentRunCancelledContextKeepsEveryUnexecutedSlot(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	plan := executionTestPlan("first", "second", "third")
	limits := executionTestLimits()
	limits.Concurrency = 2
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	actual, err := Run(ctx, &Dataset{}, plan, limits, executor, journal, record, CredentialsRedactor(nil))
	require.NoError(t, err)
	require.Equal(t, "partial", actual.Status)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 3)
	for _, a := range attempts {
		require.Equal(t, "not_executed", a.Status)
	}
	executor.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestConcurrentRunJournalFailureStopsNewDispatch(t *testing.T) {
	plan := executionTestPlan("first", "second", "third")
	limits := executionTestLimits()
	limits.Concurrency = 2
	journal, record, _ := executionTestJournal(t, plan)
	// Force Write to fail without closing the file twice in cleanup.
	require.NoError(t, journal.file.Chmod(0400))
	writable := journal.file
	readOnly, err := os.Open(writable.Name())
	require.NoError(t, err)
	journal.file = readOnly
	t.Cleanup(func() { require.NoError(t, writable.Close()) })
	executor := &caseExecutorMock{}
	for _, id := range []string{"first", "second"} {
		executor.On("Execute", mock.Anything, id).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	}
	_, err = Run(t.Context(), &Dataset{}, plan, limits, executor, journal, record, CredentialsRedactor(nil))
	require.ErrorContains(t, err, "append attempt")
	executor.AssertNotCalled(t, "Execute", mock.Anything, "third")
}

func TestConcurrentRunCancellationDuringInFlightCallsDrainsJournal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	plan := executionTestPlan("first", "second", "third")
	limits := executionTestLimits()
	limits.Concurrency = 2
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	firstStarted := make(chan struct{})
	executor.On("Execute", mock.Anything, "first").Run(func(args mock.Arguments) { close(firstStarted); <-args.Get(0).(context.Context).Done() }).Return(ExecutionOutput{RequestCount: 1}, context.Canceled).Once()
	executor.On("Execute", mock.Anything, "second").Run(func(mock.Arguments) { <-firstStarted; cancel() }).Return(ExecutionOutput{RequestCount: 1}, context.Canceled).Once()
	actual, err := Run(ctx, &Dataset{}, plan, limits, executor, journal, record, CredentialsRedactor(nil))
	require.NoError(t, err)
	require.Equal(t, "partial", actual.Status)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 3)
	counts := map[string]int{}
	for _, a := range attempts {
		counts[a.Status]++
	}
	require.Equal(t, 2, counts["execution_error"])
	require.Equal(t, 1, counts["not_executed"])
	executor.AssertExpectations(t)
}

func TestCancelledLaneRemainsAliveUntilJobQueueCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	jobs := make(chan executionSlot, 1)
	events := make(chan laneEvent, 2)
	done := make(chan struct{})
	executor := &caseExecutorMock{}
	var claims atomic.Int64
	var dispatchStopped atomic.Bool
	go func() {
		runLane(ctx, 0, jobs, events, executionTestPlan("case"), executionTestLimits(), executor, &claims, &dispatchStopped)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("worker exited while its job queue was open")
	case <-time.After(20 * time.Millisecond):
	}
	jobs <- executionSlot{caseID: "case", trial: 1}
	select {
	case event := <-events:
		require.Equal(t, "not_executed", event.attempt.Status)
		close(event.ack)
	case <-time.After(time.Second):
		t.Fatal("cancelled queued slot was not reported")
	}
	select {
	case event := <-events:
		require.True(t, event.finished)
	case <-time.After(time.Second):
		t.Fatal("slot completion was not reported")
	}
	close(jobs)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("closed job queue did not release worker")
	}
	executor.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestConcurrentFatalStopPreservesOtherInFlightResponse(t *testing.T) {
	plan := executionTestPlan("first", "second", "third")
	limits := executionTestLimits()
	limits.Concurrency = 2
	journal, record, directory := executionTestJournal(t, plan)
	executor := &caseExecutorMock{}
	secondStarted := make(chan struct{})
	inFlightCancelled := false
	executor.On("Execute", mock.Anything, "first").Run(func(mock.Arguments) { <-secondStarted }).Return(ExecutionOutput{RequestCount: 1}, &ExecutionError{Kind: "provider_configuration_error", Stop: true}).Once()
	executor.On("Execute", mock.Anything, "second").Run(func(args mock.Arguments) {
		close(secondStarted)
		select {
		case <-args.Get(0).(context.Context).Done():
			inFlightCancelled = true
		case <-time.After(20 * time.Millisecond):
		}
	}).Return(ExecutionOutput{RequestCount: 1, RawOutput: "received response"}, nil).Once()
	actual, err := Run(t.Context(), &Dataset{}, plan, limits, executor, journal, record, CredentialsRedactor(nil))
	require.NoError(t, err)
	require.Equal(t, "partial", actual.Status)
	require.False(t, inFlightCancelled)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	for _, a := range attempts {
		if a.CaseID == "second" {
			require.Equal(t, "received response", a.RawOutput)
			require.Equal(t, "executed", a.Status)
		}
	}
	executor.AssertNotCalled(t, "Execute", mock.Anything, "third")
	executor.AssertExpectations(t)
}

func TestConcurrentRunUsesHealthyLaneBeyondConcurrencyPrefix(t *testing.T) {
	plan := executionTestPlan("first", "second")
	limits := executionTestLimits()
	limits.Concurrency = 2
	journal, record, directory := executionTestJournal(t, plan)
	executor := &availableLaneExecutorMock{}
	executor.On("LaneCount").Return(3).Once()
	for lane := 0; lane < 3; lane++ {
		executor.On("LaneAvailable", lane).Return(lane == 2).Once()
	}
	for _, id := range []string{"first", "second"} {
		executor.On("ExecuteLane", mock.Anything, id, 2).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	}
	actual, err := Run(t.Context(), &Dataset{}, plan, limits, executor, journal, record, CredentialsRedactor(nil))
	require.NoError(t, err)
	require.Equal(t, "completed", actual.Status)
	_, attempts, err := ReadRun(directory)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	for _, a := range attempts {
		require.Equal(t, "executed", a.Status)
	}
	executor.AssertExpectations(t)
}

func TestConcurrentRunConsidersAllLanesButBoundsActiveExecutions(t *testing.T) {
	plan := executionTestPlan("first", "second", "third")
	limits := executionTestLimits()
	limits.Concurrency = 2
	journal, record, _ := executionTestJournal(t, plan)
	executor := &availableLaneExecutorMock{}
	executor.On("LaneCount").Return(3).Once()
	for lane := range 3 {
		executor.On("LaneAvailable", lane).Return(true).Once()
	}
	var active, peak atomic.Int32
	secondStarted := make(chan struct{})
	enter := func() {
		current := active.Add(1)
		for old := peak.Load(); current > old; old = peak.Load() {
			if peak.CompareAndSwap(old, current) {
				break
			}
		}
	}
	executor.On("ExecuteLane", mock.Anything, "first", 0).Run(func(mock.Arguments) { enter(); defer active.Add(-1); <-secondStarted }).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	executor.On("ExecuteLane", mock.Anything, "second", 1).Run(func(mock.Arguments) {
		enter()
		defer active.Add(-1)
		close(secondStarted)
		time.Sleep(10 * time.Millisecond)
	}).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	executor.On("ExecuteLane", mock.Anything, "third", 2).Run(func(mock.Arguments) { enter(); defer active.Add(-1) }).Return(ExecutionOutput{RequestCount: 1}, nil).Once()
	_, err := Run(t.Context(), &Dataset{}, plan, limits, executor, journal, record, CredentialsRedactor(nil))
	require.NoError(t, err)
	require.EqualValues(t, 2, peak.Load())
	executor.AssertExpectations(t)
}
