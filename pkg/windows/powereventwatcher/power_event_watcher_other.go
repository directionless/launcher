//go:build !windows
// +build !windows

package powereventwatcher

import (
	"time"

	"github.com/go-kit/kit/log"
	"github.com/kolide/launcher/pkg/agent/types"
)

type noOpPowerEventWatcher struct {
	interrupt chan struct{}
}

func New(_ types.Knapsack, _ log.Logger) (*noOpPowerEventWatcher, error) {
	return &noOpPowerEventWatcher{
		interrupt: make(chan struct{}),
	}, nil
}

// Once is a no-op, since we've already registered our subscription
func (n *noOpPowerEventWatcher) Once() error {
	return nil
}

func (n *noOpPowerEventWatcher) Cleanup() error {
	return nil
}

func (n *noOpPowerEventWatcher) Name() string {
	return "powerEventWatcherNoop"
}

func (n *noOpPowerEventWatcher) Period() time.Duration {
	return 0
}
