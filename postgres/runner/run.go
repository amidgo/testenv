// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package postgresrunner

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/amidgo/testenv"
	"github.com/amidgo/testenv/postgres"
	"github.com/amidgo/testenv/postgres/migrations"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func RunForTestingConfig(
	t *testing.T,
	cfg *Config,
	migrations migrations.Migrations,
	initialQueries ...migrations.Query,
) *sql.DB {
	testenv.SkipDisabled(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	db, term, err := RunConfig(ctx, cfg, migrations, initialQueries...)
	t.Cleanup(term)

	if err != nil {
		t.Fatal(err)

		return nil
	}

	return db
}

func RunForTesting(
	t *testing.T,
	migrations migrations.Migrations,
	initialQueries ...migrations.Query,
) *sql.DB {
	return RunForTestingConfig(
		t,
		nil,
		migrations,
		initialQueries...,
	)
}

func Run(
	ctx context.Context,
	migrations migrations.Migrations,
	initialQueries ...migrations.Query,
) (db *sql.DB, term func(), err error) {
	return RunConfig(ctx, nil, migrations, initialQueries...)
}

func RunConfig(
	ctx context.Context,
	cfg *Config,
	migrations migrations.Migrations,
	initialQueries ...migrations.Query,
) (db *sql.DB, term func(), err error) {
	env, err := RunContainer(cfg)(ctx)
	if err != nil {
		return nil, func() {}, err
	}

	return postgresenv.Init(ctx, env, migrations, initialQueries...)
}

type Config struct {
	DBName                    string
	DBUser                    string
	DBPassword                string
	PostgresImage             string
	DriverName                string
	DisableTestContainersLogs bool
}

func configDBName(cfg *Config) string {
	const defaultDBName = "test"

	if cfg != nil && cfg.DBName != "" {
		return cfg.DBName
	}

	return defaultDBName
}

func configDBUser(cfg *Config) string {
	const defaultDBUser = "admin"

	if cfg != nil && cfg.DBUser != "" {
		return cfg.DBUser
	}

	return defaultDBUser
}

func configDBPassword(cfg *Config) string {
	const defaultDBPassword = "admin"

	if cfg != nil && cfg.DBPassword != "" {
		return cfg.DBPassword
	}

	return defaultDBPassword
}

func configPostgresImage(cfg *Config) string {
	const defaultPostgresImage = "postgres:16-alpine"

	if cfg != nil && cfg.PostgresImage != "" {
		return cfg.PostgresImage
	}

	envPostgresImage := os.Getenv("CONTAINERS_POSTGRES_IMAGE")
	if envPostgresImage != "" {
		return envPostgresImage
	}

	return defaultPostgresImage
}

func configDriverName(cfg *Config) string {
	const defaultDriverName = "pgx"

	if cfg != nil && cfg.DriverName != "" {
		return cfg.DriverName
	}

	return defaultDriverName
}

func configDisableTestContainersLogs(cfg *Config) bool {
	if cfg == nil {
		return false
	}

	return cfg.DisableTestContainersLogs
}

func RunContainer(cfg *Config) postgresenv.ProvideEnvironmentFunc {
	return func(ctx context.Context) (postgresenv.Environment, error) {
		postgresImage := configPostgresImage(cfg)
		dbName := configDBName(cfg)
		dbUser := configDBUser(cfg)
		dbPassword := configDBPassword(cfg)

		opts := []testcontainers.ContainerCustomizer{
			postgres.WithDatabase(dbName),
			postgres.WithUsername(dbUser),
			postgres.WithPassword(dbPassword),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2),
			),
		}

		if configDisableTestContainersLogs(cfg) {
			opts = append(opts, testcontainers.WithLogger(noopLogger{}))
		}

		postgresContainer, err := postgres.Run(ctx,
			postgresImage,
			opts...,
		)
		if err != nil {
			return nil, fmt.Errorf("postgres.Run: %w", err)
		}

		driverName := configDriverName(cfg)

		env := environment{
			driverName:        driverName,
			postgresContainer: postgresContainer,
		}

		return env, nil
	}

}

type noopLogger struct{}

func (noopLogger) Printf(string, ...interface{}) {}

type environment struct {
	driverName        string
	postgresContainer *postgres.PostgresContainer
}

func (c environment) Connect(ctx context.Context, args ...string) (*sql.DB, error) {
	dataSourceName, err := c.postgresContainer.ConnectionString(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("*postgres.PostgresContainer.ConnectionString: %w", err)
	}

	if testing.Testing() {
		log.Printf("dataSourceName: %s", dataSourceName)
	}

	db, err := sql.Open(c.driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	return db, nil
}

func (c environment) Terminate(ctx context.Context) error {
	return c.postgresContainer.Terminate(ctx)
}
