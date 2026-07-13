package scheduler

import (
	"context"
	"time"
)

type Task interface {
	UrgentNotification() error
}

type scheduler struct {
	interval time.Duration
	task     Task
}

func NewScheduler(interval time.Duration, task Task) *scheduler {
	return &scheduler{
		interval: interval,
		task:     task,
	}
}

func (s *scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.executeTask()
		case <-ctx.Done():
			return
		}
	}
}

func (s *scheduler) executeTask() {
	if err := s.task.UrgentNotification(); err != nil {
		// TODO: LOGGER aqui.
		return
	}
}
