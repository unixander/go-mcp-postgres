package safety

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// SessionConfig holds per-session guard settings.
type SessionConfig struct {
	StatementTimeoutMs         int
	IdleInTransactionTimeoutMs int
}

// AfterConnect returns a pgx AfterConnect callback that enforces read-only mode
// and statement timeouts for every pooled connection.
func AfterConnect(sc SessionConfig) func(context.Context, *pgx.Conn) error {
	return func(ctx context.Context, conn *pgx.Conn) error {
		sql := fmt.Sprintf(`
SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;
SET statement_timeout = %d;
SET idle_in_transaction_session_timeout = %d;`,
			sc.StatementTimeoutMs,
			sc.IdleInTransactionTimeoutMs,
		)
		_, err := conn.Exec(ctx, sql)
		return err
	}
}
