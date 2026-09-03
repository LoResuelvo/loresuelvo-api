package bootstrap

import (
	"context"
	"io"
	"log/slog"

	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/lifecycle"
)

func (runtime RuntimeDependencies) lifecycleConfig(
	server lifecycle.HTTPServer,
	database io.Closer,
	logger *slog.Logger,
) lifecycle.Config {
	return lifecycle.Config{
		Server:    server,
		Readiness: runtime.Readiness,
		Hub:       runtime.Hub,
		Closers:   []io.Closer{database},
		Workers: []lifecycle.Worker{
			func(ctx context.Context) error {
				runtime.UrgentWorkOrderScheduler.Run(ctx)
				return nil
			},
			func(ctx context.Context) error {
				runtime.CalendarSyncRunner.Run(ctx)
				return nil
			},
			runtime.RealtimeDispatcher.Run,
		},
		Logger: logger,
	}
}

func (runtime RuntimeDependencies) Close() {
	runtime.Hub.Close()
}
