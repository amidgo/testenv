// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package goosemigrations

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/amidgo/testenv/db/migrations"

	"github.com/pressly/goose/v3"
)

type Option func(m *gooseMigrations)

func WithDialect(dialect goose.Dialect) Option {
	return func(m *gooseMigrations) {
		m.dialect = dialect
	}
}

func WithProviderOptions(opts ...goose.ProviderOption) Option {
	return func(m *gooseMigrations) {
		m.providerOpts = opts
	}
}

type gooseMigrations struct {
	fsys fs.FS

	dialect      goose.Dialect
	providerOpts []goose.ProviderOption
}

func New(
	fsys fs.FS,
	opts ...Option,
) migrations.Migrations {
	mig := &gooseMigrations{
		fsys:    fsys,
		dialect: goose.DialectPostgres,
	}

	for _, op := range opts {
		op(mig)
	}

	return mig
}

func (g *gooseMigrations) newProvider(db *sql.DB) (*goose.Provider, error) {
	gooseProvider, err := goose.NewProvider(g.dialect, db, g.fsys, g.providerOpts...)
	if err != nil {
		return nil, fmt.Errorf("goose.NewProvider: %w", err)
	}

	return gooseProvider, nil
}

func (g *gooseMigrations) Up(ctx context.Context, db *sql.DB) error {
	gooseProvider, err := g.newProvider(db)
	if err != nil {
		return err
	}

	report, err := gooseProvider.Up(ctx)
	if err != nil {
		return fmt.Errorf("gooseProvider.Up: %w", err)
	}

	for _, r := range report {
		if r.Error == nil {
			continue
		}

		return fmt.Errorf("goose provider report error: %w", r.Error)
	}

	return nil
}

func (g *gooseMigrations) Down(ctx context.Context, db *sql.DB) error {
	gooseProvider, err := g.newProvider(db)
	if err != nil {
		return err
	}

	report, err := gooseProvider.DownTo(ctx, 0)
	if err != nil {
		return fmt.Errorf("gooseProvider.DownTo(0): %w", err)
	}

	for _, r := range report {
		if r.Error == nil {
			continue
		}

		return fmt.Errorf("goose provider down migrations report error: %w", r.Error)
	}

	return nil
}
