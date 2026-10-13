package evals

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrProjectLaneStopped = errors.New("project lane stopped after rate limit")

// projectLane shares minimum spacing across CountTokens, models and phases.
// Credentials remain only in executor memory and are never used as evidence IDs.
type projectLane struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
	stopped  bool
}

func (l *projectLane) wait(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		return ErrProjectLaneStopped
	}
	if err := waitUntil(ctx, l.next); err != nil {
		return err
	}
	l.next = time.Now().Add(l.interval)
	return nil
}
func (l *projectLane) stop() { l.mu.Lock(); defer l.mu.Unlock(); l.stopped = true }

// ShareGeminiProjectLanes keeps project pacing and 429 state across sequential
// campaign phases/models, including the preflight counting operations.
func ShareGeminiProjectLanes(executors []*GeminiExecutor) {
	lanes := map[string]*projectLane{}
	for _, executor := range executors {
		for i, key := range executor.apiKeys {
			lane, exists := lanes[key]
			if !exists {
				lane = executor.lanes[i]
				lanes[key] = lane
			}
			lane.interval = max(lane.interval, executor.limits.MinInterval)
			executor.lanes[i] = lane
		}
	}
}

func (l *projectLane) available() bool { l.mu.Lock(); defer l.mu.Unlock(); return !l.stopped }
