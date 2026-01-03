// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package containers

import (
	"context"
	"errors"
	"log"
	"testing"

	"github.com/amidgo/testenv/postgres"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	m.Run()
}

func Test_Connector_Ping(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	postgresContainer := NewContainer(
		WithLogger(log.New(t.Output(), "", log.LstdFlags)),
	)

	connector := postgres.NewConnector(
		append(
			postgresContainer.ConnectorOptions(),
			postgres.WithPort(5432),
		)...,
	)

	var containerID string

	t.Cleanup(requireContainerIsTerminated(t, func() string { return containerID }))

	t.Cleanup(func() {
		err := connector.Close()
		if err != nil {
			t.Logf("close container finished with error: %s", err)

			return
		}

		t.Logf("close container successfully: %s", containerID)
	})

	pool, err := connector.Connect(ctx, (*pgxpool.Config)(nil))
	if err != nil {
		containerID = extractContainerID(err)
		require.NoError(t, err)
	}

	containerID = postgresContainer.cnt.GetContainerID()

	err = pool.Ping(ctx)
	require.NoError(t, err)
}

func Test_Environment_Ping(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	cnt := NewContainer(
		WithLogger(log.New(t.Output(), "", log.LstdFlags)),
	)

	env := postgres.NewEnvironment(
		postgres.WithConnectorReuseOptions(
			func() (*postgres.Connector, error) {
				return postgres.NewConnector(
					append(
						cnt.ConnectorOptions(),
						postgres.WithPort(5433),
					)...,
				), nil
			},
		),
	)

	var containerID string

	t.Cleanup(requireContainerIsTerminated(t, func() string { return containerID }))

	pool, err := env.Connect(ctx)
	if err != nil {
		containerID = extractContainerID(err)
		require.NoError(t, err)
	}

	t.Cleanup(pool.Close)

	containerID = cnt.cnt.GetContainerID()

	err = pool.Ping(ctx)
	require.NoError(t, err)
}

func extractContainerID(err error) string {
	if err == nil {
		return ""
	}

	var runError *runPostgresContainerError

	// in here is false positive, because runError value change after errors.As
	//nolint:revive // see revive:optimize-operand-order linter, nolint explanation is upper
	if errors.As(err, &runError) && runError.cnt != nil {
		return runError.cnt.GetContainerID()
	}

	return ""
}

func requireContainerIsTerminated(t *testing.T, getContainerID func() string) func() {
	return func() {
		containerID := getContainerID()
		if containerID == "" {
			return
		}

		containerClient, err := client.NewClientWithOpts()
		require.NoError(t, err)

		//nolint:usetesting // don't use t.Context here because in t.Cleanup context is canceled
		resp, err := containerClient.ContainerList(context.Background(),
			container.ListOptions{
				All: true,
				Filters: filters.NewArgs(
					filters.Arg("id", containerID),
				),
			},
		)
		require.NoErrorf(t, err, "container id: %s", containerID)
		require.Emptyf(t, resp, "container id: %s", containerID)
	}
}
