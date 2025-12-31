// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package minioenv

import (
	"context"

	"github.com/minio/minio-go/v7"
)

type Environment interface {
	Connect(ctx context.Context) (*minio.Client, error)
	Terminate(ctx context.Context) error
}

type ProvideEnvironmentFunc func() (Environment, error)
