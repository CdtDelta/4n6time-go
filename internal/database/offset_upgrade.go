package database

import (
	"database/sql"
	"fmt"
)

// PostgreSQL databases created before v0.18.1 declare log2timeline.offset as
// INT, which cannot hold the byte offsets Plaso reports for files past 2 GB.
// New databases use BIGINT (PostgresDialect.CreateTableSQL). Older ones are
// detected on open and upgraded only when the examiner agrees, because the
// ALTER rewrites the whole event table.

// pgOffsetTypeSQL reads the declared type of log2timeline.offset in the
// connection's current schema. No row means the table does not exist yet.
const pgOffsetTypeSQL = `SELECT data_type FROM information_schema.columns
	WHERE table_schema = current_schema() AND table_name = 'log2timeline' AND column_name = 'offset'`

// pgOffsetIsInteger reports whether log2timeline.offset is a 32-bit integer.
// A missing table is not flagged: schema creation gives it the new DDL.
func pgOffsetIsInteger(conn *sql.DB) (bool, error) {
	var dataType string
	err := conn.QueryRow(pgOffsetTypeSQL).Scan(&dataType)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking offset column type: %w", err)
	}
	return dataType == "integer", nil
}

// pgUpgradeOffset widens log2timeline.offset to BIGINT and refreshes planner
// statistics. Each statement runs on its own, outside an explicit transaction,
// so a failed ANALYZE does not undo a completed ALTER. The ALTER takes an
// exclusive lock and rewrites the table and its indexes.
func pgUpgradeOffset(conn *sql.DB) error {
	if _, err := conn.Exec(`ALTER TABLE log2timeline ALTER COLUMN "offset" TYPE BIGINT`); err != nil {
		return fmt.Errorf("altering offset column to BIGINT: %w", err)
	}
	if _, err := conn.Exec("ANALYZE log2timeline"); err != nil {
		return fmt.Errorf("analyzing log2timeline: %w", err)
	}
	return nil
}

// OffsetNeedsUpgrade reports whether this database's offset column is still INT.
func (db *PostgresStore) OffsetNeedsUpgrade() (bool, error) {
	return pgOffsetIsInteger(db.conn)
}

// UpgradeOffsetColumn widens the offset column to BIGINT. It is safe to call
// on a database that is already upgraded: the ALTER then changes nothing.
func (db *PostgresStore) UpgradeOffsetColumn() error {
	return pgUpgradeOffset(db.conn)
}

// OffsetNeedsUpgrade is always false on SQLite, whose INTEGER is 64-bit.
func (db *SQLiteStore) OffsetNeedsUpgrade() (bool, error) { return false, nil }

// UpgradeOffsetColumn does nothing on SQLite; there is nothing to widen.
func (db *SQLiteStore) UpgradeOffsetColumn() error { return nil }

// PostgresOffsetNeedsUpgrade connects to the PostgreSQL database at connStr
// and reports whether its log2timeline.offset is INT. It does not create or
// migrate anything, so it is safe on an empty database, such as a push target
// that has no schema yet.
func PostgresOffsetNeedsUpgrade(connStr string) (bool, error) {
	conn, err := openPostgresConn(connStr)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	return pgOffsetIsInteger(conn)
}

// UpgradePostgresOffset widens log2timeline.offset to BIGINT in the
// PostgreSQL database at connStr, without opening a store.
func UpgradePostgresOffset(connStr string) error {
	conn, err := openPostgresConn(connStr)
	if err != nil {
		return err
	}
	defer conn.Close()
	return pgUpgradeOffset(conn)
}

// openPostgresConn opens and pings a plain connection with no schema work.
func openPostgresConn(connStr string) (*sql.DB, error) {
	d := &PostgresDialect{}
	conn, err := sql.Open(d.DriverName(), d.DSN(connStr))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return conn, nil
}
