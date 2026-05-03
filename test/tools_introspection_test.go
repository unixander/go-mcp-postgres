package test

import (
	"context"
	"testing"

	"mcp-postgres/internal/tools"
)

// seed creates a test table for introspection tests.
func seed(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Use writable-group to create the table.
	_, err := tools.ExecuteQuery(ctx, testMgr, testRes, "writable-group", testContour, testDB, `
		CREATE TABLE IF NOT EXISTS public.introspect_test (
			id   SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			val  INT
		)`, nil, nil, 10, true)
	if err != nil {
		t.Fatalf("seed: create table: %v", err)
	}
	t.Cleanup(func() {
		tools.ExecuteQuery(ctx, testMgr, testRes, "writable-group", testContour, testDB, //nolint:errcheck
			"DROP TABLE IF EXISTS public.introspect_test", nil, nil, 10, true)
	})
}

func TestListTables(t *testing.T) {
	seed(t)
	ctx := context.Background()
	schema := "public"
	tables, err := tools.ListTables(ctx, testMgr, testGroup, testContour, testDB, &schema)
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	found := false
	for _, tb := range tables {
		if tb.Name == "introspect_test" {
			found = true
		}
	}
	if !found {
		t.Error("introspect_test table not found")
	}
}

func TestDescribeTable(t *testing.T) {
	seed(t)
	ctx := context.Background()
	detail, err := tools.DescribeTable(ctx, testMgr, testGroup, testContour, testDB, "public", "introspect_test")
	if err != nil {
		t.Fatalf("DescribeTable: %v", err)
	}
	if len(detail.Columns) == 0 {
		t.Error("expected columns")
	}
	colNames := make(map[string]bool)
	for _, c := range detail.Columns {
		colNames[c.Name] = true
	}
	for _, want := range []string{"id", "name", "val"} {
		if !colNames[want] {
			t.Errorf("column %q not found", want)
		}
	}
	if len(detail.Indexes) == 0 {
		t.Error("expected at least primary key index")
	}
}
