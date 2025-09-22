// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package minioenv

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/amidgo/testenv"

	"github.com/minio/minio-go/v7"
)

func ReuseForTesting(
	t *testing.T,
	reusable *Reusable,
	buckets ...Bucket,
) *minio.Client {
	testenv.SkipDisabled(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	minioClient, term, err := Reuse(ctx, reusable, buckets...)
	t.Cleanup(term)

	if err != nil {
		t.Fatal(err)

		return nil
	}

	return minioClient
}

func Reuse(
	ctx context.Context,
	reusable *Reusable,
	buckets ...Bucket,
) (minioClinet *minio.Client, term func(), err error) {
	return reusable.run(ctx, buckets...)
}

const defaultDuration = time.Second

type ReusableOption func(*Reusable)

func WithWaitDuration(waitDuration time.Duration) ReusableOption {
	return func(r *Reusable) {
		r.daemonWaitDuration = waitDuration
	}
}

type Reusable struct {
	ccf ProvideEnvironmentFunc

	runDaemonOnce      sync.Once
	daemon             *testenv.ReusableDaemon
	stopDaemon         context.CancelFunc
	daemonWaitDuration time.Duration
}

func NewReusable(ccf ProvideEnvironmentFunc, opts ...ReusableOption) *Reusable {
	reusable := &Reusable{
		ccf:                ccf,
		daemonWaitDuration: defaultDuration,
	}

	for _, op := range opts {
		op(reusable)
	}

	return reusable
}

func (r *Reusable) Terminate(ctx context.Context) error {
	r.stopDaemon()

	select {
	case <-r.daemon.Done():
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (r *Reusable) run(ctx context.Context, buckets ...Bucket) (client *minio.Client, term func(), err error) {
	r.runDaemonOnce.Do(r.runDaemon)

	env, err := r.enter(ctx)
	if err != nil {
		return nil, func() {}, fmt.Errorf("enter to reuse environment, %w", err)
	}

	client, term, err = r.reuse(ctx, env, buckets...)
	if err != nil {
		return nil, term, fmt.Errorf("reuse environment, %w", err)
	}

	return client, term, nil
}

func (r *Reusable) runDaemon() {
	ccf := func(ctx context.Context) (any, error) {
		return r.ccf(ctx)
	}

	ctx, cancel := context.WithCancel(context.Background())

	r.daemon = testenv.RunReusableDaemon(ctx,
		r.daemonWaitDuration,
		ccf,
	)
	r.stopDaemon = cancel
}

func (r *Reusable) enter(ctx context.Context) (Environment, error) {
	env, err := r.daemon.Enter(ctx)
	if err != nil {
		return nil, err
	}

	return env.(Environment), nil
}

func (r *Reusable) reuse(
	ctx context.Context,
	env Environment,
	buckets ...Bucket,
) (minioClient *minio.Client, term func(), err error) {
	term = r.daemon.Exit

	minioClient, err = env.Connect(ctx)
	if err != nil {
		return nil, term, fmt.Errorf("connect to environment, %w", err)
	}

	err = insertBuckets(ctx, minioClient, buckets...)
	if err != nil {
		return nil, term, err
	}

	return minioClient, term, nil
}
