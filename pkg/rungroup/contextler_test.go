package rungroup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/kolide/launcher/pkg/backoff"
	"github.com/stretchr/testify/require"
)

type testingAgent struct {
	name       string
	period     time.Duration
	count      int
	cleanCount int
	//countLock   sync.Mutex
	ReturnError error
}

func (a *testingAgent) Name() string {
	if a.name == "" {
		return "testing_agent"
	}

	return a.name
}

func (a *testingAgent) Once() error {
	a.count += 1
	return a.ReturnError
}

func (a *testingAgent) Cleanup() error {
	a.cleanCount += 1
	return nil
}

func (a *testingAgent) Period() time.Duration {
	return a.period
}

func Test_OneAgent(t *testing.T) {
	t.Parallel()

	a := NewContextler(log.NewNopLogger(), &testingAgent{period: time.Second})

	ctx, ctxCancel := context.WithCancelCause(context.Background())

	var tests = map[string]struct {
		stopFunc func(error)
	}{
		"agent interrupt":       {stopFunc: a.Interrupt},
		"parent context cancel": {stopFunc: ctxCancel},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(T *testing.T) {
			exited := false
			var exitErr error

			go func() {
				exitErr = a.Run(ctx)
				exited = true
			}()

			backoff.WaitFor(
				func() error {
					if a.Running() {
						return nil
					}
					return errors.New("starting")

				},
				10*time.Second,
				10*time.Millisecond,
			)

			require.False(t, exited)
			require.True(t, a.Running())

			tt.stopFunc(errors.New("x"))
			time.Sleep(10 * time.Millisecond)

			require.True(t, exited)
			require.False(t, a.Running())

			require.Nil(t, exitErr)

		})
	}

}
