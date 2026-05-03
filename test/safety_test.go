package test

import (
	"context"
	"strings"
	"testing"

	"mcp-postgres/internal/tools"
)

func TestReadOnlyRejectsInsert(t *testing.T) {
	ctx := context.Background()
	_, err := tools.ExecuteQuery(ctx, testMgr, testRes, testGroup, testContour, testDB,
		"CREATE TABLE _safety_test (id int)", nil, nil, 10, false)
	if err == nil {
		t.Fatal("expected error: read-only session should reject DDL")
	}
}

func TestStatementTimeout(t *testing.T) {
	ctx := context.Background()
	// pg_sleep duration exceeds the 5000ms statement_timeout set in TestMain.
	_, err := tools.ExecuteQuery(ctx, testMgr, testRes, testGroup, testContour, testDB,
		"SELECT pg_sleep(60)", nil, nil, 10, false)
	if err == nil {
		t.Fatal("expected error: statement_timeout should fire")
	}
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "57014") {
		t.Logf("got error (may still be correct): %v", err)
	}
}

func TestRowCapTruncates(t *testing.T) {
	ctx := context.Background()
	// generate_series returns 20 rows but cap is 5.
	result, err := tools.ExecuteQuery(ctx, testMgr, testRes, testGroup, testContour, testDB,
		"SELECT generate_series(1,20)", nil, nil, 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true")
	}
	if result.RowCount != 5 {
		t.Errorf("expected 5 rows, got %d", result.RowCount)
	}
}

func TestWritePathRequiresWritableGroup(t *testing.T) {
	ctx := context.Background()
	_, err := tools.ExecuteQuery(ctx, testMgr, testRes, testGroup, testContour, testDB,
		"SELECT 1", nil, nil, 10, true) // testGroup is not writable
	if err == nil {
		t.Fatal("expected error: non-writable group with write=true")
	}
}
