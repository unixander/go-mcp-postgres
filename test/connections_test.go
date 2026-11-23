package test

import (
	"context"
	"testing"
	"time"

	"mcp-postgres/internal/connections"
)

func TestPoolRetiresIdleConnections(t *testing.T) {
	cfg := *testCfg
	cfg.Defaults.PoolMinSize = 0
	cfg.Defaults.PoolMaxSize = 1
	cfg.Defaults.PoolMaxIdleTimeS = 1
	mgr := connections.NewManager(testRes, &cfg)
	defer mgr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := mgr.Acquire(ctx, testGroup, testContour, testDB)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	oldPID := conn.Conn().PgConn().PID()
	// Holding a connection beyond the idle timeout must not interrupt it.
	var value int
	if err := conn.QueryRow(ctx, "SELECT 1 FROM pg_sleep(2)").Scan(&value); err != nil {
		t.Fatalf("active connection interrupted: %v", err)
	}
	conn.Release()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for pool.Stat().TotalConns() != 0 {
		select {
		case <-ctx.Done():
			t.Fatal("idle connection was not retired")
		case <-ticker.C:
		}
	}
	var newPID uint32
	if err := pool.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&newPID); err != nil {
		t.Fatalf("reconnect after idle timeout: %v", err)
	}
	if newPID == oldPID {
		t.Fatal("expected a fresh PostgreSQL connection")
	}
}
