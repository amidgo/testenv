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
	"github.com/amidgo/testenv/internal/reuse"

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

type ReusableOption func(*Reusable)

const defaultWaitUntilCleanup = time.Second

func WithWaitDuration(waitDuration time.Duration) ReusableOption {
	return func(r *Reusable) {
		r.waitUntilCleanup = waitDuration
	}
}

const defaultCleanupTimeout = time.Second

func WithCleanupTimeout(timeout time.Duration) ReusableOption {
	return func(r *Reusable) {
		r.cleanupTimeout = timeout
	}
}

type Reusable struct {
	ccf ProvideEnvironmentFunc

	runReuseOnce sync.Once
	env          *reuse.Reuse[Environment]

	waitUntilCleanup time.Duration
	cleanupTimeout   time.Duration
}

func NewReusable(ccf ProvideEnvironmentFunc, opts ...ReusableOption) *Reusable {
	reusable := &Reusable{
		ccf:              ccf,
		waitUntilCleanup: defaultWaitUntilCleanup,
		cleanupTimeout:   defaultCleanupTimeout,
	}

	for _, op := range opts {
		op(reusable)
	}

	return reusable
}

func (r *Reusable) Shutdown() {
	r.env.Shutdown()
}

func (r *Reusable) run(
	ctx context.Context,
	buckets ...Bucket,
) (client *minio.Client, term func(), err error) {
	r.runReuseOnce.Do(r.runReuse)

	env, err := r.enter()
	if err != nil {
		return nil, func() {}, fmt.Errorf("enter to reuse environment, %w", err)
	}

	client, term, err = r.reuse(ctx, env, buckets...)
	if err != nil {
		return nil, term, fmt.Errorf("reuse environment, %w", err)
	}

	return client, term, nil
}

func (r *Reusable) runReuse() {
	r.env = reuse.Run((func() (Environment, error))(r.ccf),
		reuse.WithWaitUntilCleanup[Environment](r.waitUntilCleanup),
		reuse.WithCleanupTimeout[Environment](r.cleanupTimeout),
	)
}

func (r *Reusable) enter() (reuse.Entered[Environment], error) {
	entered, err := r.env.Enter()
	if err != nil {
		return reuse.Entered[Environment]{}, fmt.Errorf("r.env.Enter: %w", err)
	}

	return entered, nil
}

func (r *Reusable) reuse(
	ctx context.Context,
	entered reuse.Entered[Environment],
	buckets ...Bucket,
) (minioClient *minio.Client, term func(), err error) {
	term = entered.Exit

	env := entered.Value()

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
