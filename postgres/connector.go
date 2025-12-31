// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package postgres

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type (
	BeforeConnectHook func(ctx context.Context, cfg *pgxpool.Config) error
	CloseHook         func() error
)

type Connector struct {
	host   string
	port   uint16
	db     string
	schema string

	user     string
	password string

	tlsConfig *tls.Config

	beforeConnectHook BeforeConnectHook
	closeHook         CloseHook
}

type ConnectorOption func(*Connector)

func WithHost(host string) ConnectorOption {
	return func(ec *Connector) {
		ec.host = host
	}
}

func WithPort(port uint16) ConnectorOption {
	return func(ec *Connector) {
		ec.port = port
	}
}

func WithDB(db string) ConnectorOption {
	return func(ec *Connector) {
		ec.db = db
	}
}

func WithSchema(schema string) ConnectorOption {
	return func(ec *Connector) {
		ec.schema = schema
	}
}

func WithUser(user string) ConnectorOption {
	return func(ec *Connector) {
		ec.user = user
	}
}

func WithPassword(password string) ConnectorOption {
	return func(ec *Connector) {
		ec.password = password
	}
}

func WithTLSConfig(tlsConfig *tls.Config) ConnectorOption {
	return func(ec *Connector) {
		ec.tlsConfig = tlsConfig
	}
}

func WithBeforeConnectHook(hook BeforeConnectHook) ConnectorOption {
	return func(c *Connector) {
		c.beforeConnectHook = hook
	}
}

func WithCloseHook(hook CloseHook) ConnectorOption {
	return func(c *Connector) {
		c.closeHook = hook
	}
}

func NewConnector(opts ...ConnectorOption) *Connector {
	connector := &Connector{
		host:      "0.0.0.0",
		port:      5432,
		db:        "postgres",
		schema:    "public",
		user:      "postgres",
		password:  "postgres",
		tlsConfig: (*tls.Config)(nil),
	}

	for _, op := range opts {
		op(connector)
	}

	return connector
}

func (c *Connector) Connect(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	cfg, err := c.applyConfig(cfg)
	if err != nil {
		return nil, err
	}

	if c.beforeConnectHook != nil {
		err = c.beforeConnectHook(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("beforeConnectHook: %w", err)
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgxpool with %+v config, %w", cfg, err)
	}

	return pool, nil
}

func (c *Connector) Config() (*pgxpool.Config, error) {
	connString := fmt.Sprintf(
		"host=%s port=%d dbname=%s search_path=%s user=%s password=%s",
		c.host, c.port, c.db, c.schema, c.user, c.password,
	)

	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.ConnConfig.TLSConfig = c.tlsConfig

	return cfg, nil
}

func (c *Connector) Close() error {
	if c.closeHook == nil {
		return nil
	}

	err := c.closeHook()
	if err != nil {
		return fmt.Errorf("closeHook: %w", err)
	}

	return nil
}

func (c *Connector) applyConfig(cfg *pgxpool.Config) (*pgxpool.Config, error) {
	if cfg == nil {
		cfg = new(pgxpool.Config)
	}

	baseCfg, err := c.Config()
	if err != nil {
		return nil, err
	}

	if cfg.MaxConns != 0 {
		baseCfg.MaxConns = cfg.MaxConns
	}

	inConnConfig := pgconn.Config{}

	if cfg.ConnConfig != nil {
		inConnConfig = cfg.ConnConfig.Config
	}

	if inConnConfig.Host != "" {
		baseCfg.ConnConfig.Config.Host = inConnConfig.Host
	}

	if inConnConfig.Port != 0 {
		baseCfg.ConnConfig.Config.Port = inConnConfig.Port
	}

	if inConnConfig.Database != "" {
		baseCfg.ConnConfig.Config.Database = inConnConfig.Database
	}

	if inConnConfig.RuntimeParams != nil {
		baseCfg.ConnConfig.Config.RuntimeParams = inConnConfig.RuntimeParams
	}

	if inConnConfig.User != "" {
		baseCfg.ConnConfig.Config.User = inConnConfig.User
	}

	if inConnConfig.Password != "" {
		baseCfg.ConnConfig.Config.Password = inConnConfig.Password
	}

	return baseCfg, nil
}
