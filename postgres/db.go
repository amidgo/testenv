package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/stdlib"
)

type StdlibEnvironment struct {
	env  Environment
	opts []stdlib.OptionOpenDB
}

func (d *StdlibEnvironment) Connect(ctx context.Context) (*sql.DB, error) {
	pool, err := d.env.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres environment: %w", err)
	}

	connector := stdlib.GetPoolConnector(pool.Pool, d.opts...)

	connector = &closeConnectorWrapper{Connector: connector, close: pool.Close}

	db := sql.OpenDB(connector)

	return db, nil
}

func NewDB(
	env Environment,
	opts ...stdlib.OptionOpenDB,
) *StdlibEnvironment {
	return &StdlibEnvironment{
		env:  env,
		opts: opts,
	}
}

type closeConnectorWrapper struct {
	driver.Connector
	close func()
}

func (c *closeConnectorWrapper) Close() error {
	c.close()

	return nil
}
