package test

import (
	"context"
	"testing"
	"time"

	"mcp-postgres/internal/connections"

	"mcp-postgres/internal/tools"
)

func TestSchemaQueriesReleaseConnection(t *testing.T) {
	// A single connection exposes both leaked SET rows and using a different
	// connection for schema selection and execution.
	cfg := *testCfg
	cfg.Defaults.PoolMaxSize = 1
	mgr := connections.NewManager(testRes, &cfg)
	defer mgr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := mgr.Acquire(ctx, testGroup, testContour, testDB)
	if err != nil {
		t.Fatal(err)
	}
	var originalPath string
	if err := pool.QueryRow(ctx, "SHOW search_path").Scan(&originalPath); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 8; i++ {
		schema := "pg_catalog"
		result, err := tools.ExecuteQuery(ctx, mgr, testRes, testGroup, testContour, testDB,
			"SELECT current_schema(), current_setting('transaction_read_only')", nil, &schema, 10, false)
		if err != nil {
			t.Fatalf("schema query %d: %v", i, err)
		}
		if result.RowCount != 1 || result.Rows[0][0] != schema || result.Rows[0][1] != "on" {
			t.Fatalf("unexpected schema/read-only result: %+v", result)
		}
		if pool.Stat().AcquiredConns() != 0 {
			t.Fatal("connection remained acquired after schema query")
		}
	}
	var path string
	if err := pool.QueryRow(ctx, "SHOW search_path").Scan(&path); err != nil {
		t.Fatal(err)
	}
	if path != originalPath {
		t.Fatalf("schema leaked into the session: got %q, want %q", path, originalPath)
	}

	schema := "pg_catalog"
	_, err = tools.ExecuteQuery(ctx, mgr, testRes, testGroup, testContour, testDB,
		"SELECT * FROM _nonexistent_schema_regression_table", nil, &schema, 10, false)
	if err == nil {
		t.Fatal("expected query failure")
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool unusable after query failure: %v", err)
	}

	queryCtx, queryCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer queryCancel()
	_, err = tools.ExecuteQuery(queryCtx, mgr, testRes, testGroup, testContour, testDB,
		"SELECT pg_sleep(10)", nil, &schema, 10, false)
	if err == nil {
		t.Fatal("expected query cancellation")
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool unusable after query cancellation: %v", err)
	}
}

func TestExecuteQuerySelect(t *testing.T) {
	ctx := context.Background()
	result, err := tools.ExecuteQuery(ctx, testMgr, testRes, testGroup, testContour, testDB,
		"SELECT 1 AS one, 'hello' AS word", nil, nil, 10, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", result.RowCount)
	}
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}
}

func TestExecuteQueryParams(t *testing.T) {
	ctx := context.Background()
	result, err := tools.ExecuteQuery(ctx, testMgr, testRes, testGroup, testContour, testDB,
		"SELECT $1::int + $2::int AS sum", []any{3, 4}, nil, 10, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", result.RowCount)
	}
}

func TestExecuteQueryWritePath(t *testing.T) {
	ctx := context.Background()
	// Create temp table, insert, select — all in writable group.
	_, err := tools.ExecuteQuery(ctx, testMgr, testRes, "writable-group", testContour, testDB,
		"CREATE TEMP TABLE wtest (v INT)", nil, nil, 10, true)
	if err != nil {
		t.Fatalf("create temp table: %v", err)
	}
	_, err = tools.ExecuteQuery(ctx, testMgr, testRes, "writable-group", testContour, testDB,
		"INSERT INTO wtest VALUES (42)", nil, nil, 10, true)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	result, err := tools.ExecuteQuery(ctx, testMgr, testRes, "writable-group", testContour, testDB,
		"SELECT v FROM wtest", nil, nil, 10, false)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if result.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", result.RowCount)
	}
}

func TestExplainQuery(t *testing.T) {
	ctx := context.Background()
	result, err := tools.ExplainQuery(ctx, testMgr, testGroup, testContour, testDB,
		"SELECT 1", nil, false)
	if err != nil {
		t.Fatalf("ExplainQuery: %v", err)
	}
	if result.Plan == "" {
		t.Error("expected non-empty plan")
	}
}

func TestExplainQueryAnalyze(t *testing.T) {
	ctx := context.Background()
	result, err := tools.ExplainQuery(ctx, testMgr, testGroup, testContour, testDB,
		"SELECT generate_series(1,5)", nil, true)
	if err != nil {
		t.Fatalf("ExplainQuery analyze: %v", err)
	}
	if result.Plan == "" {
		t.Error("expected non-empty plan")
	}
}
