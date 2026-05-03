package test

import (
	"context"
	"testing"

	"mcp-postgres/internal/tools"
)

func TestListGroups(t *testing.T) {
	groups := tools.ListGroups(testRes)
	if len(groups) == 0 {
		t.Fatal("expected at least one group")
	}
	found := false
	for _, g := range groups {
		if g.Name == testGroup {
			found = true
		}
	}
	if !found {
		t.Errorf("testgroup %q not found in ListGroups", testGroup)
	}
}

func TestListStands(t *testing.T) {
	stands, err := tools.ListStands(testRes, testGroup, false, testMgr)
	if err != nil {
		t.Fatalf("ListStands: %v", err)
	}
	if len(stands) == 0 {
		t.Fatal("expected at least one stand")
	}
	for _, s := range stands {
		if s.Reachable != nil {
			t.Error("reachable should be nil when probe=false")
		}
	}
}

func TestListStandsUnknownGroup(t *testing.T) {
	_, err := tools.ListStands(testRes, "no-such-group", false, testMgr)
	if err == nil {
		t.Fatal("expected error for unknown group")
	}
}

func TestListDatabases(t *testing.T) {
	ctx := context.Background()
	dbs, err := tools.ListDatabases(ctx, testMgr, testRes, testGroup, testContour)
	if err != nil {
		t.Fatalf("ListDatabases: %v", err)
	}
	found := false
	for _, d := range dbs {
		if d.Name == testDB {
			found = true
		}
	}
	if !found {
		t.Errorf("database %q not found", testDB)
	}
}

func TestListSchemas(t *testing.T) {
	ctx := context.Background()
	schemas, err := tools.ListSchemas(ctx, testMgr, testGroup, testContour, testDB)
	if err != nil {
		t.Fatalf("ListSchemas: %v", err)
	}
	found := false
	for _, s := range schemas {
		if s.Name == "public" {
			found = true
		}
	}
	if !found {
		t.Error("public schema not found")
	}
}
