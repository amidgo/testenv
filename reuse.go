// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package testenv

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
)

type reuseCommand uint8

const (
	reuseCommandEnter reuseCommand = iota
	reuseCommandExit
)

type reuseEnvironmentRequest struct {
	reuseCmd reuseCommand
	ctx      context.Context
}

type reuseEnvironmentResponse struct {
	env any
	err error
}

type ProvideEnvironmentFunc func(ctx context.Context) (any, error)

type ReusableDaemon struct {
	activeUsers  int
	env          any
	waitDuration time.Duration
	mainCtx      context.Context
	termCtx      context.Context

	reqCh  chan reuseEnvironmentRequest
	respCh chan reuseEnvironmentResponse

	pef ProvideEnvironmentFunc
}

func RunReusableDaemon(
	ctx context.Context,
	waitDuration time.Duration,
	pef ProvideEnvironmentFunc,
) *ReusableDaemon {
	termCtx, cancel := context.WithCancel(context.Background())

	daemon := &ReusableDaemon{
		activeUsers:  0,
		env:          nil,
		waitDuration: waitDuration,
		mainCtx:      ctx,
		termCtx:      termCtx,
		reqCh:        make(chan reuseEnvironmentRequest),
		respCh:       make(chan reuseEnvironmentResponse),
		pef:          pef,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				daemon.clearEnvironment(termCtx)
				cancel()

				return
			case req := <-daemon.reqCh:
				daemon.handleReuseCommand(req.ctx, req.reuseCmd)
			}
		}
	}()

	return daemon
}

func (d *ReusableDaemon) Done() <-chan struct{} {
	return d.termCtx.Done()
}

func (d *ReusableDaemon) Enter(ctx context.Context) (any, error) {
	select {
	case <-d.mainCtx.Done():
		return nil, fmt.Errorf("context.Cause(*ReusableDaemon.mainCtx): %w", context.Cause(d.mainCtx))
	case d.reqCh <- reuseEnvironmentRequest{
		ctx:      ctx,
		reuseCmd: reuseCommandEnter,
	}:
		resp := <-d.respCh

		return resp.env, resp.err
	}
}

func (d *ReusableDaemon) Exit() {
	select {
	case <-d.mainCtx.Done():
		<-d.termCtx.Done()
	case d.reqCh <- reuseEnvironmentRequest{
		ctx:      context.Background(),
		reuseCmd: reuseCommandExit,
	}:
		<-d.respCh
	}
}

func (d *ReusableDaemon) handleReuseCommand(ctx context.Context, reuseCmd reuseCommand) {
	switch reuseCmd {
	case reuseCommandEnter:
		d.activeUsers++
	case reuseCommandExit:
		d.activeUsers--
	default:
		panic("invalid reuse command received: " + strconv.FormatUint(uint64(reuseCmd), 10))
	}

	switch {
	case d.activeUsers > 0:
		d.handlePositiveActiveUsers(ctx)
	case d.activeUsers == 0:
		d.handleZeroActiveUsers(ctx)
	case d.activeUsers <= 0:
		panic("reuse environment term func called twice, negative amount of active users")
	}
}

func (d *ReusableDaemon) handlePositiveActiveUsers(ctx context.Context) {
	if d.env == nil {
		env, err := d.pef(ctx)
		if err != nil {
			d.respCh <- reuseEnvironmentResponse{
				err: fmt.Errorf("create new environment, %w", err),
			}

			return
		}

		if env == nil {
			panic("nil environment returned")
		}

		d.env = env
	}

	d.respCh <- reuseEnvironmentResponse{
		env: d.env,
	}
}

func (d *ReusableDaemon) handleZeroActiveUsers(ctx context.Context) {
	select {
	case <-time.After(d.waitDuration):
		d.clearEnvironment(ctx)
		d.respCh <- reuseEnvironmentResponse{}
	case req := <-d.reqCh:
		switch req.reuseCmd {
		case reuseCommandEnter:
			d.activeUsers++
			d.respCh <- reuseEnvironmentResponse{
				env: d.env,
			}
			d.respCh <- reuseEnvironmentResponse{
				env: d.env,
			}
		case reuseCommandExit:
			panic("unexpected exit command in handleZeroActiveUsers")
		default:
			panic("invalid reuse command received: " + strconv.FormatUint(uint64(req.reuseCmd), 10))
		}
	}
}

func (d *ReusableDaemon) clearEnvironment(ctx context.Context) {
	if d.env == nil {
		return
	}

	type Terminater interface {
		Terminate(ctx context.Context) error
	}

	trm, ok := d.env.(Terminater)
	if ok {
		err := trm.Terminate(ctx)
		if err != nil {
			log.Printf("failed terminate environment, %s", err)
		}
	}

	d.env = nil
}
