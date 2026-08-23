package scheduler

import "context"

type CalendarSyncTask interface {
	Sync(context.Context) error
}

type CalendarSyncRunner struct {
	task CalendarSyncTask
}

func NewCalendarSyncRunner(task CalendarSyncTask) *CalendarSyncRunner {
	return &CalendarSyncRunner{task: task}
}

func (runner *CalendarSyncRunner) RunOnce(ctx context.Context) error {
	return runner.task.Sync(ctx)
}
