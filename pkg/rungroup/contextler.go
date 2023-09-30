package rungroup

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type contextler struct {
	logger  log.Logger
	cancel  func(error)
	running bool
	agent   onceAgent
}

func NewContextler(logger log.Logger, agent onceAgent) *contextler {
	return &contextler{
		logger: log.With(logger, "name", agent.Name()),
		agent:  agent,
	}
}

func (a *contextler) Running() bool {
	return a.running
}

func (a *contextler) Name() string {
	return a.agent.Name()
}

func (a *contextler) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	a.cancel = cancel

	// TODO: Can we share these between oncelers? But doing that starts to require some more advanced ticker
	// work. See https://github.com/VividCortex/multitick/blob/master/multitick.go for an implementation.
	heartbeatTicker := time.NewTicker(60 * time.Minute)
	defer heartbeatTicker.Stop()

	// might even be nice to figure out how to merge out tickets into once big ticker.
	var ticker *time.Ticker
	if a.agent.Period() > 0 {
		ticker = time.NewTicker(a.agent.Period())
		defer ticker.Stop()
	}

	a.running = true

	// This is run here, and not in the for loop, because it makes it heartbeatTicker handling simple
	if err := a.agent.Once(); err != nil {
		return err
	}

	for {
		select {
		case <-ticker.C:
			if err := a.agent.Once(); err != nil {
				return fmt.Errorf("%s once error: %w", a.agent.Name(), err)
			}
		case <-ctx.Done():
			level.Debug(a.logger).Log("msg", "interrupt received, exiting execute loop and signaling cleanup")
			if err := a.agent.Cleanup(); err != nil {
				level.Debug(a.logger).Log("msg", "error cleaning up", "err", err)
			}
			return nil
		case <-heartbeatTicker.C:
			level.Debug(a.logger).Log("msg", "still running", "name", a.agent.Name())
		}
	}
}

func (a *contextler) Interrupt(err error) {
	defer func() { a.running = false }()
	if a.cancel == nil {
		// Interrupt may be called before we've even started. In that case, cancel is nil. This seems to only happen
		// in tests, that don't pause after starting.
		return
	}

	a.cancel(err)
}
