package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
	"github.com/jackc/pgx/v5/stdlib"
)

// PostgresConnectTimeout limits how long opening a PostgreSQL connection may
// take (dial, TLS, and authentication). A connection string with its own
// connect_timeout, or a PGCONNECT_TIMEOUT environment variable, takes
// precedence. Tests shorten it.
var PostgresConnectTimeout = 10 * time.Second

// pgCancelDeadlineDelay is how long a cancelled query waits for the server to
// acknowledge the cancel request before the client drops the connection.
const pgCancelDeadlineDelay = 3 * time.Second

// ConnectTimeoutError reports that a PostgreSQL connection attempt got no
// response within Timeout.
type ConnectTimeoutError struct {
	Timeout time.Duration
	Err     error
}

func (e *ConnectTimeoutError) Error() string {
	return fmt.Sprintf("no response from the PostgreSQL server after %s: %v", e.Timeout, e.Err)
}

func (e *ConnectTimeoutError) Unwrap() error { return e.Err }

// explicitConnectTimeout reports whether the user set a connect timeout:
// connect_timeout in the connection string, or PGCONNECT_TIMEOUT.
func explicitConnectTimeout(connStr string) bool {
	return os.Getenv("PGCONNECT_TIMEOUT") != "" || strings.Contains(connStr, "connect_timeout")
}

// openPGDB builds the *sql.DB for every PostgreSQL path (open, create, push
// target, offset checks). It applies PostgresConnectTimeout unless the user
// set a timeout, and it makes a cancelled context send a cancel request to the
// server. pgx's default only drops the client socket, which leaves a long
// query running on the server until it next writes to the client. It returns
// the connect timeout in effect.
func openPGDB(connStr string) (*sql.DB, time.Duration, error) {
	cfg, err := pgx.ParseConfig(connStr)
	if err != nil {
		return nil, 0, fmt.Errorf("opening database: %w", err)
	}
	if !explicitConnectTimeout(connStr) {
		cfg.ConnectTimeout = PostgresConnectTimeout
	}
	cfg.BuildContextWatcherHandler = func(c *pgconn.PgConn) ctxwatch.Handler {
		return &pgconn.CancelRequestContextWatcherHandler{Conn: c, DeadlineDelay: pgCancelDeadlineDelay}
	}
	return stdlib.OpenDB(*cfg), cfg.ConnectTimeout, nil
}

// pgPing opens the first connection. A cancelled ctx returns ctx.Err(); a
// connect timeout returns *ConnectTimeoutError.
func pgPing(ctx context.Context, conn *sql.DB, timeout time.Duration) error {
	err := conn.PingContext(ctx)
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if timeout > 0 && (pgconn.Timeout(err) || errors.Is(err, context.DeadlineExceeded)) {
		return &ConnectTimeoutError{Timeout: timeout, Err: err}
	}
	return err
}
