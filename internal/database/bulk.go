package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// sqliteBulkChunk is the most IDs bound in one SQLite bulk statement. SQLite
// 3.32 and later allow 32766 host parameters (SQLITE_MAX_VARIABLE_NUMBER);
// older builds allowed 999. 500 stays under both. Tests lower it to check
// chunking.
var sqliteBulkChunk = 500

// BulkUpdateHook, when a test sets it, is called with the SQL of each UPDATE
// a bulk color or tag edit runs, so tests can count statements. It is nil in
// the application.
var BulkUpdateHook func(query string)

// SetSQLiteBulkChunk sets the most IDs per SQLite bulk statement and returns
// a function restoring the previous value. Tests lower it to check chunking.
func SetSQLiteBulkChunk(n int) (restore func()) {
	prev := sqliteBulkChunk
	sqliteBulkChunk = n
	return func() { sqliteBulkChunk = prev }
}

func noteBulkUpdate(query string) {
	if BulkUpdateHook != nil {
		BulkUpdateHook(query)
	}
}

// chunkIDs splits ids into slices of at most n.
func chunkIDs(ids []int64, n int) [][]int64 {
	var out [][]int64
	for len(ids) > n {
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	if len(ids) > 0 {
		out = append(out, ids)
	}
	return out
}

// sqliteIn returns "?,?,..." for n parameters and the ids as arguments,
// after any leading arguments.
func sqliteIn(ids []int64, leading ...interface{}) (string, []interface{}) {
	args := append([]interface{}{}, leading...)
	for _, id := range ids {
		args = append(args, id)
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

// scanBulkRows reads (id, tag, color) rows. tag and color may be NULL.
func scanBulkRows(rows *sql.Rows) ([]BulkEditRow, error) {
	defer rows.Close()
	var out []BulkEditRow
	for rows.Next() {
		var r BulkEditRow
		var tag, color sql.NullString
		if err := rows.Scan(&r.ID, &tag, &color); err != nil {
			return nil, fmt.Errorf("reading updated row: %w", err)
		}
		r.Tag, r.Color = tag.String, color.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// sqliteBulkSetColor sets color on the rows of table whose idCol is in ids,
// one UPDATE per chunk inside tx, and returns the rows as they now are.
func sqliteBulkSetColor(tx *sql.Tx, table, idCol string, ids []int64, color string) ([]BulkEditRow, error) {
	var out []BulkEditRow
	for _, chunk := range chunkIDs(ids, sqliteBulkChunk) {
		in, args := sqliteIn(chunk, color)
		q := "UPDATE " + table + " SET color = ? WHERE " + idCol + " IN (" + in + ")"
		noteBulkUpdate(q)
		if _, err := tx.Exec(q, args...); err != nil {
			return nil, fmt.Errorf("updating color: %w", err)
		}
		in, args = sqliteIn(chunk)
		rows, err := tx.Query("SELECT "+idCol+", tag, color FROM "+table+" WHERE "+idCol+" IN ("+in+")", args...)
		if err != nil {
			return nil, fmt.Errorf("reading updated rows: %w", err)
		}
		got, err := scanBulkRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}
