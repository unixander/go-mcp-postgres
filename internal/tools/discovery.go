package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"mcp-postgres/internal/connections"
	"mcp-postgres/internal/resolver"
)

type PingResult struct {
	OK            bool    `json:"ok"`
	LatencyMs     float64 `json:"latency_ms"`
	ServerVersion string  `json:"server_version"`
}

func PingStand(ctx context.Context, mgr *connections.Manager, group, contour, database string) *PingResult {
	start := time.Now()
	pool, err := mgr.Acquire(ctx, group, contour, database)
	if err != nil {
		return &PingResult{OK: false}
	}
	var version string
	err = pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return &PingResult{OK: false, LatencyMs: latency}
	}
	return &PingResult{OK: true, LatencyMs: latency, ServerVersion: version}
}

type GroupInfo struct {
	Name     string   `json:"name"`
	Contours []string `json:"contours"`
	Writable bool     `json:"writable"`
}

type StandInfo struct {
	Contour   string  `json:"contour"`
	Host      string  `json:"host"`
	Reachable *bool   `json:"reachable"`
}

type DBInfo struct {
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	SizeBytes int64  `json:"size_bytes"`
}

type SchemaInfo struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
}

func ListGroups(res *resolver.Resolver) []GroupInfo {
	cfg := res.Config()
	out := make([]GroupInfo, 0, len(cfg.Groups))
	for _, g := range cfg.Groups {
		out = append(out, GroupInfo{
			Name:     g.Name,
			Contours: cfg.Contours,
			Writable: g.Writable,
		})
	}
	return out
}

func ListStands(res *resolver.Resolver, group string, probe bool, mgr *connections.Manager) ([]StandInfo, error) {
	cfg := res.Config()
	_, ok := res.Group(group)
	if !ok {
		return nil, fmt.Errorf("group %q not found", group)
	}

	out := make([]StandInfo, 0, len(cfg.Contours))
	for _, contour := range cfg.Contours {
		host, err := res.ResolveHost(group, contour)
		if err != nil {
			return nil, err
		}
		info := StandInfo{Contour: contour, Host: host}

		if probe {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			pool, err := mgr.Acquire(ctx, group, contour, cfg.Defaults.DefaultDatabase)
			cancel()
			reachable := false
			if err == nil {
				ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
				err = pool.Ping(ctx2)
				cancel2()
				reachable = err == nil
			}
			info.Reachable = &reachable
		}
		out = append(out, info)
	}
	return out, nil
}

func ListDatabases(ctx context.Context, mgr *connections.Manager, res *resolver.Resolver, group, contour string) ([]DBInfo, error) {
	db := res.Config().Defaults.DefaultDatabase
	pool, err := mgr.Acquire(ctx, group, contour, db)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `
		SELECT datname, pg_get_userbyid(datdba), pg_database_size(datname)
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY datname`)
	if err != nil {
		return nil, fmt.Errorf("list_databases: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (DBInfo, error) {
		var d DBInfo
		return d, row.Scan(&d.Name, &d.Owner, &d.SizeBytes)
	})
}

func ListSchemas(ctx context.Context, mgr *connections.Manager, group, contour, database string) ([]SchemaInfo, error) {
	pool, err := mgr.Acquire(ctx, group, contour, database)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `
		SELECT schema_name, schema_owner
		FROM information_schema.schemata
		WHERE schema_name NOT LIKE 'pg_%'
		  AND schema_name != 'information_schema'
		ORDER BY schema_name`)
	if err != nil {
		return nil, fmt.Errorf("list_schemas: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SchemaInfo, error) {
		var s SchemaInfo
		return s, row.Scan(&s.Name, &s.Owner)
	})
}
