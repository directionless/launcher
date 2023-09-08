package rungroup

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// Onceler is designed to take an agent (or function) that exposes a once style interface, and will repeatedly run it.
// error handling is a bit odd. If rungroup holds the run loop, how does the agent signal a fatal error? And how does
// the agent clean up resources on termination?
//
// onceFunc will be run periodically. Errors are fatal -- they signal shutdown of the entire rungroup. Anything
// not fatal should be handled by the object directly.

type onceAgent interface {
	Name() string
	Once() error
	Cleanup() error // May be called more than once.
	Period() time.Duration
}

type onceler struct {
	logger    log.Logger
	interrupt chan struct{}

	agent onceAgent
}

func NewOnceler(logger log.Logger, agent onceAgent) *onceler {
	return &onceler{
		logger:    log.With(logger, "name", agent.Name()),
		interrupt: make(chan struct{}, 1),
		agent:     agent,
	}
}
func (a *onceler) Name() string {
	return a.agent.Name()
}

func (a *onceler) Run() error {
	// TODO: Can we share these between oncelers? But doing that starts to require some more advanced ticker
	// work. See https://github.com/VividCortex/multitick/blob/master/multitick.go for an implementation.
	heartbeatTicker := time.NewTicker(60 * time.Minute)
	defer heartbeatTicker.Stop()

	// might even be nice to figure out how to merge out tickets into once big ticker.
	ticker := time.NewTicker(a.agent.Period())
	defer ticker.Stop()

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
		case <-a.interrupt:
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

func (a *onceler) Interrupt(_ error) {
	select {
	case a.interrupt <- struct{}{}:
		// sent
	default:
		// channel is full. Probably another caller
	}
}
