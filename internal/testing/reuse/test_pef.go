// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package reuse

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amidgo/testenv"

	"golang.org/x/sync/errgroup"
)

var daemon = testenv.RunReusableDaemon(context.Background(), time.Second, pef())

func pef() testenv.ProvideEnvironmentFunc {
	counter := atomic.Int64{}
	errDoubleCcfCall := errors.New("double call of provide environment func")

	return testenv.ProvideEnvironmentFunc(func(ctx context.Context) (any, error) {
		defer func() { counter.Add(1) }()

		switch counter.Load() {
		case 0:
		case 1:
			return 0, fmt.Errorf("%s, %d, %w", debug.Stack(), os.Getpid(), errDoubleCcfCall)
		default:
			return 0, fmt.Errorf("%s, %d", errDoubleCcfCall, os.Getpid())
		}

		return 0, nil
	})
}

type mockTerminater struct {
	t          *testing.T
	terminated atomic.Bool
}

func newMockTerminater(t *testing.T) *mockTerminater {
	term := &mockTerminater{
		t: t,
	}

	t.Cleanup(term.assert)

	return term
}

func (m *mockTerminater) Terminate(context.Context) error {
	swapped := m.terminated.CompareAndSwap(false, true)

	if !swapped {
		m.t.Fatal("mockTerminater.Terminate called twice")
	}

	return nil
}

func (m *mockTerminater) assert() {
	terminated := m.terminated.Load()

	if !terminated {
		m.t.Fatal("assert mockTerminater is terminated failed")
	}
}

func ReuseDaemon_Zero_User_Exit(t *testing.T) {
	t.Parallel()

	waitDuration := time.Second

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	notifyCtx, notify := context.WithCancel(ctx)

	errgr := errgroup.Group{}
	errgr.SetLimit(2)

	errgr.Go(func() error {
		return simpleEnterAndExit(notify, waitDuration)
	})

	errgr.Go(func() error {
		return awaitNotifyEnterAndExit(t, notifyCtx)
	})

	err := errgr.Wait()
	if err != nil {
		t.Fatal(err)
	}
}

func simpleEnterAndExit(
	notify func(),
	waitDuration time.Duration,
) error {
	ctx := context.Background()
	defer notify()

	_, err := daemon.Enter(ctx)
	if err != nil {
		return fmt.Errorf("enter to daemon, expected no error, actual %w", err)
	}

	go daemon.Exit()

	<-time.After(waitDuration / 2)

	return nil
}

func awaitNotifyEnterAndExit(
	t *testing.T,
	notifyCtx context.Context,
) error {
	<-notifyCtx.Done()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, err := daemon.Enter(ctx)
	if err != nil {
		return fmt.Errorf("enter to daemon, expected no error, actual %w", err)
	}

	daemon.Exit()

	return nil
}
