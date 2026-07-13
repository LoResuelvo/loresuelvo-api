package scheduler

import (
	"context"
	"log/slog"
	"time"
)

type Task interface {
	UrgentNotification(ctx context.Context) error
}

type ticker interface {
	Chan() <-chan time.Time
	Stop()
}

type timeTicker struct {
	*time.Ticker
}

func (t timeTicker) Chan() <-chan time.Time {
	return t.C
}

type Scheduler struct {
	interval  time.Duration
	task      Task
	newTicker func(time.Duration) ticker
}

func NewScheduler(interval time.Duration, task Task) *Scheduler {
	return &Scheduler{
		interval: interval,
		task:     task,
		newTicker: func(interval time.Duration) ticker {
			return timeTicker{Ticker: time.NewTicker(interval)}
		},
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := s.newTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			if err := s.RunOnce(ctx); err != nil {
				slog.Error("scheduler task failed", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	return s.task.UrgentNotification(ctx)
}
