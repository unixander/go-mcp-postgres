package tools

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"mcp-postgres/internal/connections"
)

type TableInfo struct {
	Schema       string `json:"schema"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	RowsEstimate int64  `json:"rows_estimate"`
}

type ColumnInfo struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable bool    `json:"nullable"`
	Default  *string `json:"default,omitempty"`
	Comment  *string `json:"comment,omitempty"`
}

type IndexInfo struct {
	Name      string `json:"name"`
	Definition string `json:"definition"`
	IsUnique  bool   `json:"is_unique"`
	IsPrimary bool   `json:"is_primary"`
}

type ConstraintInfo struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Definition string `json:"definition"`
}

type TableDetail struct {
	Columns     []ColumnInfo     `json:"columns"`
	Indexes     []IndexInfo      `json:"indexes"`
	Constraints []ConstraintInfo `json:"constraints"`
}

func ListTables(ctx context.Context, mgr *connections.Manager, group, contour, database string, schema *string) ([]TableInfo, error) {
	pool, err := mgr.Acquire(ctx, group, contour, database)
	if err != nil {
		return nil, err
	}

	q := `
		SELECT
			n.nspname AS schema,
			c.relname AS name,
			CASE c.relkind
				WHEN 'r' THEN 'table'
				WHEN 'v' THEN 'view'
				WHEN 'm' THEN 'matview'
				WHEN 'p' THEN 'partitioned'
				ELSE c.relkind::text
			END AS kind,
			c.reltuples::bigint AS rows_estimate
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r','v','m','p')
		  AND n.nspname NOT LIKE 'pg_%'
		  AND n.nspname != 'information_schema'`

	args := []any{}
	if schema != nil {
		q += " AND n.nspname = $1"
		args = append(args, *schema)
	}
	q += " ORDER BY n.nspname, c.relname"

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list_tables: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (TableInfo, error) {
		var t TableInfo
		return t, row.Scan(&t.Schema, &t.Name, &t.Kind, &t.RowsEstimate)
	})
}

func DescribeTable(ctx context.Context, mgr *connections.Manager, group, contour, database, schema, table string) (*TableDetail, error) {
	pool, err := mgr.Acquire(ctx, group, contour, database)
	if err != nil {
		return nil, err
	}

	cols, err := describeColumns(ctx, pool, schema, table)
	if err != nil {
		return nil, err
	}
	idxs, err := describeIndexes(ctx, pool, schema, table)
	if err != nil {
		return nil, err
	}
	cons, err := describeConstraints(ctx, pool, schema, table)
	if err != nil {
		return nil, err
	}
	return &TableDetail{Columns: cols, Indexes: idxs, Constraints: cons}, nil
}

func describeColumns(ctx context.Context, pool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, schema, table string) ([]ColumnInfo, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod),
			NOT a.attnotnull,
			pg_get_expr(d.adbin, d.adrelid),
			col_description(a.attrelid, a.attnum)
		FROM pg_attribute a
		LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relname = $2
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY a.attnum`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("describe columns: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ColumnInfo, error) {
		var c ColumnInfo
		return c, row.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.Comment)
	})
}

func describeIndexes(ctx context.Context, pool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, schema, table string) ([]IndexInfo, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			i.relname,
			pg_get_indexdef(ix.indexrelid),
			ix.indisunique,
			ix.indisprimary
		FROM pg_index ix
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1
		  AND t.relname = $2
		ORDER BY i.relname`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("describe indexes: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (IndexInfo, error) {
		var idx IndexInfo
		return idx, row.Scan(&idx.Name, &idx.Definition, &idx.IsUnique, &idx.IsPrimary)
	})
}

func describeConstraints(ctx context.Context, pool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, schema, table string) ([]ConstraintInfo, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			con.conname,
			CASE con.contype
				WHEN 'p' THEN 'PRIMARY KEY'
				WHEN 'f' THEN 'FOREIGN KEY'
				WHEN 'u' THEN 'UNIQUE'
				WHEN 'c' THEN 'CHECK'
				ELSE con.contype::text
			END,
			pg_get_constraintdef(con.oid)
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relname = $2
		ORDER BY con.conname`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("describe constraints: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ConstraintInfo, error) {
		var c ConstraintInfo
		return c, row.Scan(&c.Name, &c.Kind, &c.Definition)
	})
}
