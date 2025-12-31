package postgres

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/amidgo/testenv/internal/reuse"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	*pgxpool.Pool

	exit func()
}

func (p *Pool) Close() {
	defer func() {
		if p.exit != nil {
			p.exit()
		}
	}()

	p.Pool.Close()
}

type ReuseStrategy interface {
	strategy() ReuseStrategy
}

type ReuseStrategyCreateNewDB struct {
	NewDBName func() string
}

func (r *ReuseStrategyCreateNewDB) strategy() ReuseStrategy { return r }

type ReuseStrategyCreateNewSchema struct {
	DB            string
	NewSchemaName func() string
}

func (r *ReuseStrategyCreateNewSchema) strategy() ReuseStrategy { return r }

type Environment struct {
	reuseStrategy     ReuseStrategy
	maxConnPerConnect int32

	connectorReuse *reuse.Reuse[*Connector]
}

type EnvironmentOption func(*Environment)

func WithReuseStrategy(strategy ReuseStrategy) EnvironmentOption {
	return func(e *Environment) {
		e.reuseStrategy = strategy
	}
}

func WithConnectorReuseOptions(
	createFunc func() (*Connector, error),
	opts ...reuse.Option[*Connector],
) EnvironmentOption {
	return func(e *Environment) {
		if e.connectorReuse != nil {
			e.connectorReuse.Kill()
		}

		e.connectorReuse = reuse.Run(createFunc, opts...)
	}
}

func WithMaxConnPerConnect(maxConnPerConnect int32) EnvironmentOption {
	return func(e *Environment) {
		e.maxConnPerConnect = maxConnPerConnect
	}
}

func NewEnvironment(opts ...EnvironmentOption) *Environment {
	env := &Environment{
		reuseStrategy:     ReuseStrategy(nil),
		maxConnPerConnect: 1,
		connectorReuse: reuse.Run(func() (*Connector, error) {
			return NewConnector(), nil
		}),
	}

	for _, op := range opts {
		op(env)
	}

	return env
}

func (e *Environment) Connect(ctx context.Context) (*Pool, error) {
	switch reuseStrategy := e.reuseStrategy.(type) {
	case *ReuseStrategyCreateNewSchema:
		return e.connectToNewSchema(ctx, reuseStrategy)

	case *ReuseStrategyCreateNewDB:
		return e.connectToNewDB(ctx, reuseStrategy)

	default:
		return e.connectToRawPostgres(ctx)
	}
}

func (e *Environment) connectToNewSchema(
	ctx context.Context,
	reuseStrategy *ReuseStrategyCreateNewSchema,
) (*Pool, error) {
	entered, err := e.connectorReuse.Enter()
	if err != nil {
		return nil, fmt.Errorf("enter to connector reuse: %w", err)
	}

	connector := entered.Value()

	moved := false

	defer func() {
		if !moved {
			entered.Exit()
		}
	}()

	pool, err := connector.Connect(ctx, &pgxpool.Config{
		ConnConfig: &pgx.ConnConfig{
			Config: pgconn.Config{
				Database: reuseStrategy.DB,
			},
		},
		MaxConns: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("connecto to postgres %s database: %w", reuseStrategy.DB, err)
	}

	if reuseStrategy.NewSchemaName == nil {
		reuseStrategy.NewSchemaName = rand.Text
	}

	schemaName := reuseStrategy.NewSchemaName()

	query := fmt.Sprintf("CREATE SCHEMA %s", schemaName)

	_, err = pool.Exec(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("create %q schema, %w", schemaName, err)
	}

	pool.Close()

	pool, err = connector.Connect(ctx,
		&pgxpool.Config{
			ConnConfig: &pgx.ConnConfig{
				Config: pgconn.Config{
					Database: reuseStrategy.DB,
					RuntimeParams: map[string]string{
						"search_path": schemaName,
					},
				},
			},
			MaxConns: e.maxConnPerConnect,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("connect to %q schema, %w", schemaName, err)
	}

	moved = true

	return &Pool{Pool: pool, exit: entered.Exit}, nil
}

func (e *Environment) connectToNewDB(
	ctx context.Context,
	reuseStrategy *ReuseStrategyCreateNewDB,
) (*Pool, error) {
	entered, err := e.connectorReuse.Enter()
	if err != nil {
		return nil, fmt.Errorf("enter to connector reuse: %w", err)
	}

	connector := entered.Value()

	moved := false

	defer func() {
		if !moved {
			entered.Exit()
		}
	}()

	pool, err := connector.Connect(ctx, &pgxpool.Config{
		MaxConns: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("create base pool: %w", err)
	}

	if reuseStrategy.NewDBName == nil {
		reuseStrategy.NewDBName = rand.Text
	}

	dbName := reuseStrategy.NewDBName()

	query := fmt.Sprintf("CREATE DATABASE %s", dbName)

	_, err = pool.Exec(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("create %q database, %w", dbName, err)
	}

	pool.Close()

	pool, err = connector.Connect(ctx,
		&pgxpool.Config{
			ConnConfig: &pgx.ConnConfig{
				Config: pgconn.Config{
					Database: dbName,
				},
			},
			MaxConns: e.maxConnPerConnect,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("connect to %q database, %w", dbName, err)
	}

	moved = true

	return &Pool{Pool: pool, exit: entered.Exit}, nil
}

func (e *Environment) connectToRawPostgres(ctx context.Context) (*Pool, error) {
	entered, err := e.connectorReuse.Enter()
	if err != nil {
		return nil, fmt.Errorf("enter to connector reuse: %w", err)
	}

	connector := entered.Value()

	moved := false

	defer func() {
		if !moved {
			entered.Exit()
		}
	}()

	pool, err := connector.Connect(ctx, &pgxpool.Config{
		MaxConns: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	moved = true

	return &Pool{Pool: pool, exit: entered.Exit}, nil
}
