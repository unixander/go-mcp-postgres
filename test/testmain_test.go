package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"mcp-postgres/internal/config"
	"mcp-postgres/internal/connections"
	"mcp-postgres/internal/resolver"
)

const (
	testGroup   = "testgroup"
	testContour = "lkoz"
	testDB      = "postgres"
)

var (
	testCfg *config.Config
	testRes *resolver.Resolver
	testMgr *connections.Manager
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(testDB),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("testpassword"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	host, err := pgContainer.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get container host: %v\n", err)
		os.Exit(1)
	}
	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		fmt.Fprintf(os.Stderr, "get container port: %v\n", err)
		os.Exit(1)
	}

	testCfg = &config.Config{
		Admin: config.Admin{
			Username: "postgres",
			Password: "testpassword",
			Port:     int(port.Num()),
		},
		HostnameTemplate: "{group}-{contour}.example.com",
		Contours:         []string{testContour},
		Groups: []config.GroupConfig{
			{Name: testGroup, Writable: false},
			{Name: "writable-group", Writable: true},
		},
		Defaults: config.Defaults{
			StatementTimeoutMs:         5000,
			IdleInTransactionTimeoutMs: 5000,
			MaxRows:                    100,
			PoolMinSize:                0,
			PoolMaxSize:                2,
			ConnectTimeoutS:            5,
			DefaultDatabase:            testDB,
		},
	}

	// Override hostname to point at the container.
	testCfg.Groups[0].Overrides = map[string]string{testContour: host}
	testCfg.Groups[1].Overrides = map[string]string{testContour: host}

	testRes = resolver.New(testCfg)
	testMgr = connections.NewManager(testRes, testCfg)
	defer testMgr.Close()

	os.Exit(m.Run())
}
