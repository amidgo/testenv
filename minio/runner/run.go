// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package miniorunner

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	minioenv "github.com/amidgo/testenv/minio"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/amidgo/testenv"

	miniocnt "github.com/testcontainers/testcontainers-go/modules/minio"
)

func RunForTesting(
	t *testing.T,
	buckets ...minioenv.Bucket,
) *minio.Client {
	return RunForTestingConfig(t, nil, buckets...)
}

func RunForTestingConfig(
	t *testing.T,
	cfg *Config,
	buckets ...minioenv.Bucket,
) *minio.Client {
	testenv.SkipDisabled(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	minioClient, term, err := RunConfig(ctx, cfg, buckets...)
	t.Cleanup(term)

	if err != nil {
		t.Fatalf("run minio container with config, %s", err.Error())

		return nil
	}

	return minioClient
}

func Run(
	ctx context.Context,
	buckets ...minioenv.Bucket,
) (minioClient *minio.Client, term func(), err error) {
	return RunConfig(ctx, nil, buckets...)
}

func RunConfig(
	ctx context.Context,
	cfg *Config,
	buckets ...minioenv.Bucket,
) (minioClient *minio.Client, term func(), err error) {
	env, err := RunContainer(cfg)()
	if err != nil {
		return nil, func() {}, fmt.Errorf("run container, %w", err)
	}

	return minioenv.Init(ctx, env, buckets...)
}

type Config struct {
	MinioImage string
	Username   string
	Password   string
	Timeout    time.Duration
}

func configMinioImage(cfg *Config) string {
	const defaultMinioImage = "minio/minio:RELEASE.2024-01-16T16-07-38Z"

	if cfg != nil && cfg.MinioImage != "" {
		return cfg.MinioImage
	}

	postgresImage := os.Getenv("CONTAINERS_MINIO_IMAGE")
	if postgresImage != "" {
		return postgresImage
	}

	return defaultMinioImage
}

func configUsername(cfg *Config) string {
	const defaultUsername = "minioadmin"

	if cfg == nil || cfg.Username == "" {
		return defaultUsername
	}

	return cfg.Username
}

func configPassword(cfg *Config) string {
	const defaultPassword = "minioadmin"

	if cfg == nil || cfg.Password == "" {
		return defaultPassword
	}

	return cfg.Password
}

func configTimeout(cfg *Config) time.Duration {
	const defaultTimeout = time.Second * 3

	if cfg == nil || cfg.Timeout <= 0 {
		return defaultTimeout
	}

	return cfg.Timeout
}

func RunContainer(cfg *Config) minioenv.ProvideEnvironmentFunc {
	return func() (minioenv.Environment, error) {
		minioImage := configMinioImage(cfg)
		username := configUsername(cfg)
		password := configPassword(cfg)

		ctx, cancel := context.WithTimeout(context.Background(), configTimeout(cfg))
		defer cancel()

		minioContainer, err := miniocnt.Run(ctx,
			minioImage,
			miniocnt.WithUsername(username),
			miniocnt.WithPassword(password),
		)
		if err != nil {
			return nil, fmt.Errorf("run minio container, %w", err)
		}

		return container{
			minioContainer: minioContainer,
		}, nil
	}
}

type container struct {
	minioContainer *miniocnt.MinioContainer
}

func (c container) Connect(ctx context.Context) (*minio.Client, error) {
	endpoint, err := c.minioContainer.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to minio container, get endpoint, %w", err)
	}

	opts := &minio.Options{
		Creds: credentials.NewStaticV4(c.minioContainer.Username, c.minioContainer.Password, ""),
	}

	minioClient, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("create minio client, %w", err)
	}

	return minioClient, nil
}

func (c container) Terminate(ctx context.Context) error {
	err := c.minioContainer.Terminate(ctx)
	if err != nil {
		return fmt.Errorf("terminate minio container: %w", err)
	}

	return nil
}
