// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package postgresenv

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/amidgo/testenv"

	"github.com/amidgo/testenv/postgres/migrations"
)

var externalReusable = NewReusable(ExternalEnvironment(nil))

func ExternalReusable() *Reusable {
	return externalReusable
}

func UseExternalForTestingConfig(
	t *testing.T,
	cfg *ExternalEnvironmentConfig,
	migrations migrations.Migrations,
	initialQueries ...migrations.Query,
) *sql.DB {
	testenv.SkipDisabled(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	db, term, err := UseExternalConfig(ctx, cfg, migrations, initialQueries...)
	t.Cleanup(term)

	if err != nil {
		t.Fatal(err)

		return nil
	}

	return db
}

func UseExternalForTesting(
	t *testing.T,
	migrations migrations.Migrations,
	initialQueries ...migrations.Query,
) *sql.DB {
	return UseExternalForTestingConfig(
		t,
		nil,
		migrations,
		initialQueries...,
	)
}

func UseExternalConfig(
	ctx context.Context,
	cfg *ExternalEnvironmentConfig,
	migrations migrations.Migrations,
	initialQueries ...migrations.Query,
) (db *sql.DB, term func(), err error) {
	env, err := ExternalEnvironment(cfg)(ctx)
	if err != nil {
		return nil, func() {}, err
	}

	return Init(ctx, env, migrations, initialQueries...)
}

func UseExternal(
	ctx context.Context,
	migrations migrations.Migrations,
	initialQueries ...migrations.Query,
) (db *sql.DB, term func(), err error) {
	var cfg *ExternalEnvironmentConfig

	return UseExternalConfig(
		ctx,
		cfg,
		migrations,
		initialQueries...,
	)
}

type ExternalEnvironmentConfig struct {
	DriverName       string
	ConnectionString string
}

func externalEnvironmentDriverName(cfg *ExternalEnvironmentConfig) string {
	const defaultDriverName = "pgx"

	if cfg != nil && cfg.DriverName != "" {
		return cfg.DriverName
	}

	return defaultDriverName
}

func externalEnvironmentConnectionString(cfg *ExternalEnvironmentConfig) string {
	const connectionStringEnvName = "CONTAINERS_POSTGRES_CONNECTION_STRING"

	if cfg != nil && cfg.ConnectionString != "" {
		return cfg.ConnectionString
	}

	defaultConnectionString := os.Getenv(connectionStringEnvName)

	if defaultConnectionString == "" {
		panic("connection string is empty and environment variable " + connectionStringEnvName + " is empty")
	}

	return defaultConnectionString
}

func ExternalEnvironment(cfg *ExternalEnvironmentConfig) ProvideEnvironmentFunc {
	return func(context.Context) (Environment, error) {
		connectionString := externalEnvironmentConnectionString(cfg)
		driverName := externalEnvironmentDriverName(cfg)

		return externalEnvironment{
				connectionString: connectionString,
				driverName:       driverName,
			},
			nil
	}
}

type externalEnvironment struct {
	connectionString string
	driverName       string
}

func (_ externalEnvironment) Terminate(_ context.Context) error {
	return nil
}

func (e externalEnvironment) Connect(_ context.Context, args ...string) (*sql.DB, error) {
	extraArgs := strings.Join(args, "&")

	dataSourceName := e.connectionString + "?" + extraArgs

	db, err := sql.Open(e.driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	return db, nil
}
