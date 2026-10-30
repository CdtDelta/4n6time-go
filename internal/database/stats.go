package database

import (
	"database/sql"
	"errors"
	"fmt"
)

// StatEventCount is the l2t_stats key holding the number of evidence rows in
// log2timeline. Examiner notes are not counted.
//
// The value changes only inside InsertEvent and InsertEvents, in the same
// transaction as the insert (incrementStat), so a failed or crashed import
// rolls back the rows and the increment together, and concurrent imports on
// PostgreSQL add up under the row lock. Nothing deletes from log2timeline.
//
// When the row is missing (a database from before v0.19.0, or one never
// counted), the app counts live and inserts the row with InsertStatIfMissing.
// An import that commits between that live count and the insert is missed:
// its increment found no row, and the live count ran before its rows were
// visible. Tools > Recount Events (SetStat) corrects it.
//
// A database with no evidence is seeded with event_count = 0 when its stats
// table is created or migrated (seedEventCount), so new databases count from
// their first import without a rebuild. An import into an empty database whose
// insert has not committed when the seed runs is missed in the same way: the
// seed saw no rows, and the import's increment found no row to update.
const StatEventCount = "event_count"

// migrateStats creates l2t_stats when missing. Its error is returned, so on
// read-only storage an older database fails the migration with
// SQLITE_READONLY and gets the read-only upgrade dialog.
func migrateStats(conn sqlRunner, d Dialect) error {
	if _, err := conn.Exec(d.CreateStatsTableSQL()); err != nil {
		return fmt.Errorf("creating l2t_stats table: %w", err)
	}
	return nil
}

// seedEventCount stores event_count = 0 when the row is missing and
// log2timeline has no rows. A database that already has evidence but no row
// is left alone: counting it here would make every open of a large database
// slow, and the first read rebuilds the row instead. The emptiness check reads
// at most one row.
func seedEventCount(conn sqlRunner, d Dialect) error {
	if _, ok, err := getStat(conn, d, StatEventCount); err != nil || ok {
		return err
	}
	var one int
	err := conn.QueryRow("SELECT 1 FROM log2timeline LIMIT 1").Scan(&one)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("checking for events: %w", err)
	}
	return insertStatIfMissing(conn, d, StatEventCount, 0)
}

// getStat reads one l2t_stats value. ok is false when the key has no row.
func getStat(conn sqlRunner, d Dialect, key string) (value int64, ok bool, err error) {
	err = conn.QueryRow("SELECT value FROM l2t_stats WHERE key = "+d.Placeholder(1), key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("reading %s: %w", key, err)
	}
	return value, true, nil
}

// insertStatIfMissing inserts key = value unless the key already has a row,
// which it leaves unchanged.
func insertStatIfMissing(conn sqlRunner, d Dialect, key string, value int64) error {
	_, err := conn.Exec("INSERT INTO l2t_stats (key, value) VALUES ("+d.Placeholder(1)+", "+d.Placeholder(2)+
		") ON CONFLICT (key) DO NOTHING", key, value)
	if err != nil {
		return fmt.Errorf("writing %s: %w", key, err)
	}
	return nil
}

// setStat writes key = value, replacing any existing row.
func setStat(conn sqlRunner, d Dialect, key string, value int64) error {
	_, err := conn.Exec("INSERT INTO l2t_stats (key, value) VALUES ("+d.Placeholder(1)+", "+d.Placeholder(2)+
		") ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value)
	if err != nil {
		return fmt.Errorf("writing %s: %w", key, err)
	}
	return nil
}

// incrementStat adds n to key's value inside the caller's transaction. A
// missing row is left missing; the next read rebuilds it with a live count.
func incrementStat(conn sqlRunner, d Dialect, key string, n int) error {
	if n == 0 {
		return nil
	}
	_, err := conn.Exec("UPDATE l2t_stats SET value = value + "+d.Placeholder(1)+" WHERE key = "+d.Placeholder(2), int64(n), key)
	if err != nil {
		return fmt.Errorf("updating %s: %w", key, err)
	}
	return nil
}

// GetStat returns one cached statistic; ok is false when it has no row.
func (db *SQLiteStore) GetStat(key string) (int64, bool, error) {
	return getStat(db.conn, db.dialect, key)
}

// InsertStatIfMissing stores a statistic unless it already has a row.
func (db *SQLiteStore) InsertStatIfMissing(key string, value int64) error {
	return insertStatIfMissing(db.conn, db.dialect, key, value)
}

// SetStat stores a statistic, replacing any existing value.
func (db *SQLiteStore) SetStat(key string, value int64) error {
	return setStat(db.conn, db.dialect, key, value)
}

// GetStat returns one cached statistic; ok is false when it has no row.
func (db *PostgresStore) GetStat(key string) (int64, bool, error) {
	return getStat(db.conn, db.dialect, key)
}

// InsertStatIfMissing stores a statistic unless it already has a row.
func (db *PostgresStore) InsertStatIfMissing(key string, value int64) error {
	return insertStatIfMissing(db.conn, db.dialect, key, value)
}

// SetStat stores a statistic, replacing any existing value.
func (db *PostgresStore) SetStat(key string, value int64) error {
	return setStat(db.conn, db.dialect, key, value)
}
