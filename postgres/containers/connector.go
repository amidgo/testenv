// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package containers

import (
	"context"
	"fmt"
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

func ConnectorOptions(opts ...ContainerOption) []postgres.ConnectorOption {
	container := NewContainer(opts...)

	return container.ConnectorOptions()
}

func NewContainer(
	opts ...ContainerOption,
) *Container {
	cnt := &Container{
		mu:    sync.Mutex{},
		cnt:   (*pgcnt.PostgresContainer)(nil),
		image: "postgres:16-alpine",
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

	if c.cnt != nil {
		return nil
	}

	customizers := []testcontainers.ContainerCustomizer{
		pgcnt.WithDatabase(cfg.ConnConfig.Database),
		pgcnt.WithUsername(cfg.ConnConfig.User),
		pgcnt.WithPassword(cfg.ConnConfig.Password),
		pgcnt.BasicWaitStrategies(),

		testcontainers.CustomizeRequestOption(func(req *testcontainers.GenericContainerRequest) error {
			req.HostConfigModifier = func(hc *container.HostConfig) {
				hc.PortBindings = nat.PortMap{
					"5432/tcp": []nat.PortBinding{
						{
							HostIP:   cfg.ConnConfig.Host,
							HostPort: fmt.Sprint(cfg.ConnConfig.Port),
						},
					},
				}
			}

			return nil
		}),
	}

	customizers = append(customizers, c.containerCustomizers...)

	cnt, err := pgcnt.Run(ctx,
		c.image,
		customizers...,
	)
	if err != nil {
		return fmt.Errorf("create postgres container: %w", err)
	}

	c.cnt = cnt

	return nil
}

func (c *Container) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cnt == nil {
		return nil
	}

	err := c.cnt.Terminate(context.Background(), c.terminateOptions...)
	if err != nil {
		return fmt.Errorf("terminate container: %w", err)
	}

	c.cnt = nil

	return nil
}
