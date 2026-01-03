// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package containers

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"sync"

	"github.com/amidgo/testenv/postgres"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	pgcnt "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type Container struct {
	image                string
	containerCustomizers []testcontainers.ContainerCustomizer
	terminateOptions     []testcontainers.TerminateOption

	logger *log.Logger

	mu  sync.Mutex
	cnt *pgcnt.PostgresContainer
}

type ContainerOption func(*Container)

func WithPostgresImage(image string) ContainerOption {
	return func(c *Container) {
		c.image = image
	}
}

func WithContainerCustomizers(customizers ...testcontainers.ContainerCustomizer) ContainerOption {
	return func(c *Container) {
		c.containerCustomizers = customizers
	}
}

func WithTerminateOptions(terminateOptions ...testcontainers.TerminateOption) ContainerOption {
	return func(c *Container) {
		c.terminateOptions = terminateOptions
	}
}

func WithLogger(logger *log.Logger) ContainerOption {
	return func(c *Container) {
		c.logger = logger
	}
}

func ConnectorOptions(opts ...ContainerOption) []postgres.ConnectorOption {
	cnt := NewContainer(opts...)

	return cnt.ConnectorOptions()
}

func NewContainer(
	opts ...ContainerOption,
) *Container {
	cnt := &Container{
		mu:                   sync.Mutex{},
		cnt:                  (*pgcnt.PostgresContainer)(nil),
		image:                "postgres:16-alpine",
		logger:               log.New(io.Discard, "", log.LstdFlags),
		containerCustomizers: []testcontainers.ContainerCustomizer(nil),
		terminateOptions:     []testcontainers.TerminateOption(nil),
	}

	for _, op := range opts {
		op(cnt)
	}

	return cnt
}

func (c *Container) ConnectorOptions() []postgres.ConnectorOption {
	return []postgres.ConnectorOption{
		postgres.WithBeforeConnectHook(c.Launch),
		postgres.WithCloseHook(c.Close),
	}
}

func (c *Container) Launch(
	ctx context.Context,
	cfg *pgxpool.Config,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Println("launching container")

	if c.cnt != nil {
		c.logger.Println("container already is launched, exit from Launch")

		return nil
	}

	customizers := []testcontainers.ContainerCustomizer{
		pgcnt.WithDatabase(cfg.ConnConfig.Database),
		pgcnt.WithUsername(cfg.ConnConfig.User),
		pgcnt.WithPassword(cfg.ConnConfig.Password),
		pgcnt.BasicWaitStrategies(),

		testcontainers.WithLogger(c.logger),

		testcontainers.CustomizeRequestOption(
			func(req *testcontainers.GenericContainerRequest) error {
				req.HostConfigModifier = func(hc *container.HostConfig) {
					hc.PortBindings = nat.PortMap{
						"5432/tcp": []nat.PortBinding{
							{
								HostIP:   cfg.ConnConfig.Host,
								HostPort: strconv.FormatUint(uint64(cfg.ConnConfig.Port), 10),
							},
						},
					}
				}

				return nil
			},
		),
	}

	customizers = append(customizers, c.containerCustomizers...)

	c.logger.Printf(
		"launching container, image: %s host: %s port: %d database: %s user: %s password: %s",
		c.image,
		cfg.ConnConfig.Host,
		cfg.ConnConfig.Port,
		cfg.ConnConfig.Database,
		cfg.ConnConfig.User,
		cfg.ConnConfig.Password,
	)

	cnt, err := pgcnt.Run(ctx,
		c.image,
		customizers...,
	)
	if err != nil {
		c.logger.Printf("failed run new postgres container: %s", err)

		_ = terminateContainer(ctx, cnt, c.logger, c.terminateOptions...)

		return &runPostgresContainerError{cnt: cnt, err: err}
	}

	c.cnt = cnt

	return nil
}

type runPostgresContainerError struct {
	cnt *pgcnt.PostgresContainer
	err error
}

func (r *runPostgresContainerError) Unwrap() error {
	return r.err
}

func (r *runPostgresContainerError) Error() string {
	return fmt.Sprintf("run postgres container: %s", r.err)
}

func terminateContainer(
	ctx context.Context,
	cnt *pgcnt.PostgresContainer,
	logger *log.Logger,
	terminateOptions ...testcontainers.TerminateOption,
) error {
	logger.Println("enter to terminateContainer")

	if cnt == nil {
		logger.Println("postgres container is nil, nothing to terminate, exit")

		return nil
	}

	logger.Println("non nil postgres container, start terminate")

	err := cnt.Terminate(ctx, terminateOptions...)
	if err != nil {
		logger.Printf("failed terminate postgres container: %s", err)

		return fmt.Errorf("terminate container: %w", err)
	}

	logger.Println("successfully terminate postgres container")

	return nil
}

func (c *Container) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := terminateContainer(context.Background(), c.cnt, c.logger, c.terminateOptions...)
	if err != nil {
		return err
	}

	c.cnt = nil

	return nil
}
