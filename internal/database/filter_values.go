package database

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/cdtdelta/4n6time/internal/model"
)

// importIDFilterField is the filter panel's import field. It has no l2t_*
// table; its values come from l2t_imports.
const importIDFilterField = "import_id"

// filterValuesFromMetadata returns the unscoped filter panel values for fields
// without scanning log2timeline. It is shared by both stores, which pass their
// own live per-field query (GetDistinctValues) for the fallback and their own
// GetImports.
//
// Sources, per field:
//   - metadata fields (source, sourcetype, host, ...): the l2t_<field>s table
//     that UpdateMetadata fills after every import and push. Like the live
//     query, it holds no empty strings (and no NULLs).
//   - import_id: the IDs of complete imports with at least one event.
//   - source and sourcetype also get EXAMINER and Examiner Note when
//     examiner_notes has rows, as GetDistinctValues adds them.
//
// Fallback: when no metadata table has any row but log2timeline does, the
// metadata was never built (a database from the original Python 4n6time or
// another tool), so an empty table says nothing, and the field is read with
// live. Once any metadata table has rows, UpdateMetadata has run, and an empty
// table means the field has no non-empty values, which is what live would
// return after a full scan.
//
// Each field is read independently. A field that fails is left out of the
// result and its error is joined into the returned error, so one bad field
// does not hide the others; the panel did the same with one call per field.
func filterValuesFromMetadata(conn *sql.DB, d Dialect, fields []string,
	live func(field string) (map[string]int64, error),
	imports func() ([]model.ImportRecord, error)) (map[string][]string, error) {

	built, err := metadataBuilt(conn)
	if err != nil {
		return nil, err
	}
	hasEvents := false
	if !built {
		if hasEvents, err = tableHasRows(conn, "log2timeline"); err != nil {
			return nil, err
		}
	}

	result := make(map[string][]string, len(fields))
	var errs []error
	for _, f := range fields {
		var values []string
		var ferr error
		switch {
		case f == importIDFilterField:
			values, ferr = completedImportIDs(imports)
		case isMetadataField(f):
			values, ferr = metadataValues(conn, d, f)
			if ferr == nil && len(values) == 0 && hasEvents {
				values, ferr = liveValues(live, f)
			} else if ferr == nil {
				values = addExaminerValues(conn, f, values)
			}
		default:
			ferr = fmt.Errorf("invalid filter field: %s", f)
		}
		if ferr != nil {
			errs = append(errs, fmt.Errorf("loading %s values: %w", f, ferr))
			continue
		}
		result[f] = sortedUnique(values)
	}
	return result, errors.Join(errs...)
}

// isMetadataField reports whether f has an l2t_<f>s table.
func isMetadataField(f string) bool {
	for _, m := range metadataFields {
		if m == f {
			return true
		}
	}
	return false
}

// metadataTable returns the l2t_* table name for a metadata field. Names are
// written unquoted, as UpdateMetadata writes them, so PostgreSQL folds them
// the same way (l2t_MACBs becomes l2t_macbs in both places).
func metadataTable(f string) string { return "l2t_" + f + "s" }

// metadataBuilt reports whether any metadata table has a row.
func metadataBuilt(conn *sql.DB) (bool, error) {
	for _, f := range metadataFields {
		ok, err := tableHasRows(conn, metadataTable(f))
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// tableHasRows reports whether table has at least one row. It reads one row,
// so it costs the same on 77M rows as on ten. table is a fixed internal name.
func tableHasRows(conn *sql.DB, table string) (bool, error) {
	var one int
	err := conn.QueryRow("SELECT 1 FROM " + table + " LIMIT 1").Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking %s for rows: %w", table, err)
	}
	return true, nil
}

// metadataValues reads the non-empty values stored for f. UpdateMetadata
// already excludes empty strings; NULLs and empties are skipped here too, as
// GetDistinctValues skips them.
func metadataValues(conn *sql.DB, d Dialect, f string) ([]string, error) {
	rows, err := conn.Query("SELECT " + d.QuoteColumn(f) + " FROM " + metadataTable(f))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", metadataTable(f), err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scanning %s: %w", metadataTable(f), err)
		}
		if v.Valid && v.String != "" {
			values = append(values, v.String)
		}
	}
	return values, rows.Err()
}

// liveValues runs the store's live per-field query and keeps the values.
func liveValues(live func(string) (map[string]int64, error), f string) ([]string, error) {
	m, err := live(f)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(m))
	for v := range m {
		values = append(values, v)
	}
	return values, nil
}

// addExaminerValues adds the examiner-note entries GetDistinctValues adds: the
// note source and sourcetype, when at least one note exists. A failed count
// adds nothing and is not an error, matching GetDistinctValues.
func addExaminerValues(conn *sql.DB, f string, values []string) []string {
	var entry string
	switch f {
	case "source":
		entry = "EXAMINER"
	case "sourcetype":
		entry = "Examiner Note"
	default:
		return values
	}
	var cnt int64
	if err := conn.QueryRow("SELECT COUNT(*) FROM examiner_notes").Scan(&cnt); err == nil && cnt > 0 {
		values = append(values, entry)
	}
	return values
}

// completedImportIDs returns the IDs of complete imports with events, as text,
// matching the values GROUP BY import_id produces: every event's import_id
// points at a complete record, because a failed import rolls its rows back.
func completedImportIDs(imports func() ([]model.ImportRecord, error)) ([]string, error) {
	recs, err := imports()
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, r := range recs {
		if r.Status == model.ImportStatusComplete && r.EventCount > 0 {
			ids = append(ids, strconv.FormatInt(r.ID, 10))
		}
	}
	return ids, nil
}

// sortedUnique sorts values and drops duplicates (EXAMINER can also be an
// evidence source). The panel sorts again for display; this keeps results
// stable for callers and tests.
func sortedUnique(values []string) []string {
	sort.Strings(values)
	out := values[:0]
	for i, v := range values {
		if i == 0 || v != values[i-1] {
			out = append(out, v)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// GetFilterValues returns the unscoped filter panel values for fields from the
// metadata tables (see filterValuesFromMetadata).
func (db *SQLiteStore) GetFilterValues(fields []string) (map[string][]string, error) {
	return filterValuesFromMetadata(db.conn, db.dialect, fields, db.GetDistinctValues, db.GetImports)
}

// GetFilterValues returns the unscoped filter panel values for fields from the
// metadata tables (see filterValuesFromMetadata).
func (db *PostgresStore) GetFilterValues(fields []string) (map[string][]string, error) {
	return filterValuesFromMetadata(db.conn, db.dialect, fields, db.GetDistinctValues, db.GetImports)
}
