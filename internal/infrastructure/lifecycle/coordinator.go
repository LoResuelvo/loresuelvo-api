package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const DefaultShutdownTimeout = 30 * time.Second

type HTTPServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
	Close() error
}

type Readiness interface {
	MarkDraining()
}

type Hub interface {
	Close()
}

type Worker func(context.Context) error

type Config struct {
	Server          HTTPServer
	Readiness       Readiness
	Hub             Hub
	Workers         []Worker
	Closers         []io.Closer
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

type Coordinator struct {
	server          HTTPServer
	readiness       Readiness
	hub             Hub
	workers         []Worker
	closers         []io.Closer
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

func NewCoordinator(config Config) (*Coordinator, error) {
	if config.Server == nil {
		return nil, errors.New("lifecycle server is required")
	}
	if config.Readiness == nil {
		return nil, errors.New("lifecycle readiness is required")
	}

	shutdownTimeout := config.ShutdownTimeout
	if shutdownTimeout == 0 {
		var err error
		shutdownTimeout, err = ShutdownTimeoutFromEnv()
		if err != nil {
			return nil, err
		}
	}
	if shutdownTimeout <= 0 {
		return nil, errors.New("shutdown timeout must be greater than zero")
	}

	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Coordinator{
		server:          config.Server,
		readiness:       config.Readiness,
		hub:             config.Hub,
		workers:         append([]Worker(nil), config.Workers...),
		closers:         append([]io.Closer(nil), config.Closers...),
		shutdownTimeout: shutdownTimeout,
		logger:          logger,
	}, nil
}

func ShutdownTimeoutFromEnv() (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT"))
	if value == "" {
		return DefaultShutdownTimeout, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration: %w", err)
	}
	if timeout <= 0 {
		return 0, errors.New("SHUTDOWN_TIMEOUT must be greater than zero")
	}
	return timeout, nil
}

func (coordinator *Coordinator) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	workerContext, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()

	workerErrors := make(chan error, len(coordinator.workers))
	workerDone := make(chan struct{})
	var workers sync.WaitGroup
	for _, worker := range coordinator.workers {
		workers.Add(1)
		go func(worker Worker) {
			defer workers.Done()
			if err := worker(workerContext); err != nil && workerContext.Err() == nil {
				workerErrors <- err
			}
		}(worker)
	}
	go func() {
		workers.Wait()
		close(workerDone)
	}()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- coordinator.server.ListenAndServe()
	}()

	var runErr error
	serverResultReceived := false
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		serverResultReceived = true
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("running HTTP server: %w", err)
		}
	case err := <-workerErrors:
		runErr = fmt.Errorf("running background worker: %w", err)
	}

	shutdownErr := coordinator.shutdown(cancelWorkers, workerDone, serverErrors, serverResultReceived)
	return errors.Join(runErr, shutdownErr)
}

func (coordinator *Coordinator) shutdown(
	cancelWorkers context.CancelFunc,
	workerDone <-chan struct{},
	serverErrors <-chan error,
	serverResultReceived bool,
) error {
	coordinator.readiness.MarkDraining()
	cancelWorkers()
	if coordinator.hub != nil {
		coordinator.hub.Close()
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), coordinator.shutdownTimeout)
	shutdownErr := coordinator.server.Shutdown(shutdownContext)
	cancelShutdown()
	if shutdownErr != nil {
		coordinator.logger.Warn("HTTP server graceful shutdown timed out; forcing close", "error", shutdownErr)
		if closeErr := coordinator.server.Close(); closeErr != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("forcing HTTP server close: %w", closeErr))
		}
	}

	if !serverResultReceived {
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("HTTP server stopped during shutdown: %w", err))
		}
	}
	<-workerDone

	for _, closer := range coordinator.closers {
		if err := closer.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("closing lifecycle resource: %w", err))
		}
	}
	return shutdownErr
}
