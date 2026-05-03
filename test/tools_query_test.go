package test

import (
	"context"
	"testing"

	"mcp-postgres/internal/tools"
)

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
