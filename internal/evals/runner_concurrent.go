package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type laneExecutor interface {
	LaneCount() int
	ExecuteLane(context.Context, string, int) (ExecutionOutput, error)
}

func executeLane(executor CaseExecutor, ctx context.Context, id string, lane int) (ExecutionOutput, error) {
	if lanes, ok := executor.(laneExecutor); ok {
		return lanes.ExecuteLane(ctx, id, lane)
	}
	return executor.Execute(ctx, id)
}

type executionSlot struct {
	caseID string
	trial  int
}
type laneEvent struct {
	lane     int
	attempt  *Attempt
	err      error
	finished bool
	stopLane bool
	ack      chan struct{}
}

// runConcurrent keeps all durable writes on the coordinator. Workers wait for
// its acknowledgement before retrying or accepting another logical slot.
func runConcurrent(ctx context.Context, _ *Dataset, plan *Plan, limits ExecutionLimits, executor CaseExecutor, journal *Journal, record RunRecord, redact func(string) string) (RunRecord, error) {
	laneCount := limits.Concurrency
	if lanes, ok := executor.(laneExecutor); ok {
		laneCount = lanes.LaneCount()
		if limits.Concurrency > laneCount {
			return record, fmt.Errorf("concurrency exceeds configured independent project lanes")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, limits.GlobalTimeout)
	defer cancel()
	slots := make([]executionSlot, 0, len(plan.Cases)*plan.Trials)
	for _, c := range plan.Cases {
		for trial := 1; trial <= plan.Trials; trial++ {
			slots = append(slots, executionSlot{c.CaseID, trial})
		}
	}
	events := make(chan laneEvent, limits.Concurrency)
	jobs := make([]chan executionSlot, laneCount)
	var claims atomic.Int64
	var dispatchStopped atomic.Bool
	var workers sync.WaitGroup
	for lane := range laneCount {
		jobs[lane] = make(chan executionSlot, 1)
		workers.Go(func() { runLane(ctx, lane, jobs[lane], events, plan, limits, executor, &claims, &dispatchStopped) })
	}
	next, active, notExecuted := 0, 0, 0
	usedRequests := 0
	available := make([]int, 0, laneCount)
	for i := range laneCount {
		if lanes, ok := executor.(interface{ LaneAvailable(int) bool }); !ok || lanes.LaneAvailable(i) {
			available = append(available, i)
		}
	}
	stopReason := ""
	var journalErr error
	for next < len(slots) || active > 0 {
		if ctx.Err() != nil && stopReason == "" {
			stopReason = ctx.Err().Error()
		}
		for stopReason == "" && next < len(slots) && len(available) > 0 && active < limits.Concurrency {
			lane := available[0]
			available = available[1:]
			jobs[lane] <- slots[next]
			next++
			active++
		}
		if active == 0 {
			if next < len(slots) && stopReason == "" {
				stopReason = "all project lanes stopped"
			}
			break
		}
		event := <-events
		if event.attempt != nil {
			a := *event.attempt
			usedRequests += a.RequestCount
			if err := sanitizeAttempt(&a, redact); err != nil {
				journalErr = errors.Join(journalErr, err)
				cancel()
				stopReason = "journal failure"
			} else if journalErr == nil {
				if err := journal.Append(a); err != nil {
					journalErr = err
					cancel()
					stopReason = "journal failure"
				}
			}
			if a.Status == "not_executed" {
				notExecuted++
			}
			var classified *ExecutionError
			if errors.As(event.err, &classified) && classified.Stop {
				if stopReason == "" {
					stopReason = classified.Kind
				}
				dispatchStopped.Store(true)
			}
			if stopReason == "" && usedRequests >= plan.RequestLimit {
				stopReason = "request budget exhausted"
				dispatchStopped.Store(true)
			}
			close(event.ack)
		}
		if event.finished {
			active--
			if !event.stopLane {
				available = append(available, event.lane)
			}
		}
	}
	for _, queue := range jobs {
		close(queue)
	}
	cancel()
	workers.Wait()
	if journalErr != nil {
		return record, journalErr
	}
	for _, slot := range slots[next:] {
		notExecuted++
		if err := journal.Append(Attempt{CaseID: slot.caseID, Trial: slot.trial, Status: "not_executed", Error: stopReason}); err != nil {
			return record, err
		}
	}
	finished := time.Now().UTC()
	record.FinishedOn = &finished
	record.Status = "completed"
	if notExecuted > 0 {
		record.Status = "partial"
	}
	record.ReleaseApproved = false
	return record, journal.Finish(&record)
}

func runLane(ctx context.Context, lane int, jobs <-chan executionSlot, events chan<- laneEvent, plan *Plan, limits ExecutionLimits, executor CaseExecutor, claims *atomic.Int64, dispatchStopped *atomic.Bool) {
	nextStart := time.Time{}
	for slot := range jobs {
		stopLane := false
		for retry := 0; retry <= plan.MaxRetries; retry++ {
			a := Attempt{CaseID: slot.caseID, Trial: slot.trial, Retry: retry, Status: "not_executed"}
			var callErr error
			if callErr = waitUntil(ctx, nextStart); callErr == nil && dispatchStopped.Load() {
				callErr = &ExecutionError{Kind: "dispatch_stopped", Stop: true}
			}
			if callErr == nil {
				for {
					used := claims.Load()
					if used >= int64(plan.RequestLimit) {
						callErr = &ExecutionError{Kind: "budget_exhausted", Stop: true, Cause: ErrRequestBudget}
						break
					}
					if claims.CompareAndSwap(used, used+1) {
						break
					}
				}
			}
			if callErr == nil {
				started := time.Now().UTC()
				nextStart = started.Add(limits.MinInterval)
				attemptCtx, cancelAttempt := context.WithTimeout(ctx, limits.AttemptTimeout)
				output, err := executeLane(executor, attemptCtx, slot.caseID, lane)
				cancelAttempt()
				callErr = err
				claims.Add(int64(output.RequestCount) - 1)
				a = attemptFromOutput(output, slot.caseID, slot.trial, retry, started, err)
			} else {
				a.Error = callErr.Error()
			}
			ack := make(chan struct{})
			events <- laneEvent{lane: lane, attempt: &a, err: callErr, ack: ack}
			<-ack
			var classified *ExecutionError
			if errors.As(callErr, &classified) {
				stopLane = classified.LaneStop
			}
			if callErr == nil || ctx.Err() != nil || dispatchStopped.Load() || stopLane || !errors.As(callErr, &classified) || classified.Stop || !classified.Retryable {
				break
			}
		}
		events <- laneEvent{lane: lane, finished: true, stopLane: stopLane}
	}
}

func attemptFromOutput(output ExecutionOutput, id string, trial, retry int, started time.Time, err error) Attempt {
	a := Attempt{Provider: output.Provider, ProviderUsage: output.ProviderUsage, UsageSource: output.UsageSource, HTTPStatus: output.HTTPStatus, RequestID: output.RequestID, CaseID: id, Trial: trial, Retry: retry, StartedOn: started, LatencyMillis: time.Since(started).Milliseconds(), Status: "executed", Input: output.Input, PromptSHA256: output.PromptSHA256, GenerationConfig: output.GenerationConfig, RawOutput: output.RawOutput, ParsedOutput: output.ParsedOutput, ProviderResponse: output.ProviderResponse, RequestCount: output.RequestCount}
	if err != nil {
		a.Status = "execution_error"
		a.Error = err.Error()
		var classified *ExecutionError
		if errors.As(err, &classified) {
			if output.RequestCount == 0 && classified.NotExecuted {
				a.Status = "not_executed"
			} else if classified.Kind == "asset_error" {
				a.Status = "asset_error"
			}
		}
	}
	return a
}
func sanitizeAttempt(a *Attempt, redact func(string) string) error {
	encoded, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if err = json.Unmarshal([]byte(redact(string(encoded))), a); err != nil {
		return err
	}
	if len(a.Input) > 0 {
		a.InputSHA256 = digest(a.Input)
		a.PromptSHA256, err = promptHash(a.Input)
	}
	return err
}

// RedactAttempt sanitizes all persisted strings and recalculates input/prompt
// hashes after redaction. Recovery shares the same boundary as ordinary runs.
func RedactAttempt(a Attempt, keys []string) (Attempt, error) {
	err := sanitizeAttempt(&a, CredentialsRedactor(keys))
	return a, err
}

func firstAvailableLane(executor CaseExecutor) int {
	if lanes, ok := executor.(interface {
		LaneCount() int
		LaneAvailable(int) bool
	}); ok {
		for i := 0; i < lanes.LaneCount(); i++ {
			if lanes.LaneAvailable(i) {
				return i
			}
		}
		return -1
	}
	return 0
}
