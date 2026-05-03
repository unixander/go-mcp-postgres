package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"mcp-postgres/internal/config"
	"mcp-postgres/internal/connections"
	"mcp-postgres/internal/resolver"
	"mcp-postgres/internal/tools"
)

// Build constructs the MCP server and an HTTP mux with the MCP handler + /health.
func Build(cfg *config.Config, res *resolver.Resolver, mgr *connections.Manager) (*server.MCPServer, *http.ServeMux) {
	name := os.Getenv("MCP_SERVER_NAME")
	if name == "" {
		name = "mcp-postgres"
	}
	s := server.NewMCPServer(
		name,
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	registerTools(s, cfg, res, mgr)

	mux := http.NewServeMux()
	sh := server.NewStreamableHTTPServer(s)
	mux.Handle("/mcp", sh)
	mux.HandleFunc("/health", healthHandler)

	return s, mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok":true}`)
}

func registerTools(s *server.MCPServer, cfg *config.Config, res *resolver.Resolver, mgr *connections.Manager) {
	// ping_self
	s.AddTool(
		mcp.NewTool("ping_self",
			mcp.WithDescription("Health check: returns ok=true."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(map[string]bool{"ok": true})
		},
	)

	// list_groups
	s.AddTool(
		mcp.NewTool("list_groups",
			mcp.WithDescription("List all configured groups with their contours and write mode."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(tools.ListGroups(res))
		},
	)

	// list_stands
	s.AddTool(
		mcp.NewTool("list_stands",
			mcp.WithDescription("List stands (contour × host) for a group."),
			mcp.WithString("group", mcp.Required(), mcp.Description("Group name")),
			mcp.WithBoolean("probe", mcp.Description("Ping each host to check reachability")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			group, err := req.RequireString("group")
			if err != nil {
				return nil, err
			}
			probe := req.GetBool("probe", false)
			result, err := tools.ListStands(res, group, probe, mgr)
			if err != nil {
				return nil, err
			}
			return jsonResult(result)
		},
	)

	// list_databases
	s.AddTool(
		mcp.NewTool("list_databases",
			mcp.WithDescription("List databases on a stand."),
			mcp.WithString("group", mcp.Required(), mcp.Description("Group name")),
			mcp.WithString("contour", mcp.Required(), mcp.Description("Contour name"), mcp.Enum(cfg.Contours...)),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			group, err := req.RequireString("group")
			if err != nil {
				return nil, err
			}
			contour, err := req.RequireString("contour")
			if err != nil {
				return nil, err
			}
			result, err := tools.ListDatabases(ctx, mgr, res, group, contour)
			if err != nil {
				return nil, err
			}
			return jsonResult(result)
		},
	)

	// list_schemas
	s.AddTool(
		mcp.NewTool("list_schemas",
			mcp.WithDescription("List schemas in a database on a stand."),
			mcp.WithString("group", mcp.Required(), mcp.Description("Group name")),
			mcp.WithString("contour", mcp.Required(), mcp.Description("Contour name"), mcp.Enum(cfg.Contours...)),
			mcp.WithString("database", mcp.Required(), mcp.Description("Database name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			group, err := req.RequireString("group")
			if err != nil {
				return nil, err
			}
			contour, err := req.RequireString("contour")
			if err != nil {
				return nil, err
			}
			database, err := req.RequireString("database")
			if err != nil {
				return nil, err
			}
			result, err := tools.ListSchemas(ctx, mgr, group, contour, database)
			if err != nil {
				return nil, err
			}
			return jsonResult(result)
		},
	)

	// list_tables
	s.AddTool(
		mcp.NewTool("list_tables",
			mcp.WithDescription("List tables, views, matviews, and partitioned tables in a database."),
			mcp.WithString("group", mcp.Required(), mcp.Description("Group name")),
			mcp.WithString("contour", mcp.Required(), mcp.Description("Contour name"), mcp.Enum(cfg.Contours...)),
			mcp.WithString("database", mcp.Required(), mcp.Description("Database name")),
			mcp.WithString("schema", mcp.Description("Filter by schema name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			group, err := req.RequireString("group")
			if err != nil {
				return nil, err
			}
			contour, err := req.RequireString("contour")
			if err != nil {
				return nil, err
			}
			database, err := req.RequireString("database")
			if err != nil {
				return nil, err
			}
			var schema *string
			if s := req.GetString("schema", ""); s != "" {
				schema = &s
			}
			result, err := tools.ListTables(ctx, mgr, group, contour, database, schema)
			if err != nil {
				return nil, err
			}
			return jsonResult(result)
		},
	)

	// describe_table
	s.AddTool(
		mcp.NewTool("describe_table",
			mcp.WithDescription("Describe columns, indexes, and constraints of a table."),
			mcp.WithString("group", mcp.Required(), mcp.Description("Group name")),
			mcp.WithString("contour", mcp.Required(), mcp.Description("Contour name"), mcp.Enum(cfg.Contours...)),
			mcp.WithString("database", mcp.Required(), mcp.Description("Database name")),
			mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name")),
			mcp.WithString("table", mcp.Required(), mcp.Description("Table name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			group, err := req.RequireString("group")
			if err != nil {
				return nil, err
			}
			contour, err := req.RequireString("contour")
			if err != nil {
				return nil, err
			}
			database, err := req.RequireString("database")
			if err != nil {
				return nil, err
			}
			schema, err := req.RequireString("schema")
			if err != nil {
				return nil, err
			}
			table, err := req.RequireString("table")
			if err != nil {
				return nil, err
			}
			result, err := tools.DescribeTable(ctx, mgr, group, contour, database, schema, table)
			if err != nil {
				return nil, err
			}
			return jsonResult(result)
		},
	)

	// execute_query
	s.AddTool(
		mcp.NewTool("execute_query",
			mcp.WithDescription("Execute a SQL query against a stand. Read-only by default."),
			mcp.WithString("group", mcp.Required(), mcp.Description("Group name")),
			mcp.WithString("contour", mcp.Required(), mcp.Description("Contour name"), mcp.Enum(cfg.Contours...)),
			mcp.WithString("database", mcp.Required(), mcp.Description("Database name")),
			mcp.WithString("sql", mcp.Required(), mcp.Description("SQL query")),
			mcp.WithString("schema", mcp.Description("Set search_path to this schema")),
			mcp.WithNumber("max_rows", mcp.Description("Max rows to return (default from config)")),
			mcp.WithBoolean("write", mcp.Description("Allow writes (only valid for writable groups)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			group, err := req.RequireString("group")
			if err != nil {
				return nil, err
			}
			contour, err := req.RequireString("contour")
			if err != nil {
				return nil, err
			}
			database, err := req.RequireString("database")
			if err != nil {
				return nil, err
			}
			sql, err := req.RequireString("sql")
			if err != nil {
				return nil, err
			}
			var schema *string
			if s := req.GetString("schema", ""); s != "" {
				schema = &s
			}
			maxRows := int(req.GetFloat("max_rows", 0))
			write := req.GetBool("write", false)
			result, err := tools.ExecuteQuery(ctx, mgr, res, group, contour, database, sql, nil, schema, maxRows, write)
			if err != nil {
				return nil, err
			}
			return jsonResult(result)
		},
	)

	// explain_query
	s.AddTool(
		mcp.NewTool("explain_query",
			mcp.WithDescription("Run EXPLAIN (or EXPLAIN ANALYZE) on a query."),
			mcp.WithString("group", mcp.Required(), mcp.Description("Group name")),
			mcp.WithString("contour", mcp.Required(), mcp.Description("Contour name"), mcp.Enum(cfg.Contours...)),
			mcp.WithString("database", mcp.Required(), mcp.Description("Database name")),
			mcp.WithString("sql", mcp.Required(), mcp.Description("SQL query to explain")),
			mcp.WithBoolean("analyze", mcp.Description("Use EXPLAIN ANALYZE (executes the query)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			group, err := req.RequireString("group")
			if err != nil {
				return nil, err
			}
			contour, err := req.RequireString("contour")
			if err != nil {
				return nil, err
			}
			database, err := req.RequireString("database")
			if err != nil {
				return nil, err
			}
			sql, err := req.RequireString("sql")
			if err != nil {
				return nil, err
			}
			analyze := req.GetBool("analyze", false)
			result, err := tools.ExplainQuery(ctx, mgr, group, contour, database, sql, nil, analyze)
			if err != nil {
				return nil, err
			}
			return jsonResult(result)
		},
	)

	// ping
	s.AddTool(
		mcp.NewTool("ping",
			mcp.WithDescription("Ping a stand and return server version and latency."),
			mcp.WithString("group", mcp.Required(), mcp.Description("Group name")),
			mcp.WithString("contour", mcp.Required(), mcp.Description("Contour name"), mcp.Enum(cfg.Contours...)),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			group, err := req.RequireString("group")
			if err != nil {
				return nil, err
			}
			contour, err := req.RequireString("contour")
			if err != nil {
				return nil, err
			}
			return jsonResult(tools.PingStand(ctx, mgr, group, contour, cfg.Defaults.DefaultDatabase))
		},
	)
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}
