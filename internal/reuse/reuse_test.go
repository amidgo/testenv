// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package reuse

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type closer struct {
	closeCh chan struct{}
	err     error
}

func (c *closer) Close() error {
	close(c.closeCh)

	return c.err
}

type testingTrace[T any] struct{ t *testing.T }

func (t *testingTrace[T]) TraceChangePhase(oldPhase, newPhase phase[T]) {
	t.t.Logf("phase changed, old: %s, new: %s", oldPhase, newPhase)
}

func (t *testingTrace[T]) TraceCommandReceived(receiverPhase phase[T], cmd command[T]) {
	t.t.Logf("command received, phase: %s, cmd: %s", receiverPhase, cmd)
}

func Test_Reuse_base_cleanup_after_exit_closer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const waitUntilCleanup = time.Second

		rse := RunForTesting(t,
			func() (*closer, error) {
				return &closer{
					closeCh: make(chan struct{}),
					err:     error(nil),
				}, nil
			},
			WithTracer(&testingTrace[*closer]{t: t}),
			WithWaitUntilCleanup[*closer](waitUntilCleanup),
		)

		var value *closer

		entered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		entered.Exit()

		value = entered.Value()

		select {
		case <-value.closeCh:
		default:
			t.Fatal("value not closed")
		}
	})
}

func Test_Reuse_base_cleanup_after_exit_closer_err(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const waitUntilCleanup = time.Second

		rse := RunForTesting(t,
			func() (*closer, error) {
				return &closer{
					closeCh: make(chan struct{}),
					err:     io.ErrUnexpectedEOF,
				}, nil
			},
			WithTracer(&testingTrace[*closer]{t: t}),
			WithWaitUntilCleanup[*closer](waitUntilCleanup),
		)

		var value *closer

		entered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		entered.Exit()

		value = entered.Value()

		select {
		case <-value.closeCh:
		default:
			t.Fatal("value not closed")
		}
	})
}

type terminator struct {
	closeCh chan struct{}
	err     error
}

func (t *terminator) Terminate() error {
	close(t.closeCh)

	return t.err
}

func Test_Reuse_base_cleanup_after_exit_terminate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const waitUntilCleanup = time.Second

		rse := RunForTesting(t,
			func() (*terminator, error) {
				return &terminator{
					closeCh: make(chan struct{}),
					err:     error(nil),
				}, nil
			},
			WithTracer(&testingTrace[*terminator]{t: t}),
			WithWaitUntilCleanup[*terminator](waitUntilCleanup),
		)

		var value *terminator

		entered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		entered.Exit()

		value = entered.Value()

		select {
		case <-value.closeCh:
		default:
			t.Fatal("value not closed")
		}
	})
}

func Test_Reuse_base_cleanup_after_exit_terminate_err(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const waitUntilCleanup = time.Second

		rse := RunForTesting(t,
			func() (*terminator, error) {
				return &terminator{
					closeCh: make(chan struct{}),
					err:     io.ErrUnexpectedEOF,
				}, nil
			},
			WithTracer(&testingTrace[*terminator]{t: t}),
			WithWaitUntilCleanup[*terminator](waitUntilCleanup),
		)

		var value *terminator

		entered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		entered.Exit()

		value = entered.Value()

		select {
		case <-value.closeCh:
		default:
			t.Fatal("value not closed")
		}
	})
}

type terminatorContext struct {
	closeCh chan struct{}
	err     error
}

func (t *terminatorContext) Terminate(context.Context) error {
	close(t.closeCh)

	return t.err
}

func Test_Reuse_base_cleanup_after_exit_terminate_context(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const waitUntilCleanup = time.Second

		rse := RunForTesting(t,
			func() (*terminatorContext, error) {
				return &terminatorContext{
					closeCh: make(chan struct{}),
					err:     error(nil),
				}, nil
			},
			WithTracer(&testingTrace[*terminatorContext]{t: t}),
			WithWaitUntilCleanup[*terminatorContext](waitUntilCleanup),
		)

		var value *terminatorContext

		entered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		entered.Exit()

		value = entered.Value()

		select {
		case <-value.closeCh:
		default:
			t.Fatal("value not closed")
		}
	})
}

func Test_Reuse_base_cleanup_after_exit_terminate_context_err(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const waitUntilCleanup = time.Second

		rse := RunForTesting(t,
			func() (*terminatorContext, error) {
				return &terminatorContext{
					closeCh: make(chan struct{}),
					err:     io.ErrUnexpectedEOF,
				}, nil
			},
			WithTracer(&testingTrace[*terminatorContext]{t: t}),
			WithWaitUntilCleanup[*terminatorContext](waitUntilCleanup),
		)

		var value *terminatorContext

		entered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		entered.Exit()

		value = entered.Value()

		select {
		case <-value.closeCh:
		default:
			t.Fatal("value not closed")
		}
	})
}

func Test_Reuse_base_cleanup_after_exit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const waitUntilCleanup = time.Second

		rse := RunForTesting(t,
			func() (int, error) {
				return 1, nil
			},
			WithTracer(&testingTrace[int]{t: t}),
			WithWaitUntilCleanup[int](waitUntilCleanup),
		)

		entered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		entered.Exit()
	})
}

func Test_Reuse_base_shutdown_in_phase_wait_until_cleanup(t *testing.T) {
	synctest.Test(t, testReuseShutdownInPhaseWaitUntilCleanup)
}

func testReuseShutdownInPhaseWaitUntilCleanup(t *testing.T) {
	const waitUntilCleanup = time.Hour

	rse := RunForTesting(t,
		func() (*closer, error) {
			return &closer{
				closeCh: make(chan struct{}),
				err:     error(nil),
			}, nil
		},
		WithTracer(&testingTrace[*closer]{t: t}),
		WithWaitUntilCleanup[*closer](waitUntilCleanup),
	)

	entered, err := rse.Enter()
	if err != nil {
		t.Fatal(err)
	}

	value := entered.Value()

	wg := sync.WaitGroup{}

	wg.Go(entered.Exit)
	wg.Go(rse.Shutdown)

	wg.Wait()

	select {
	case <-value.closeCh:
	default:
		t.Fatal("value not closed")
	}
}

func Test_Reuse_base_kill_in_phase_wait_until_cleanup(t *testing.T) {
	synctest.Test(t, testReuseKillInPhaseWaitUntilCleanup)
}

func testReuseKillInPhaseWaitUntilCleanup(t *testing.T) {
	const waitUntilCleanup = time.Hour

	rse := RunForTesting(t,
		func() (*closer, error) {
			return &closer{
				closeCh: make(chan struct{}),
				err:     error(nil),
			}, nil
		},
		WithTracer(&testingTrace[*closer]{t: t}),
		WithWaitUntilCleanup[*closer](waitUntilCleanup),
	)

	entered, err := rse.Enter()
	if err != nil {
		t.Fatal(err)
	}

	value := entered.Value()

	wg := sync.WaitGroup{}

	wg.Go(entered.Exit)
	wg.Go(rse.Kill)

	wg.Wait()

	select {
	case <-value.closeCh:
	default:
		t.Fatal("value not closed")
	}
}

func Test_Reuse_base_cleanup_after_exit_parallel(t *testing.T) {
	synctest.Test(t, testReuseCleanupAfterExitParallel)
}

func testReuseCleanupAfterExitParallel(t *testing.T) {
	const waitUntilCleanup = time.Hour

	rse := RunForTesting(t,
		func() (*closer, error) {
			return &closer{
				closeCh: make(chan struct{}),
				err:     error(nil),
			}, nil
		},
		WithTracer(&testingTrace[*closer]{t: t}),
		WithWaitUntilCleanup[*closer](waitUntilCleanup),
	)

	var value *closer

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}

	checkValue := func(enteredValue *closer) {
		mu.Lock()
		defer mu.Unlock()

		if value == nil {
			value = enteredValue

			return
		}

		if value != enteredValue {
			t.Fatal("created new value")
		}
	}

	for range 100 {
		wg.Go(func() {
			entered, err := rse.Enter()
			if err != nil {
				t.Fatal(err)
			}

			checkValue(entered.Value())

			entered.Exit()
		})
	}

	now := time.Now()

	wg.Wait()

	if time.Since(now) != waitUntilCleanup {
		t.Fatal("wrong wait until cleanup")
	}

	select {
	case <-value.closeCh:
	default:
		t.Fatal("value not closed")
	}
}

func Test_Reuse_base_cant_enter_after_shutdown(t *testing.T) {
	rse := Run(func() (int, error) { return 0, nil })

	rse.Shutdown()

	_, err := rse.Enter()
	if !errors.Is(err, ErrReuseIsClosed) {
		t.Fatalf("unexpected error: %s", err)
	}
}

func Test_Reuse_base_cant_enter_after_kill(t *testing.T) {
	rse := Run(func() (int, error) { return 0, nil })

	rse.Kill()

	_, err := rse.Enter()
	if !errors.Is(err, ErrReuseIsClosed) {
		t.Fatalf("unexpected error: %s", err)
	}
}

func Test_Reuse_base_done_is_closed_after_shutdown(t *testing.T) {
	rse := Run(func() (int, error) { return 0, nil })

	t.Cleanup(func() {
		select {
		case <-rse.done:
		default:
			t.Fatal("reuse.done channel not closed")
		}
	})

	t.Cleanup(rse.Kill)
	t.Cleanup(rse.Kill)
	t.Cleanup(rse.Shutdown)
	t.Cleanup(rse.Shutdown)
}

func Test_Reuse_base_done_is_closed_after_kill(t *testing.T) {
	rse := Run(func() (int, error) { return 0, nil })

	t.Cleanup(func() {
		select {
		case <-rse.done:
		default:
			t.Fatal("reuse.done channel not closed")
		}
	})

	t.Cleanup(rse.Shutdown)
	t.Cleanup(rse.Shutdown)
	t.Cleanup(rse.Kill)
	t.Cleanup(rse.Kill)
}

func Test_Reuse_base_double_exit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rse := RunForTesting(t, func() (*closer, error) {
			return &closer{
				closeCh: make(chan struct{}),
				err:     error(nil),
			}, nil
		})

		entered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		value := entered.Value()

		entered.Exit()
		entered.Exit()

		select {
		case <-value.closeCh:
		default:
			t.Fatal("value not closed")
		}
	})
}

func Test_Reuse_base_create_func_return_error(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rse := RunForTesting(t, func() (int, error) {
			return 0, io.ErrUnexpectedEOF
		})

		_, err := rse.Enter()
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("unexpected error returned from rse.Enter: %s", err)
		}
	})
}

func Test_command(*testing.T) {
	cmds := []command[any]{
		commandCleanup[any]{},
		commandEnter[any]{},
		commandExit[any]{},
		commandKill[any]{},
		commandShutdown[any]{},
	}

	for _, cmd := range cmds {
		cmd.command()
	}
}

func Test_Reuse_phase_shutdown_enter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rse := Run(func() (*closer, error) {
			return &closer{
				closeCh: make(chan struct{}),
				err:     error(nil),
			}, nil
		},
		)

		initialEntered, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		go rse.Shutdown()

	Loop:
		for {
			entered, err := rse.Enter()
			switch {
			case err == nil:
				entered.Exit()
			case errors.Is(err, ErrShutdown):
				break Loop
			}
		}

		initialEntered.Exit()
	})
}

func Test_Reuse_phase_shutdown_kill(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rse := Run(func() (*closer, error) {
			return &closer{
				closeCh: make(chan struct{}),
				err:     error(nil),
			}, nil
		},
		)

		_, err := rse.Enter()
		if err != nil {
			t.Fatal(err)
		}

		go rse.Shutdown()

	Loop:
		for {
			entered, err := rse.Enter()
			switch {
			case err == nil:
				entered.Exit()
			case errors.Is(err, ErrShutdown):
				break Loop
			}
		}

		rse.Kill()
	})
}

func Test_Reuse_Run_nil_create_func(t *testing.T) {
	defer func() {
		msg := recover().(string)
		if msg == "" {
			t.Fatal("expected panic message but got empty")
		}
	}()

	_ = Run[int](nil)
}

type inlineTerminater func(ctx context.Context) error

func (in inlineTerminater) Terminate(ctx context.Context) error {
	return in(ctx)
}

func Test_Reuse_Option_cleanup_timeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cleanupTimeout := time.Second * 64
		startTime := time.Now()

		rse := Run(func() (inlineTerminater, error) {
			return func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("ctx should have deadline")
				}

				timeout := deadline.Sub(startTime)

				if timeout != cleanupTimeout {
					t.Fatalf("unexpected context deadline: %s", timeout)
				}

				return nil
			}, nil
		},
			WithCleanupTimeout[inlineTerminater](cleanupTimeout),
		)

		rse.Shutdown()
	})
}
