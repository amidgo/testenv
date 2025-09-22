// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package postgresenv

import (
	"context"
	"database/sql"
)

type Environment interface {
	Connect(ctx context.Context, args ...string) (*sql.DB, error)
	Terminate(ctx context.Context) error
}

type ProvideEnvironmentFunc func(ctx context.Context) (Environment, error)
