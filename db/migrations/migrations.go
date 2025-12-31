// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"database/sql"
	"fmt"

	dbenv "github.com/amidgo/testenv/db"
)

type Migrations interface {
	Up(ctx context.Context, db *sql.DB) error
	Down(ctx context.Context, db *sql.DB) error
}

var Nil nilMigrations

type nilMigrations struct{}

func (nilMigrations) Up(context.Context, *sql.DB) error {
	return nil
}

func (nilMigrations) Down(context.Context, *sql.DB) error {
	return nil
}

type environment struct {
	migrations  Migrations
	environment dbenv.Environment
}

func (e *environment) Connect(ctx context.Context) (*sql.DB, error) {
	db, err := e.environment.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to environment for migrations: %w", err)
	}

	err = e.migrations.Up(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("up migrations: %w", err)
	}

	return db, nil
}

func Environment(mig Migrations, env dbenv.Environment) dbenv.Environment {
	return &environment{
		migrations:  mig,
		environment: env,
	}
}
