// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package reuse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"sync"
	"time"
)

const (
	defaultWaitUntilCleanup = time.Second
	defaultCleanupTimeout   = 5 * time.Second
)

type Tracer[T any] interface {
	TraceChangePhase(oldPhase, newPhase phase[T])
	TraceCommandReceived(receiverPhase phase[T], cmd command[T])
}

type noopTracer[T any] struct{}

func (noopTracer[T]) TraceChangePhase(_, _ phase[T])            {}
func (noopTracer[T]) TraceCommandReceived(phase[T], command[T]) {}

type CreateFunc[T any] func() (T, error)

type Config[T any] struct {
	WaitUntilCleanup time.Duration
	CleanupTimeout   time.Duration
	Tracer           Tracer[T]

	createFunc CreateFunc[T]
}

func Cleanup(ctx context.Context, value any) (bool, error) {
	switch typedValue := value.(type) {
	case io.Closer:
		err := typedValue.Close()
		if err != nil {
			return true, fmt.Errorf("value.Close: %w", err)
		}

		return true, nil

	case interface {
		Terminate(ctx context.Context) error
	}:
		err := typedValue.Terminate(ctx)
		if err != nil {
			return true, fmt.Errorf("value.Terminate(ctx): %w", err)
		}

		return true, nil

	case interface{ Terminate() error }:
		err := typedValue.Terminate()
		if err != nil {
			return true, fmt.Errorf("value.Terminate: %w", err)
		}

		return true, nil

	default:
		return false, nil
	}
}

//sumtype:decl
type command[T any] interface {
	command()
	String() string
}

type commandEnter[T any] struct {
	replyCh chan commandEnterResponse[T]
}

func (commandEnter[T]) String() string {
	return "commandEnter"
}

func (c commandEnter[T]) command() { var _ command[T] = c }

type commandEnterResponse[T any] struct {
	value T
	err   error
}

type commandExit[T any] struct {
	replyCh chan commandExitResponse
}

func (c commandExit[T]) command() { var _ command[T] = c }

func (commandExit[T]) String() string {
	return "commandExit"
}

type commandExitResponse struct {
	waitUntilCleanupCh chan struct{}
}

type commandCleanup[T any] struct {
	replyCh chan commandCleanupResponse
}

func (c commandCleanup[T]) command() { var _ command[T] = c }

func (commandCleanup[T]) String() string {
	return "commandCleeanup"
}

type commandCleanupResponse struct{}

type commandShutdown[T any] struct {
	replyCh   chan commandShutdownResponse
	terminate func()
}

func (c commandShutdown[T]) command() { var _ command[T] = c }

func (commandShutdown[T]) String() string {
	return "commandShutdown"
}

type commandShutdownResponse struct{}

type commandKill[T any] struct {
	replyCh   chan commandKillResponse
	terminate func()
}

func (commandKill[T]) String() string {
	return "commandKill"
}

func (c commandKill[T]) command() { var _ command[T] = c }

type commandKillResponse struct{}

type phase[T any] interface {
	handleCommand(cfg Config[T], value T, cmd command[T]) (T, phase[T])
	String() string
}

func panicInvalidCommand[T any](cmd command[T]) (T, phase[T]) {
	msg := fmt.Sprintf("invalid command type received: %T", cmd)

	panic(msg)
}

type phaseInitial[T any] struct{}

func (phaseInitial[T]) String() string {
	return "phaseInitial"
}

func (phaseInitial[T]) handleEnterCommand(
	cfg Config[T],
	_ T,
	cmd commandEnter[T],
) (T, phase[T]) {
	value, err := cfg.createFunc()
	if err != nil {
		var zero T

		cmd.replyCh <- commandEnterResponse[T]{
			value: zero,
			err:   err,
		}

		return zero, phaseInitial[T]{}
	}

	cmd.replyCh <- commandEnterResponse[T]{
		value: value,
		err:   error(nil),
	}

	return value, phaseActive[T]{users: 1}
}

func (phaseInitial[T]) handleCommandExit(
	Config[T],
	T,
	commandExit[T],
) (T, phase[T]) {
	panic("unexpected exit command in initial phase")
}

func (phaseInitial[T]) handleCommandCleanup(
	Config[T],
	T,
	commandCleanup[T],
) (T, phase[T]) {
	panic("unexpected cleanup command in initial phase")
}

func (phaseInitial[T]) handleCommandShutdown(
	_ Config[T],
	_ T,
	cmd commandShutdown[T],
) (T, phase[T]) {
	cmd.terminate()

	cmd.replyCh <- commandShutdownResponse{}

	var zero T

	return zero, phase[T](nil)
}

func (phaseInitial[T]) handleCommandKill(
	_ Config[T],
	_ T,
	cmd commandKill[T],
) (T, phase[T]) {
	cmd.terminate()

	cmd.replyCh <- commandKillResponse{}

	var zero T

	return zero, phase[T](nil)
}

func (p phaseInitial[T]) handleCommand(
	cfg Config[T],
	value T,
	cmd command[T],
) (T, phase[T]) {
	switch typedCmd := cmd.(type) {
	case commandEnter[T]:
		return p.handleEnterCommand(cfg, value, typedCmd)

	case commandExit[T]:
		return p.handleCommandExit(cfg, value, typedCmd)

	case commandCleanup[T]:
		return p.handleCommandCleanup(cfg, value, typedCmd)

	case commandShutdown[T]:
		return p.handleCommandShutdown(cfg, value, typedCmd)

	case commandKill[T]:
		return p.handleCommandKill(cfg, value, typedCmd)

	default:
		return panicInvalidCommand[T](typedCmd)
	}
}

type phaseActive[T any] struct {
	users int
}

func (phaseActive[T]) String() string {
	return "phaseActive"
}

func (p phaseActive[T]) handleCommandEnter(
	_ Config[T],
	value T,
	cmd commandEnter[T],
) (T, phase[T]) {
	cmd.replyCh <- commandEnterResponse[T]{
		value: value,
		err:   error(nil),
	}

	return value, phaseActive[T]{users: p.users + 1}
}

func (p phaseActive[T]) handleCommandExit(
	cfg Config[T],
	value T,
	cmd commandExit[T],
) (T, phase[T]) {
	if p.users <= 0 {
		panic("unexpected zero or negative value of users: " + strconv.Itoa(p.users))
	}

	if p.users > 1 {
		cmd.replyCh <- commandExitResponse{waitUntilCleanupCh: chan struct{}(nil)}

		return value, phaseActive[T]{users: p.users - 1}
	}

	waitUntilCleanupCh := make(chan struct{}, 1)

	once := sync.Once{}

	doCleanup := func() {
		once.Do(func() {
			close(waitUntilCleanupCh)
		})
	}

	stopDoCleanupTimer := time.AfterFunc(cfg.WaitUntilCleanup, doCleanup)

	cancelCleanupByTimeout := func() {
		once.Do(func() {
			stopDoCleanupTimer.Stop()

			waitUntilCleanupCh <- struct{}{}

			close(waitUntilCleanupCh)
		})
	}

	cmd.replyCh <- commandExitResponse{waitUntilCleanupCh: waitUntilCleanupCh}

	return value, phaseWaitUntilCleanup[T]{cancelCleanupByTimeout: cancelCleanupByTimeout}
}

func (p phaseActive[T]) handleCommandCleanup(
	_ Config[T],
	value T,
	cmd commandCleanup[T],
) (T, phase[T]) {
	cmd.replyCh <- commandCleanupResponse{}

	return value, phaseActive[T]{users: p.users}
}

func (p phaseActive[T]) handleCommandShutdown(
	_ Config[T],
	value T,
	cmd commandShutdown[T],
) (T, phase[T]) {
	return value, phaseShutdown[T]{
		users:     p.users,
		terminate: cmd.terminate,
		replyCh:   cmd.replyCh,
	}
}

func (phaseActive[T]) handleCommandKill(
	cfg Config[T],
	value T,
	cmd commandKill[T],
) (T, phase[T]) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.CleanupTimeout)
	defer cancel()

	_, _ = Cleanup(ctx, value)

	cmd.terminate()

	cmd.replyCh <- commandKillResponse{}

	var zero T

	return zero, phase[T](nil)
}

func (p phaseActive[T]) handleCommand(
	cfg Config[T],
	value T,
	cmd command[T],
) (T, phase[T]) {
	switch typedCmd := cmd.(type) {
	case commandEnter[T]:
		return p.handleCommandEnter(cfg, value, typedCmd)

	case commandExit[T]:
		return p.handleCommandExit(cfg, value, typedCmd)

	case commandCleanup[T]:
		return p.handleCommandCleanup(cfg, value, typedCmd)

	case commandShutdown[T]:
		return p.handleCommandShutdown(cfg, value, typedCmd)

	case commandKill[T]:
		return p.handleCommandKill(cfg, value, typedCmd)

	default:
		return panicInvalidCommand[T](typedCmd)
	}
}

type phaseWaitUntilCleanup[T any] struct {
	cancelCleanupByTimeout func()
}

func (phaseWaitUntilCleanup[T]) String() string {
	return "phaseWaitUntilCleanup"
}

func (p phaseWaitUntilCleanup[T]) handleCommandEnter(
	_ Config[T],
	value T,
	cmd commandEnter[T],
) (T, phase[T]) {
	p.cancelCleanupByTimeout()

	cmd.replyCh <- commandEnterResponse[T]{
		value: value,
		err:   error(nil),
	}

	return value, phaseActive[T]{users: 1}
}

func (phaseWaitUntilCleanup[T]) handleCommandExit(
	Config[T],
	T,
	commandExit[T],
) (T, phase[T]) {
	panic("unexpected exit in wait until cleanup phase")
}

func (phaseWaitUntilCleanup[T]) handleCommandCleanup(
	cfg Config[T],
	value T,
	cmd commandCleanup[T],
) (T, phase[T]) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.CleanupTimeout)
	defer cancel()

	_, _ = Cleanup(ctx, value)

	cmd.replyCh <- commandCleanupResponse{}

	var zero T

	return zero, phaseInitial[T]{}
}

func (p phaseWaitUntilCleanup[T]) handleCommandShutdown(
	cfg Config[T],
	value T,
	cmd commandShutdown[T],
) (T, phase[T]) {
	p.cancelCleanupByTimeout()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.CleanupTimeout)
	defer cancel()

	_, _ = Cleanup(ctx, value)

	cmd.terminate()

	cmd.replyCh <- commandShutdownResponse{}

	var zero T

	return zero, phase[T](nil)
}

func (p phaseWaitUntilCleanup[T]) handleCommandKill(
	cfg Config[T],
	value T,
	cmd commandKill[T],
) (T, phase[T]) {
	p.cancelCleanupByTimeout()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.CleanupTimeout)
	defer cancel()

	_, _ = Cleanup(ctx, value)

	cmd.terminate()

	cmd.replyCh <- commandKillResponse{}

	var zero T

	return zero, phase[T](nil)
}

func (p phaseWaitUntilCleanup[T]) handleCommand(
	cfg Config[T],
	value T,
	cmd command[T],
) (T, phase[T]) {
	switch typedCmd := cmd.(type) {
	case commandEnter[T]:
		return p.handleCommandEnter(cfg, value, typedCmd)

	case commandExit[T]:
		return p.handleCommandExit(cfg, value, typedCmd)

	case commandCleanup[T]:
		return p.handleCommandCleanup(cfg, value, typedCmd)

	case commandShutdown[T]:
		return p.handleCommandShutdown(cfg, value, typedCmd)

	case commandKill[T]:
		return p.handleCommandKill(cfg, value, typedCmd)

	default:
		return panicInvalidCommand[T](typedCmd)
	}
}

type phaseShutdown[T any] struct {
	users int

	terminate func()
	replyCh   chan commandShutdownResponse
}

func (phaseShutdown[T]) String() string {
	return "phaseShutdown"
}

var ErrShutdown = errors.New("reuse in shutdown phase, any enter is blocked")

func (p phaseShutdown[T]) handleCommandEnter(
	_ Config[T],
	value T,
	cmd commandEnter[T],
) (T, phase[T]) {
	var zero T

	cmd.replyCh <- commandEnterResponse[T]{
		value: zero,
		err:   ErrShutdown,
	}

	return value, phaseShutdown[T]{
		users:     p.users,
		terminate: p.terminate,
		replyCh:   p.replyCh,
	}
}

func (p phaseShutdown[T]) handleCommandExit(
	cfg Config[T],
	value T,
	cmd commandExit[T],
) (T, phase[T]) {
	if p.users > 1 {
		cmd.replyCh <- commandExitResponse{}

		return value, phaseShutdown[T]{
			users:     p.users - 1,
			terminate: p.terminate,
			replyCh:   p.replyCh,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.CleanupTimeout)
	defer cancel()

	_, _ = Cleanup(ctx, value)

	p.terminate()

	cmd.replyCh <- commandExitResponse{}

	p.replyCh <- commandShutdownResponse{}

	var zero T

	return zero, phase[T](nil)
}

func (_ phaseShutdown[T]) handleCommandCleanup(
	Config[T],
	T,
	commandCleanup[T],
) (T, phase[T]) {
	panic("unexpected command cleanup in shutdown phase")
}

func (_ phaseShutdown[T]) handleCommandShutdown(
	Config[T],
	T,
	commandShutdown[T],
) (T, phase[T]) {
	panic("unexpected command shutdown in shutdown phase")
}

func (p phaseShutdown[T]) handleCommandKill(
	cfg Config[T],
	value T,
	cmd commandKill[T],
) (T, phase[T]) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.CleanupTimeout)
	defer cancel()

	_, _ = Cleanup(ctx, value)

	p.terminate()

	p.replyCh <- commandShutdownResponse{}

	cmd.terminate()

	cmd.replyCh <- commandKillResponse{}

	var zero T

	return zero, phase[T](nil)
}

func (p phaseShutdown[T]) handleCommand(
	cfg Config[T],
	value T,
	cmd command[T],
) (T, phase[T]) {
	switch typedCmd := cmd.(type) {
	case commandEnter[T]:
		return p.handleCommandEnter(cfg, value, typedCmd)

	case commandExit[T]:
		return p.handleCommandExit(cfg, value, typedCmd)

	case commandCleanup[T]:
		return p.handleCommandCleanup(cfg, value, typedCmd)

	case commandShutdown[T]:
		return p.handleCommandShutdown(cfg, value, typedCmd)

	case commandKill[T]:
		return p.handleCommandKill(cfg, value, typedCmd)

	default:
		return panicInvalidCommand[T](typedCmd)
	}
}

func runDaemon[T any](cfg Config[T], commandCh chan command[T]) {
	var (
		value T
		phase phase[T] = phaseInitial[T]{}
	)

	for cmd := range commandCh {
		cfg.Tracer.TraceCommandReceived(phase, cmd)

		newValue, newPhase := phase.handleCommand(cfg, value, cmd)
		if newPhase == nil {
			return
		}

		if reflect.TypeOf(phase) != reflect.TypeOf(newPhase) {
			cfg.Tracer.TraceChangePhase(phase, newPhase)
		}

		value = newValue
		phase = newPhase
	}
}

type Entered[T any] struct {
	exit  func()
	value T
}

func (e Entered[T]) Exit() {
	e.exit()
}

func (e Entered[T]) Value() T {
	return e.value
}

type Reuse[T any] struct {
	cfg       Config[T]
	commandCh chan command[T]

	done chan struct{}

	shutdownOnce sync.Once
	closeOnce    sync.Once
}

func (r *Reuse[T]) cleanup() {
	replyCh := make(chan commandCleanupResponse)

	select {
	case <-r.done:
	case r.commandCh <- commandCleanup[T]{replyCh: replyCh}:

		<-replyCh
	}
}

func (r *Reuse[T]) exit() {
	replyCh := make(chan commandExitResponse)

	select {
	case <-r.done:
		return

	case r.commandCh <- commandExit[T]{replyCh: replyCh}:
		resp := <-replyCh

		if resp.waitUntilCleanupCh == nil {
			return
		}

		select {
		case <-r.done:
		case _, ok := <-resp.waitUntilCleanupCh:
			if !ok {
				r.cleanup()
			}
		}
	}
}

var ErrReuseIsClosed = errors.New("reuse is closed")

func (r *Reuse[T]) Enter() (Entered[T], error) {
	replyCh := make(chan commandEnterResponse[T])

	select {
	case <-r.done:
		return Entered[T]{}, ErrReuseIsClosed

	case r.commandCh <- commandEnter[T]{replyCh: replyCh}:
		resp := <-replyCh

		if resp.err != nil {
			return Entered[T]{}, resp.err
		}

		return Entered[T]{
			exit:  sync.OnceFunc(r.exit),
			value: resp.value,
		}, nil
	}
}

func (r *Reuse[T]) terminate() {
	r.closeOnce.Do(func() { close(r.done) })
}

func (r *Reuse[T]) Kill() {
	replyCh := make(chan commandKillResponse)

	select {
	case <-r.done:
	case r.commandCh <- commandKill[T]{
		replyCh:   replyCh,
		terminate: r.terminate,
	}:
		<-replyCh
	}
}

func (r *Reuse[T]) Shutdown() {
	r.shutdownOnce.Do(func() {
		replyCh := make(chan commandShutdownResponse)

		select {
		case <-r.done:
		case r.commandCh <- commandShutdown[T]{
			replyCh:   replyCh,
			terminate: r.terminate,
		}:
			<-replyCh
		}
	})
}

// Run T must be safe to use by many goroutines
func Run[T any](
	createFunc CreateFunc[T],
	opts ...Option[T],
) *Reuse[T] {
	if createFunc == nil {
		panic("pass nil cfg.CreateFunc in reuse.Run")
	}

	cfg := Config[T]{
		WaitUntilCleanup: defaultWaitUntilCleanup,
		CleanupTimeout:   defaultCleanupTimeout,
		Tracer:           noopTracer[T]{},
		createFunc:       createFunc,
	}

	for _, op := range opts {
		op(&cfg)
	}

	commandCh := make(chan command[T])

	go runDaemon(cfg, commandCh)

	return &Reuse[T]{
		cfg:       cfg,
		commandCh: commandCh,

		done: make(chan struct{}),

		shutdownOnce: sync.Once{},
		closeOnce:    sync.Once{},
	}
}

type Cleanupper interface {
	Cleanup(doCleanup func())
}

type Option[T any] func(cfg *Config[T])

func WithCleanupTimeout[T any](cleanupTimeout time.Duration) Option[T] {
	return func(cfg *Config[T]) {
		cfg.CleanupTimeout = cleanupTimeout
	}
}

func WithWaitUntilCleanup[T any](wait time.Duration) Option[T] {
	return func(cfg *Config[T]) {
		cfg.WaitUntilCleanup = wait
	}
}

func WithTracer[T any](tracer Tracer[T]) Option[T] {
	return func(cfg *Config[T]) {
		cfg.Tracer = tracer
	}
}

func RunForTesting[T any](
	cleanupper Cleanupper,
	createFunc CreateFunc[T],
	opts ...Option[T],
) *Reuse[T] {
	reuse := Run(createFunc, opts...)

	cleanupper.Cleanup(reuse.Shutdown)

	return reuse
}
