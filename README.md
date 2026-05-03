# mcp-postgres

An MCP server that exposes a PostgreSQL estate — organised as **groups × contours** — as a set of discovery, introspection, and query tools. Connect any MCP client (Claude Desktop, Claude Code, or a custom agent) to browse databases, inspect schemas, and run read-only (or opt-in write) queries across multiple environments without leaving the conversation.

## Features

- **One server, many stands** — a single instance covers all groups (`apple`, `orange`, …) and contours (`one`, `two`, `three`, `four`).
- **Read-only by default** — every connection is locked to `READ ONLY` at the session level; Postgres enforces it, not string matching.
- **Opt-in writes** — mark a group `writable: true` and pass `write: true` in `execute_query`.
- **Per-group hostname overrides** — override individual contour hostnames without touching the template.
- **Structured audit log** — every `execute_query` call emits a JSON line to stdout (`ts`, `group`, `contour`, `database`, `write`, `row_count`, `duration_ms`, `sql_hash`).
- **Lazy connection pools** — one `pgxpool` per `(group, contour, database)` triple, created on first use and bounded in size.
- **Distroless image** — multi-stage Docker build produces a minimal, non-root binary image.

## Tools exposed

| Tool | Description |
|---|---|
| `ping_self` | Server health check |
| `list_groups` | All groups with contours and write mode |
| `list_stands` | Stands for a group; optional reachability probe |
| `list_databases` | Databases on a stand (`pg_database`) |
| `list_schemas` | Schemas in a database (excludes `pg_*`, `information_schema`) |
| `list_tables` | Tables, views, matviews, partitioned tables |
| `describe_table` | Columns, indexes, constraints for a table |
| `execute_query` | Run SQL; read-only by default, rows capped |
| `explain_query` | `EXPLAIN` or `EXPLAIN ANALYZE` |
| `ping` | Latency + server version for a stand |

---

## Quickstart

### Prerequisites

- Go 1.23+
- Docker + Docker Compose (for the container path)
- A `.env` file (copy from `.env.example`)

```sh
cp .env.example .env
# edit .env and set PG_ADMIN_USER and PG_ADMIN_PASSWORD
```

### Run locally

```sh
make build
make run
```

The server listens on `http://localhost:3000`. MCP endpoint: `http://localhost:3000/mcp`.

### Run in Docker

```sh
make docker-up
```

### Run tests

Integration tests spin up a real Postgres via testcontainers — Docker must be running.

```sh
make test
```

Unit-only tests (no Docker needed):

```sh
make test-short
```

---

## Configuration

All configuration lives in a single YAML file. The path is resolved from the `MCP_CONFIG_PATH` environment variable (default: `config/config.yaml`).

`${VAR}` placeholders in the file are expanded from the process environment before parsing. Any unresolved placeholder causes a startup error.

### Full reference

```yaml
# Credentials used for every stand.
admin:
  username: ${PG_ADMIN_USER}
  password: ${PG_ADMIN_PASSWORD}
  port: 5432                      # default: 5432

# Go template for hostnames. Supported placeholders: {group}, {contour}.
hostname_template: "{group}-{contour}-hostname.com"

# Ordered list of valid contour names.
# Used to validate overrides and populate the tool input schema enum.
contours: [one, two, three, four]

groups:
  - name: orange               # required; must be unique

  - name: apple
    overrides:
      # Replace the computed hostname for a specific (group, contour) pair.
      # Only contours listed in `contours` above are valid keys here.
      one: apple-one-pg.hostname.com

  - name: vulcan
    writable: true               # default: false
                                 # Enables write=true in execute_query for this group.

defaults:
  statement_timeout_ms: 30000          # Postgres statement_timeout per connection
  idle_in_transaction_timeout_ms: 30000
  max_rows: 1000                       # Hard cap on rows returned by execute_query
  pool_min_size: 0
  pool_max_size: 4
  connect_timeout_s: 5
  default_database: postgres           # Used by discovery tools when no db is specified
```

### Hostname resolution

For a given `(group, contour)` pair the server resolves the hostname as follows:

1. Look up `group` in config — error if unknown.
2. If `groups[group].overrides[contour]` exists → use it verbatim.
3. Otherwise → substitute `{group}` and `{contour}` in `hostname_template`.

The final DSN is:

```
postgres://<username>:<password>@<host>:<port>/<database>?connect_timeout=<connect_timeout_s>
```

### Environment variables

| Variable | Description |
|---|---|
| `PG_ADMIN_USER` | Admin username (referenced in config as `${PG_ADMIN_USER}`) |
| `PG_ADMIN_PASSWORD` | Admin password |
| `MCP_CONFIG_PATH` | Path to the YAML config file (default: `config/config.yaml`) |
| `MCP_ADDR` | Listen address (default: `:3000`) |
| `LOG_LEVEL` | `info` (default) or `debug` (also enables SQL body in audit log) |

---

## Safety model

- **Read-only by default.** Every pooled connection runs:
  ```sql
  SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;
  SET statement_timeout = <configured>;
  SET idle_in_transaction_session_timeout = <configured>;
  ```
  Postgres enforces this — there is no SQL parsing or allowlist on the server side.

- **Writes are double-gated.** A write only proceeds if _both_ conditions hold:
  1. The group has `writable: true` in config.
  2. The caller passes `write: true` to `execute_query`.
  Writes run inside an explicit transaction; failure always rolls back.

- **Row cap.** `max_rows` is always enforced, including on `RETURNING` payloads.

- **Parameter binding.** The `params` argument to `execute_query` is forwarded as pgx bind parameters (`$1`, `$2`, …). No string interpolation occurs in server code.

- **Identifier quoting.** Schema and table names passed to catalog queries are sanitised via `pgx.Identifier.Sanitize()`.

---

## MCP client configuration

### Claude Desktop (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "postgres": {
      "url": "http://localhost:3000/mcp"
    }
  }
}
```

### Claude Code

```sh
claude mcp add postgres --transport http http://localhost:3000/mcp
```

---

## Project layout

```
├── cmd/mcp-postgres/main.go      entry point; server + healthcheck subcommands
├── internal/
│   ├── config/       Viper loader, structs, env expansion, validation
│   ├── resolver/     ResolveHost / ResolveDSN
│   ├── connections/  lazy pgxpool.Pool manager
│   ├── safety/       AfterConnect hook (read-only, timeouts)
│   ├── audit/        slog JSON audit logger
│   ├── errors/       typed errors
│   ├── tools/        discovery, introspection, query logic
│   └── server/       mcp-go tool registration, /health handler
├── test/             integration tests (testcontainers-go)
├── config/
│   └── config.yaml   sample config mounted into container
├── Dockerfile
└── docker-compose.yml
```

---

## Make targets

| Target | Description |
|---|---|
| `make build` | Compile binary to `./mcp-postgres` |
| `make run` | Build and run locally |
| `make test` | Full integration test suite (requires Docker) |
| `make test-short` | Unit tests only — config and resolver (no Docker) |
| `make vet` | `go vet ./...` |
| `make lint` | `golangci-lint run ./...` |
| `make tidy` | `go mod tidy` |
| `make docker-build` | Build the Docker image |
| `make docker-up` | Build and start via Docker Compose |
| `make docker-down` | Stop and remove containers |
| `make clean` | Remove compiled binary |
