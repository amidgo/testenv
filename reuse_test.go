// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package testenv_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amidgo/testenv"
)

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

func Test_ReuseDaemon_Zero_User_Exit(t *testing.T) {
	t.Parallel()

	waitDuration := time.Second

	called := false
	trm := newMockTerminater(t)
	errDoubleCffCall := errors.New("unexpected, second call to ccf")

	ccf := testenv.ProvideEnvironmentFunc(func(ctx context.Context) (any, error) {
		if called {
			return nil, errDoubleCffCall
		}

		called = true

		return trm, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	daemon := testenv.RunReusableDaemon(ctx, waitDuration, ccf)

	notifyCtx, notify := context.WithCancel(ctx)

	wg := sync.WaitGroup{}

	wg.Add(2)

	go func() {
		defer wg.Done()

		simpleEnterAndExit(t, daemon, notify, trm, waitDuration)
	}()

	go func() {
		defer wg.Done()

		awaitNotifyEnterAndExit(t, daemon, notifyCtx, trm)
	}()

	wg.Wait()
}

func Test_ManyConcurrentEnterAndExit(t *testing.T) {
	t.Parallel()

	waitDuration := time.Second

	called := false
	errDoubleCffCall := errors.New("unexpected, second call to ccf")

	ccf := testenv.ProvideEnvironmentFunc(func(ctx context.Context) (any, error) {
		<-time.After(time.Second * 2)

		if called {
			return false, errDoubleCffCall
		}

		called = true

		return true, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	daemon := testenv.RunReusableDaemon(ctx, waitDuration, ccf)

	count := 1000
	if testing.Short() {
		count = 10
	}

	runConcurrentEnters(t, ctx, daemon, count)
}

func runConcurrentEnters(t *testing.T, ctx context.Context, daemon *testenv.ReusableDaemon, count int) {
	sendCh := make(chan struct{})
	defer close(sendCh)

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	for range count {
		go func() {
			_, err := daemon.Enter(ctx)
			if err != nil {
				cancel(err)
			}

			sleepDuration := time.Duration(rand.IntN(1000)) * time.Millisecond
			<-time.After(sleepDuration)

			sendCh <- struct{}{}
		}()
	}

	entered := 0

	for {
		select {
		case <-ctx.Done():
			t.Fatal(context.Cause(ctx))

			return
		case <-sendCh:
			entered++

			daemon.Exit()

			if entered == count {
				return
			}
		}
	}
}

func simpleEnterAndExit(
	t *testing.T,
	daemon *testenv.ReusableDaemon,
	notify func(),
	expectedEnv any,
	waitDuration time.Duration,
) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	env, err := daemon.Enter(ctx)
	if err != nil {
		t.Fatalf("enter to daemon, expected no error, actual %s", err)
	}

	if !reflect.DeepEqual(expectedEnv, env) {
		t.Fatalf("enter to daemon, expected %+v, actual %+v", expectedEnv, env)
	}

	go daemon.Exit()

	<-time.After(waitDuration / 2)

	notify()
}

func awaitNotifyEnterAndExit(
	t *testing.T,
	daemon *testenv.ReusableDaemon,
	notifyCtx context.Context,
	expectedEnv any,
) {
	<-notifyCtx.Done()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	env, err := daemon.Enter(ctx)
	if err != nil {
		t.Fatalf("enter to daemon, expected no error, actual %s", err)
	}

	if !reflect.DeepEqual(expectedEnv, env) {
		t.Fatalf("enter to daemon, expected %+v, actual %+v", expectedEnv, env)
	}

	daemon.Exit()
}

func Test_ReuseDaemon_Cancel_Root_Context(t *testing.T) {
	t.Parallel()

	t.Run("canceled ctx", canceledCtx)
	t.Run("in time canceled ctx", inTimeCanceledCtx)
}

func canceledCtx(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	rootCtx, rootCancel := context.WithCancel(ctx)

	trm := newMockTerminater(t)

	called := false

	ccf := testenv.ProvideEnvironmentFunc(func(ctx context.Context) (any, error) {
		if called {
			t.Fatal("ccf called twice")
		}

		called = true

		return trm, nil
	})

	waitDuration := time.Duration(-1)

	daemon := testenv.RunReusableDaemon(
		rootCtx,
		waitDuration,
		ccf,
	)

	enterEnv, err := daemon.Enter(ctx)
	if err != nil {
		t.Fatalf("expected no error, actual %+v", err)
	}

	if !reflect.DeepEqual(enterEnv, trm) {
		t.Fatalf("wrong enterEnv, expected %+v, actual %+v", trm, enterEnv)
	}

	rootCancel()

	enterEnv, err = daemon.Enter(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"unexpected error of entering in canceled daemon, expected context.Canceled, actual %+v",
			err,
		)
	}

	if enterEnv != nil {
		t.Fatalf("unexpected env, expected nil, actual %+v", trm)
	}

	daemon.Exit()

	enterEnv, err = daemon.Enter(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"unexpected error of entering in canceled daemon, expected context.Canceled, actual %+v",
			err,
		)
	}
}

func inTimeCanceledCtx(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	waitDuration := time.Millisecond

	ccf := testenv.ProvideEnvironmentFunc(func(ctx context.Context) (any, error) {
		return newMockTerminater(t), nil
	})

	runInTimeCanceledReuseDaemon(
		t,
		ctx,
		waitDuration,
		ccf,
	)
}

func runInTimeCanceledReuseDaemon(
	t *testing.T,
	ctx context.Context,
	waitDuration time.Duration,
	ccf testenv.ProvideEnvironmentFunc,
) {
	rootCtx, rootCancel := context.WithCancel(ctx)

	daemon := testenv.RunReusableDaemon(
		rootCtx,
		waitDuration,
		ccf,
	)

	timeCtx, timeCancel := context.WithTimeout(ctx, time.Millisecond*10)
	t.Cleanup(timeCancel)

	go func() {
		<-timeCtx.Done()
		rootCancel()
	}()

	<-timeCtx.Done()

	_, err := daemon.Enter(ctx)
	switch err {
	case nil:
		daemon.Exit()

		runInTimeCanceledReuseDaemon(
			t,
			ctx,
			waitDuration,
			ccf,
		)
	default:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error, expected context.Canceled, actual %+v", err)
		}

		runInTimeCanceledReuseDaemonWhileCanceled(
			t,
			ctx,
			waitDuration,
			ccf,
		)
	}
}

func runInTimeCanceledReuseDaemonWhileCanceled(
	t *testing.T,
	ctx context.Context,
	waitDuration time.Duration,
	ccf testenv.ProvideEnvironmentFunc,
) {
	rootCtx, rootCancel := context.WithCancel(ctx)

	daemon := testenv.RunReusableDaemon(
		rootCtx,
		waitDuration,
		ccf,
	)

	timeCtx, timeCancel := context.WithTimeout(ctx, time.Millisecond*10)
	t.Cleanup(timeCancel)

	go func() {
		<-timeCtx.Done()
		rootCancel()
	}()

	<-timeCtx.Done()

	_, err := daemon.Enter(ctx)
	switch err {
	case nil:
		daemon.Exit()
	default:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error, expected context.Canceled, actual %+v", err)
		}

		runInTimeCanceledReuseDaemonWhileCanceled(
			t,
			ctx,
			waitDuration,
			ccf,
		)
	}
}
