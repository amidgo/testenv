// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package minioenv

import (
	"context"
	"os"
	"testing"

	"github.com/amidgo/testenv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var externalReusable = NewReusable(ExternalEnvironment(nil))

func ExternalReusable() *Reusable {
	return externalReusable
}

func UseExternalForTestingConfig(
	t *testing.T,
	cfg *ExternalEnvironmentConfig,
	buckets ...Bucket,
) *minio.Client {
	testenv.SkipDisabled(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	minioClient, term, err := UseExternalConfig(ctx, cfg, buckets...)
	t.Cleanup(term)

	if err != nil {
		t.Fatal(err)

		return nil
	}

	return minioClient
}

func UseExternalForTesting(t *testing.T, buckets ...Bucket) *minio.Client {
	return UseExternalForTestingConfig(t, nil, buckets...)
}

func UseExternalConfig(
	ctx context.Context,
	cfg *ExternalEnvironmentConfig,
	buckets ...Bucket,
) (minioClient *minio.Client, term func(), err error) {
	env, err := ExternalEnvironment(cfg)()
	if err != nil {
		return nil, func() {}, err
	}

	return Init(ctx, env, buckets...)
}

func UseExternal(
	ctx context.Context,
	buckets ...Bucket,
) (minioClient *minio.Client, term func(), err error) {
	return UseExternalConfig(ctx, nil, buckets...)
}

type ExternalEnvironmentConfig struct {
	Endpoint string
	User     string
	Password string
}

func externalEnvironmentUser(cfg *ExternalEnvironmentConfig) string {
	const defaultUser = "minio"

	if cfg != nil && cfg.User != "" {
		return cfg.User
	}

	return defaultUser
}

func externalEnvironmentPassword(cfg *ExternalEnvironmentConfig) string {
	const defaultPassword = "minio"

	if cfg != nil && cfg.Password != "" {
		return cfg.Password
	}

	return defaultPassword
}

func externalEnvironmentEndpoint(cfg *ExternalEnvironmentConfig) string {
	const endpointEnvName = "CONTAINERS_MINIO_ENDPOINT"

	if cfg != nil && cfg.Endpoint != "" {
		return cfg.Endpoint
	}

	envEndpoint := os.Getenv(endpointEnvName)
	if envEndpoint == "" {
		panic("endpoint is empty and environment variable " + endpointEnvName + " is empty")
	}

	return envEndpoint
}

func ExternalEnvironment(cfg *ExternalEnvironmentConfig) ProvideEnvironmentFunc {
	return func() (Environment, error) {
		endpoint := externalEnvironmentEndpoint(cfg)
		user := externalEnvironmentUser(cfg)
		password := externalEnvironmentPassword(cfg)

		return externalEnvironment{
			endpoint: endpoint,
			userName: user,
			password: password,
		}, nil
	}
}

type externalEnvironment struct {
	endpoint string
	userName string
	password string
}

func (_ externalEnvironment) Terminate(context.Context) error {
	return nil
}

func (e externalEnvironment) Connect(ctx context.Context) (*minio.Client, error) {
	return minio.New(e.endpoint,
		&minio.Options{
			Creds:           credentials.NewStaticV4(e.userName, e.password, ""),
			TrailingHeaders: true,
		},
	)
}
