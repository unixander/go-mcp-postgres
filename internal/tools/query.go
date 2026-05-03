package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"mcp-postgres/internal/audit"
	"mcp-postgres/internal/connections"
	"mcp-postgres/internal/resolver"
)

type QueryResult struct {
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	RowCount int      `json:"row_count"`
	Truncated bool    `json:"truncated"`
}

type ExplainResult struct {
	Plan string `json:"plan"`
}

// ExecuteQuery runs sql against the given stand. write=true is only allowed when
// the group has writable:true in config; otherwise the read-only session guard
// at the Postgres level will reject it anyway.
func ExecuteQuery(
	ctx context.Context,
	mgr *connections.Manager,
	res *resolver.Resolver,
	group, contour, database, sql string,
	params []any,
	schema *string,
	maxRows int,
	write bool,
) (*QueryResult, error) {
	g, ok := res.Group(group)
	if !ok {
		return nil, fmt.Errorf("group %q not found", group)
	}
	if write && !g.Writable {
		return nil, fmt.Errorf("group %q is not configured as writable", group)
	}

	cfg := res.Config()
	if maxRows <= 0 || maxRows > cfg.Defaults.MaxRows {
		maxRows = cfg.Defaults.MaxRows
	}

	pool, err := mgr.Acquire(ctx, group, contour, database)
	if err != nil {
		return nil, err
	}

	start := audit.Now()

	var result *QueryResult
	if write {
		result, err = execWrite(ctx, pool, sql, params, schema, maxRows)
	} else {
		result, err = execRead(ctx, pool, sql, params, schema, maxRows)
	}

	durationMs := time.Since(start).Milliseconds()
	rowCount := 0
	if result != nil {
		rowCount = result.RowCount
	}

	audit.Log(ctx, audit.Entry{
		Group:      group,
		Contour:    contour,
		Database:   database,
		Write:      write,
		RowCount:   rowCount,
		DurationMs: durationMs,
		SQLHash:    audit.HashSQL(sql),
		SQL:        sql,
	})

	return result, err
}

func execRead(ctx context.Context, pool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, sql string, params []any, schema *string, maxRows int) (*QueryResult, error) {
	if schema != nil {
		schemaID := pgx.Identifier{*schema}.Sanitize()
		if _, err := pool.Query(ctx, fmt.Sprintf("SET search_path TO %s", schemaID)); err != nil {
			return nil, fmt.Errorf("set search_path: %w", err)
		}
	}

	rows, err := pool.Query(ctx, sql, params...)
	if err != nil {
		return nil, wrapPGError(err)
	}
	defer rows.Close()

	return collectRows(rows, maxRows)
}

func execWrite(ctx context.Context, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, sql string, params []any, schema *string, maxRows int) (*QueryResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if schema != nil {
		schemaID := pgx.Identifier{*schema}.Sanitize()
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL search_path TO %s", schemaID)); err != nil {
			return nil, fmt.Errorf("set search_path: %w", err)
		}
	}

	rows, err := tx.Query(ctx, sql, params...)
	if err != nil {
		return nil, wrapPGError(err)
	}

	result, err := collectRows(rows, maxRows)
	rows.Close()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return result, nil
}

func collectRows(rows pgx.Rows, maxRows int) (*QueryResult, error) {
	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i, fd := range fds {
		cols[i] = string(fd.Name)
	}

	var data [][]any
	for rows.Next() {
		if len(data) >= maxRows {
			// drain remainder so rows.Err() works
			for rows.Next() {
			}
			r := &QueryResult{Columns: cols, Rows: data, RowCount: len(data), Truncated: true}
			return r, rows.Err()
		}
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		data = append(data, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &QueryResult{Columns: cols, Rows: data, RowCount: len(data), Truncated: false}, nil
}

func ExplainQuery(
	ctx context.Context,
	mgr *connections.Manager,
	group, contour, database, sql string,
	params []any,
	analyze bool,
) (*ExplainResult, error) {
	pool, err := mgr.Acquire(ctx, group, contour, database)
	if err != nil {
		return nil, err
	}

	keyword := "EXPLAIN"
	if analyze {
		keyword = "EXPLAIN ANALYZE"
	}

	explainSQL := fmt.Sprintf("%s %s", keyword, sql)
	rows, err := pool.Query(ctx, explainSQL, params...)
	if err != nil {
		return nil, wrapPGError(err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	plan := ""
	for _, l := range lines {
		plan += l + "\n"
	}
	return &ExplainResult{Plan: plan}, nil
}

func wrapPGError(err error) error { return err }
