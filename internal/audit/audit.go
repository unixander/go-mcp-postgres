package audit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"time"
)

type Entry struct {
	Group       string
	Contour     string
	Database    string
	Write       bool
	RowCount    int
	DurationMs  int64
	SQLHash     string
	SQL         string // only logged when includeSQLBody is true
}

var includeSQLBody bool

func SetIncludeSQLBody(v bool) { includeSQLBody = v }

func Log(ctx context.Context, e Entry) {
	args := []any{
		"group", e.Group,
		"contour", e.Contour,
		"database", e.Database,
		"write", e.Write,
		"row_count", e.RowCount,
		"duration_ms", e.DurationMs,
		"sql_hash", e.SQLHash,
	}
	if includeSQLBody {
		args = append(args, "sql", e.SQL)
	}
	slog.InfoContext(ctx, "execute_query", args...)
}

func HashSQL(sql string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))[:16]
}

func Now() time.Time { return time.Now() }
