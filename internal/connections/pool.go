package connections

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"mcp-postgres/internal/config"
	"mcp-postgres/internal/resolver"
	"mcp-postgres/internal/safety"
)

type poolKey struct {
	group, contour, database string
}

// Manager holds one pgxpool.Pool per (group, contour, database) triple.
type Manager struct {
	mu      sync.Mutex
	pools   map[poolKey]*pgxpool.Pool
	res     *resolver.Resolver
	cfg     *config.Config
}

func NewManager(res *resolver.Resolver, cfg *config.Config) *Manager {
	return &Manager{
		pools: make(map[poolKey]*pgxpool.Pool),
		res:   res,
		cfg:   cfg,
	}
}

// Acquire returns (or lazily creates) the pool for the given triple.
func (m *Manager) Acquire(ctx context.Context, group, contour, database string) (*pgxpool.Pool, error) {
	key := poolKey{group, contour, database}

	m.mu.Lock()
	if p, ok := m.pools[key]; ok {
		m.mu.Unlock()
		return p, nil
	}
	m.mu.Unlock()

	// Build outside the lock to avoid blocking other acquires.
	pool, err := m.build(ctx, group, contour, database)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Another goroutine may have won the race; use its pool.
	if existing, ok := m.pools[key]; ok {
		pool.Close()
		return existing, nil
	}
	m.pools[key] = pool
	return pool, nil
}

func (m *Manager) build(ctx context.Context, group, contour, database string) (*pgxpool.Pool, error) {
	dsn, err := m.res.ResolveDSN(group, contour, database)
	if err != nil {
		return nil, err
	}

	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DSN: %w", err)
	}

	d := m.cfg.Defaults
	pcfg.MinConns = int32(d.PoolMinSize)
	pcfg.MaxConns = int32(d.PoolMaxSize)
	pcfg.AfterConnect = safety.AfterConnect(safety.SessionConfig{
		StatementTimeoutMs:         d.StatementTimeoutMs,
		IdleInTransactionTimeoutMs: d.IdleInTransactionTimeoutMs,
	})

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("create pool [%s/%s/%s]: %w", group, contour, database, err)
	}
	return pool, nil
}

// Close closes all pools. Call on server shutdown.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, p := range m.pools {
		p.Close()
		delete(m.pools, k)
	}
}
