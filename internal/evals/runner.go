package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ExecutionLimits struct {
	Concurrency     int           `json:"concurrency"`
	AttemptTimeout  time.Duration `json:"attempt_timeout_ns"`
	GlobalTimeout   time.Duration `json:"global_timeout_ns"`
	MinInterval     time.Duration `json:"min_interval_ns"`
	MaxOutputTokens int32         `json:"max_output_tokens"`
}

func (l ExecutionLimits) Validate() error {
	if l.Concurrency != 1 {
		return fmt.Errorf("this runner supports concurrency=1 only")
	}
	if l.AttemptTimeout <= 0 || l.GlobalTimeout <= 0 || l.MinInterval <= 0 || l.MaxOutputTokens <= 0 {
		return fmt.Errorf("timeouts, request interval and output token limit must be positive")
	}
	if l.AttemptTimeout > l.GlobalTimeout {
		return fmt.Errorf("attempt timeout exceeds global timeout")
	}
	return nil
}

type ExecutionOutput struct {
	RequestID        *string
	Input            json.RawMessage
	PromptSHA256     string
	GenerationConfig json.RawMessage
	RawOutput        string
	ParsedOutput     json.RawMessage
	ProviderResponse json.RawMessage
	RequestCount     int
}

type CaseExecutor interface {
	Execute(context.Context, string) (ExecutionOutput, error)
}

// ExecutionError separates technical failures from model quality failures.
type ExecutionError struct {
	Kind      string
	Stop      bool
	Retryable bool
	Cause     error
}

func (e *ExecutionError) Error() string {
	if e.Cause == nil {
		return e.Kind
	}
	return e.Kind + ": " + e.Cause.Error()
}
func (e *ExecutionError) Unwrap() error { return e.Cause }

// Run executes an already authorized plan. Construction of live dependencies is
// deliberately outside this function; deterministic tests inject a small port.
func Run(ctx context.Context, dataset *Dataset, plan *Plan, limits ExecutionLimits, executor CaseExecutor, journal *Journal, record RunRecord, redact func(string) string) (RunRecord, error) {
	if err := limits.Validate(); err != nil {
		return record, err
	}
	ctx, cancel := context.WithTimeout(ctx, limits.GlobalTimeout)
	defer cancel()
	var stopReason string
	notExecuted := 0
	usedRequests := 0
	nextStart := time.Time{}
	for _, c := range plan.Cases {
		for trial := 1; trial <= plan.Trials; trial++ {
			if ctx.Err() != nil {
				stopReason = ctx.Err().Error()
			}
			if usedRequests >= plan.RequestLimit {
				stopReason = "request budget exhausted"
			}
			if stopReason != "" {
				notExecuted++
				a := Attempt{CaseID: c.CaseID, Trial: trial, Status: "not_executed", Error: stopReason}
				if err := journal.Append(a); err != nil {
					return record, err
				}
				continue
			}
			for retry := 0; retry <= plan.MaxRetries; retry++ {
				if err := waitUntil(ctx, nextStart); err != nil {
					stopReason = err.Error()
					notExecuted++
					if writeErr := journal.Append(Attempt{CaseID: c.CaseID, Trial: trial, Retry: retry, Status: "not_executed", Error: stopReason}); writeErr != nil {
						return record, writeErr
					}
					break
				}
				started := time.Now().UTC()
				nextStart = started.Add(limits.MinInterval)
				attemptCtx, cancelAttempt := context.WithTimeout(ctx, limits.AttemptTimeout)
				output, err := executor.Execute(attemptCtx, c.CaseID)
				cancelAttempt()
				usedRequests += output.RequestCount
				a := Attempt{RequestID: output.RequestID, CaseID: c.CaseID, Trial: trial, Retry: retry, StartedOn: started, LatencyMillis: time.Since(started).Milliseconds(), Status: "executed", Input: output.Input, PromptSHA256: output.PromptSHA256, GenerationConfig: output.GenerationConfig, RawOutput: output.RawOutput, ParsedOutput: output.ParsedOutput, ProviderResponse: output.ProviderResponse, RequestCount: output.RequestCount}
				if len(a.Input) > 0 {
					a.InputSHA256 = digest(a.Input)
				}
				if err != nil {
					a.Status = "execution_error"
					var classified *ExecutionError
					if errors.As(err, &classified) && classified.Kind == "asset_error" {
						a.Status = "asset_error"
					}
					a.Error = redact(err.Error())
				}
				// Scrub the configured credential from all strings before persistence.
				encoded, encodeErr := json.Marshal(a)
				if encodeErr != nil {
					return record, encodeErr
				}
				if decodeErr := json.Unmarshal([]byte(redact(string(encoded))), &a); decodeErr != nil {
					return record, decodeErr
				}
				if len(a.Input) > 0 {
					a.InputSHA256 = digest(a.Input)
					hash, hashErr := promptHash(a.Input)
					if hashErr != nil {
						return record, hashErr
					}
					a.PromptSHA256 = hash
				}
				if appendErr := journal.Append(a); appendErr != nil {
					return record, appendErr
				}
				var executionErr *ExecutionError
				if errors.As(err, &executionErr) && executionErr.Stop {
					stopReason = executionErr.Kind
				}
				if usedRequests >= plan.RequestLimit {
					stopReason = "request budget exhausted"
				}
				if err == nil || stopReason != "" || !errors.As(err, &executionErr) || !executionErr.Retryable {
					break
				}
			}
		}
	}
	finished := time.Now().UTC()
	record.FinishedOn = &finished
	record.Status = "completed"
	if notExecuted > 0 {
		record.Status = "partial"
	}
	record.ReleaseApproved = false
	if err := journal.Finish(&record); err != nil {
		return record, err
	}
	return record, nil
}
func waitUntil(ctx context.Context, when time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	delay := time.Until(when)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func CredentialRedactor(secret string) func(string) string {
	secret = strings.TrimSpace(secret)
	return func(text string) string {
		if secret == "" {
			return text
		}
		return strings.ReplaceAll(text, secret, "[REDACTED]")
	}
}
