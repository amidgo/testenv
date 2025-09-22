// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package postgresenv

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/amidgo/testenv"

	"github.com/amidgo/testenv/postgres/migrations"
)

func ReuseForTesting(
	t *testing.T,
	reuse *Reusable,
	mig migrations.Migrations,
	initialQueries ...migrations.Query,
) *sql.DB {
	testenv.SkipDisabled(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	db, term, err := Reuse(ctx, reuse, mig, initialQueries...)
	t.Cleanup(term)

	if err != nil {
		t.Fatalf("Reuse: %s", err)

		return nil
	}

	return db
}

func Reuse(
	ctx context.Context,
	reuse *Reusable,
	mig migrations.Migrations,
	initialQueries ...migrations.Query,
) (db *sql.DB, term func(), err error) {
	return reuse.run(ctx, mig, initialQueries...)
}

const defaultDuration = time.Second

type ReusableOption func(r *Reusable)

func WithWaitDuration(duration time.Duration) ReusableOption {
	return func(r *Reusable) {
		r.daemonWaitDuration = duration
	}
}

type Reusable struct {
	ccf ProvideEnvironmentFunc

	runDaemonOnce      sync.Once
	dm                 *testenv.ReusableDaemon
	stopDaemon         context.CancelFunc
	daemonWaitDuration time.Duration
}

func NewReusable(ccf ProvideEnvironmentFunc, opts ...ReusableOption) *Reusable {
	r := &Reusable{
		ccf:                ccf,
		daemonWaitDuration: defaultDuration,
	}

	for _, op := range opts {
		op(r)
	}

	return r
}

func (r *Reusable) runDaemon() {
	ccf := func(ctx context.Context) (any, error) {
		return r.ccf(ctx)
	}

	ctx, cancel := context.WithCancel(context.Background())

	daemon := testenv.RunReusableDaemon(ctx, r.daemonWaitDuration, ccf)

	r.dm = daemon
	r.stopDaemon = cancel
}

func (r *Reusable) Terminate(ctx context.Context) error {
	r.stopDaemon()

	select {
	case <-r.dm.Done():
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (r *Reusable) run(
	ctx context.Context,
	mig migrations.Migrations,
	initialQueries ...migrations.Query,
) (db *sql.DB, term func(), err error) {
	r.runDaemonOnce.Do(r.runDaemon)

	env, err := r.enter(ctx)
	if err != nil {
		return nil, func() {}, fmt.Errorf("*Reusable.enter: %w", err)
	}

	db, term, err = r.reuse(ctx, env, mig, initialQueries...)
	if err != nil {
		return db, term, fmt.Errorf("*Reusable.reuse: %w", err)
	}

	return db, term, nil
}

func (r *Reusable) reuse(
	ctx context.Context,
	env Environment,
	mig migrations.Migrations,
	initialQueries ...migrations.Query,
) (db *sql.DB, term func(), err error) {
	term = r.dm.Exit

	schemaName, err := r.createNewSchema(ctx, env)
	if err != nil {
		return nil, term, err
	}

	db, err = connectToSchema(ctx, env, schemaName)
	if err != nil {
		return db, term, err
	}

	term = func() {
		_ = db.Close()
		r.dm.Exit()
	}

	if mig != nil {
		err = mig.Up(ctx, db)
		if err != nil {
			return db, term, fmt.Errorf("migrations.Up: %w", err)
		}
	}

	for _, initialQuery := range initialQueries {
		err = migrations.ExecQuery(ctx, db, initialQuery)
		if err != nil {
			return db, term, err
		}
	}

	return db, term, nil
}

func (r *Reusable) createNewSchema(ctx context.Context, env Environment) (schemaName string, err error) {
	baseDB, err := env.Connect(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("environment.Connect: %w", err)
	}

	defer baseDB.Close()

	schemaName, err = r.createSchema(ctx, baseDB)
	if err != nil {
		return "", err
	}

	return schemaName, nil
}

func (r *Reusable) createSchema(ctx context.Context, db *sql.DB) (schemaName string, err error) {
	schemaName = fmt.Sprintf("public%d", rand.Int64())

	query := fmt.Sprintf("CREATE SCHEMA %s", schemaName)

	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("db.ExecContext(CREATE SCHEMA %s), %w", schemaName, err)
	}

	return schemaName, nil
}

func connectToSchema(ctx context.Context, env Environment, schemaName string) (*sql.DB, error) {
	db, err := env.Connect(ctx, "sslmode=disable", "search_path="+schemaName)
	if err != nil {
		return nil, fmt.Errorf("enviroment.Connect(sslmode=disable, search_path=%s): %w", schemaName, err)
	}

	return db, nil
}

func (r *Reusable) enter(ctx context.Context) (Environment, error) {
	env, err := r.dm.Enter(ctx)
	if err != nil {
		return nil, err
	}

	return env.(Environment), nil
}
