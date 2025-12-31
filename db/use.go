// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package dbenv

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

//go:generate go tool mockgen -source use.go -destination ./mocks/use_mocks.gen.go -package dbenvmocks

type Environment interface {
	Connect(ctx context.Context) (*sql.DB, error)
}

func Use(ctx context.Context, env Environment) (*sql.DB, error) {
	db, err := env.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to environment: %w", err)
	}

	return db, nil
}

func UseForTesting(
	t *testing.T,
	env Environment,
) *sql.DB {
	db, err := Use(t.Context(), env)
	if err != nil {
		t.Fatalf("Use %s environment: %s", env, err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}
