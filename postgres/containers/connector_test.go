// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package containers

import (
	"context"
	"testing"

	"github.com/amidgo/testenv/postgres"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func Test_Connector_Ping(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	containerClient, err := client.NewClientWithOpts()
	require.NoError(t, err)

	postgresContainer := NewContainer()

	connector := postgres.NewConnector(
		postgresContainer.ConnectorOptions()...,
	)

	var containerID string

	t.Cleanup(func() {
		resp, err := containerClient.ContainerList(context.Background(),
			container.ListOptions{
				Filters: filters.NewArgs(
					filters.Arg("id", containerID),
				),
			},
		)
		require.NoError(t, err)
		require.Empty(t, resp)
	})

	t.Cleanup(func() {
		err := connector.Close()
		if err != nil {
			t.Logf("close container finished with error: %s", err)

			return
		}

		t.Log("close container successfully")
	})

	pool, err := connector.Connect(ctx, (*pgxpool.Config)(nil))
	require.NoError(t, err)

	containerID = postgresContainer.cnt.GetContainerID()

	err = pool.Ping(ctx)
	require.NoError(t, err)
}
