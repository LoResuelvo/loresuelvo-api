package health

import (
	"context"
	"errors"
	"sync/atomic"
)

var errDraining = errors.New("service is draining")

type Pinger interface {
	PingContext(context.Context) error
}

type Readiness struct {
	pinger Pinger
	state  atomic.Uint32
}

const (
	readyState uint32 = iota
	drainingState
)

func NewReadiness(pinger Pinger) *Readiness {
	readiness := &Readiness{pinger: pinger}
	readiness.state.Store(readyState)
	return readiness
}

func (readiness *Readiness) CheckReadiness(ctx context.Context) error {
	if readiness.state.Load() == drainingState {
		return errDraining
	}

	if err := readiness.pinger.PingContext(ctx); err != nil {
		return err
	}

	if readiness.state.Load() == drainingState {
		return errDraining
	}

	return nil
}

func (readiness *Readiness) MarkDraining() {
	readiness.state.Store(drainingState)
}
