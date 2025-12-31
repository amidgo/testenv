// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

type Query any

type sqlizer interface {
	ToSql() (sql string, args []any, err error)
}

type queries struct {
	migrations Migrations
	queries    []Query
}

func (q *queries) Up(ctx context.Context, db *sql.DB) error {
	err := q.migrations.Up(ctx, db)
	if err != nil {
		return fmt.Errorf("up migrations for queries: %w", err)
	}

	for _, query := range q.queries {
		err := ExecQuery(ctx, db, query)
		if err != nil {
			return fmt.Errorf("exec query: %v", query)
		}
	}

	return nil
}

func (q *queries) Down(ctx context.Context, db *sql.DB) error {
	err := q.migrations.Down(ctx, db)
	if err != nil {
		return fmt.Errorf("down migrations: %w", err)
	}

	return nil
}

func Queries(migrations Migrations, qrs ...Query) Migrations {
	return &queries{
		migrations: migrations,
		queries:    qrs,
	}
}

func ExecQuery(ctx context.Context, db *sql.DB, query Query) error {
	switch query := query.(type) {
	case sqlizer:
		return execSqlizer(ctx, db, query)

	case string:
		return execString(ctx, db, query)

	default:
		return fmt.Errorf("invalid query type, expected string or sqlizer, got %T", query)
	}
}

func execSqlizer(ctx context.Context, db *sql.DB, query sqlizer) error {
	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("query.ToSql: %w", err)
	}

	_, err = db.ExecContext(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("*sql.DB.ExecContext(%s): %w", sql, err)
	}

	return nil
}

func execString(ctx context.Context, db *sql.DB, query string) error {
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("*sql.DB.ExecContext(%s): %w", query, err)
	}

	return nil
}
