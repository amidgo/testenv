// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package postgresenv

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/amidgo/testenv/postgres/migrations"
)

func Init(
	ctx context.Context,
	env Environment,
	mig migrations.Migrations,
	initialQueries ...migrations.Query,
) (db *sql.DB, term func(), err error) {
	term = func() {
		terminateErr := env.Terminate(ctx)
		if terminateErr != nil {
			log.Printf("failed to terminate postgres environment: %s", terminateErr)
		}
	}

	db, err = env.Connect(ctx, "sslmode=disable")
	if err != nil {
		return nil, term, fmt.Errorf("connect to db, %w", err)
	}

	term = func() {
		_ = db.Close()

		terminateErr := env.Terminate(ctx)
		if terminateErr != nil {
			log.Printf("failed to terminate postgres environment: %s", terminateErr)
		}
	}

	if mig != nil {
		err = mig.Up(ctx, db)
		if err != nil {
			return db, term, fmt.Errorf("up migrations, %w", err)
		}
	}

	for _, initialQuery := range initialQueries {
		err = migrations.ExecQuery(ctx, db, initialQuery)
		if err != nil {
			return db, term, err
		}
	}

	return db, term, nil
}
