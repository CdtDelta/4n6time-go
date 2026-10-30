package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cdtdelta/4n6time/internal/model"
)

// importedAtLayout is the UTC timestamp format stored in l2t_imports.imported_at.
// PostgreSQL stores the same value in a TIMESTAMP column, so both backends
// return identical strings to callers.
const importedAtLayout = "2006-01-02 15:04:05"

// importIDIndexName names the index on log2timeline.import_id. It follows the
// field_idx convention so RebuildIndexes can drop and recreate it.
const importIDIndexName = "import_id_idx"

// ErrReadOnlyField is wrapped by UpdateEvent when a caller names a column that
// cannot change after insert.
var ErrReadOnlyField = errors.New("is read-only")

// readOnlyFields lists log2timeline columns that are never editable, whatever
// updatableFields says. import_id is write-once: it is set when the event is
// inserted and never rewritten, so provenance cannot be edited through any
// update path. Kept as an explicit guard alongside the allowlist.
var readOnlyFields = map[string]bool{
	"import_id": true,
}

// updatableFields lists the only log2timeline columns UpdateEvent may change:
// the examiner's own annotations. The event detail panel sends tag, color,
// notes, and reportnotes; bookmark is examiner state too (the UI toggles it
// through ToggleBookmark and BulkSetBookmark). Every other column is evidence
// as imported, including import_id, which is write-once provenance.
var updatableFields = map[string]bool{
	"tag":         true,
	"color":       true,
	"notes":       true,
	"reportnotes": true,
	"bookmark":    true,
}

// checkUpdatableField validates one field name from an UpdateEvent map. Both
// stores call it for every field before building SQL, so a rejected field
// never leads to a partial update. A known column outside updatableFields is
// read-only (ErrReadOnlyField); a name that is not a column is invalid.
func checkUpdatableField(field string) error {
	if !isValidField(field) {
		return fmt.Errorf("invalid field name: %s", field)
	}
	if readOnlyFields[field] || !updatableFields[field] {
		return fmt.Errorf("%s %w", field, ErrReadOnlyField)
	}
	return nil
}

// importColumns is the SELECT list for l2t_imports. Scan order matches
// model.ImportRecord and each store's scan helper.
const importColumns = "SELECT id, filename, label, format, imported_at, event_count, status, error_message FROM l2t_imports"

// sqlRunner is satisfied by *sql.DB and *sql.Tx, so the same migration runs
// on open and inside schema creation.
type sqlRunner interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// migrateImportProvenance creates the l2t_imports table, adds the
// log2timeline.import_id column when missing, and indexes it. It also adds
// l2t_imports.error_message (v0.19.0) to an older l2t_imports table. Every
// step is idempotent, so running it on an already-migrated database changes
// nothing. Existing rows keep NULL import_id, which reads back as 0, and NULL
// error_message, which reads back as "".
//
// Schema creation calls this before it creates any index. CREATE TABLE IF NOT
// EXISTS keeps a pre-provenance log2timeline table without import_id, so the
// column has to be added before the index that names it.
func migrateImportProvenance(conn sqlRunner, d Dialect) error {
	if _, err := conn.Exec(d.CreateImportsTableSQL()); err != nil {
		return fmt.Errorf("creating l2t_imports table: %w", err)
	}
	if err := addImportErrorColumn(conn, d); err != nil {
		return err
	}

	var count int
	if err := conn.QueryRow(d.SchemaCheckColumnSQL("log2timeline", "import_id")).Scan(&count); err != nil {
		return fmt.Errorf("checking for import_id column: %w", err)
	}
	if count == 0 {
		if _, err := conn.Exec("ALTER TABLE log2timeline ADD COLUMN import_id INT"); err != nil {
			return fmt.Errorf("adding import_id column: %w", err)
		}
	}

	if _, err := conn.Exec(d.CreateIndexSQL(importIDIndexName, "log2timeline", "import_id")); err != nil {
		return fmt.Errorf("creating import_id index: %w", err)
	}
	return nil
}

// addImportErrorColumn adds l2t_imports.error_message when it is missing. The
// column is nullable with no default, so existing rows read back as no error.
func addImportErrorColumn(conn sqlRunner, d Dialect) error {
	var count int
	if err := conn.QueryRow(d.SchemaCheckColumnSQL("l2t_imports", "error_message")).Scan(&count); err != nil {
		return fmt.Errorf("checking for error_message column: %w", err)
	}
	if count > 0 {
		return nil
	}
	if _, err := conn.Exec(d.AddImportErrorColumnSQL()); err != nil {
		return fmt.Errorf("adding error_message column: %w", err)
	}
	return nil
}

// maxImportErrorLen caps the stored error text, in bytes. Parser errors can
// quote file content, and the Imports list only needs enough to diagnose.
const maxImportErrorLen = 4000

// importErrorArg converts a FinalizeImport error message to its column value.
// Only a failed import keeps its text; anything else, or an empty message,
// becomes SQL NULL. The text is made valid UTF-8 without NUL bytes, because
// PostgreSQL rejects both and a rejected UPDATE would leave the record in
// progress, and it is cut to maxImportErrorLen on a rune boundary.
func importErrorArg(status, msg string) interface{} {
	if status != model.ImportStatusFailed {
		return nil
	}
	msg = strings.ReplaceAll(strings.ToValidUTF8(msg, "\uFFFD"), "\x00", "")
	if len(msg) > maxImportErrorLen {
		cut := maxImportErrorLen
		for cut > 0 && !utf8.RuneStart(msg[cut]) {
			cut--
		}
		msg = msg[:cut] + "..."
	}
	if msg == "" {
		return nil
	}
	return msg
}

// importIDArg converts an in-memory ImportID to its column value. Zero becomes
// SQL NULL so unattributed events are distinguishable from real import IDs.
func importIDArg(id int64) interface{} {
	if id == 0 {
		return nil
	}
	return id
}

// resolveImportedAt returns the timestamp to store for a new import. An empty
// value means "now" in UTC. A supplied value must match importedAtLayout so
// both backends hold the same format.
func resolveImportedAt(s string) (string, error) {
	if s == "" {
		return time.Now().UTC().Format(importedAtLayout), nil
	}
	if _, err := time.Parse(importedAtLayout, s); err != nil {
		return "", fmt.Errorf("invalid imported_at %q (want %s): %w", s, importedAtLayout, err)
	}
	return s, nil
}

// validateFinalStatus rejects any status other than complete or failed. An
// import is only finalized once, so in_progress is not a valid final state.
func validateFinalStatus(status string) error {
	switch status {
	case model.ImportStatusComplete, model.ImportStatusFailed:
		return nil
	default:
		return fmt.Errorf("invalid import status %q (want %q or %q)",
			status, model.ImportStatusComplete, model.ImportStatusFailed)
	}
}

// validateEventCount rejects negative counts before they reach the database.
func validateEventCount(n int) error {
	if n < 0 {
		return fmt.Errorf("invalid event count %d: must not be negative", n)
	}
	return nil
}

// CreateImport records a new import run with status in_progress and returns its ID.
// rec.Status is ignored because every import starts in progress. An empty
// rec.ImportedAt is filled with the current UTC time.
func (db *SQLiteStore) CreateImport(rec model.ImportRecord) (int64, error) {
	importedAt, err := resolveImportedAt(rec.ImportedAt)
	if err != nil {
		return 0, err
	}
	result, err := db.conn.Exec(db.dialect.InsertImportSQL(),
		rec.Filename, rec.Label, rec.Format, importedAt, 0, model.ImportStatusInProgress)
	if err != nil {
		return 0, fmt.Errorf("inserting import record: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting import ID: %w", err)
	}
	return id, nil
}

// FinalizeImport sets the event count, final status (complete or failed), and
// error text on an import. errorMessage is stored only for a failed import;
// it is NULL for a complete one, and for a failed one with no text.
func (db *SQLiteStore) FinalizeImport(id int64, eventCount int, status, errorMessage string) error {
	if err := validateFinalStatus(status); err != nil {
		return err
	}
	if err := validateEventCount(eventCount); err != nil {
		return err
	}
	ph := db.dialect.Placeholder
	result, err := db.conn.Exec(
		"UPDATE l2t_imports SET event_count = "+ph(1)+", status = "+ph(2)+", error_message = "+ph(3)+" WHERE id = "+ph(4),
		eventCount, status, importErrorArg(status, errorMessage), id)
	if err != nil {
		return fmt.Errorf("finalizing import %d: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("finalizing import %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("finalizing import %d: no such import", id)
	}
	return nil
}

// GetImports returns every import record ordered by ID.
func (db *SQLiteStore) GetImports() ([]model.ImportRecord, error) {
	rows, err := db.conn.Query(importColumns + " ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("querying imports: %w", err)
	}
	defer rows.Close()

	var records []model.ImportRecord
	for rows.Next() {
		rec, err := sqliteScanImport(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// GetImport returns one import record, or nil with no error when the ID does not exist.
func (db *SQLiteStore) GetImport(id int64) (*model.ImportRecord, error) {
	row := db.conn.QueryRow(importColumns+" WHERE id = "+db.dialect.Placeholder(1), id)
	rec, err := sqliteScanImport(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

// sqliteScanImport scans one l2t_imports row. Text columns go through
// sql.NullString so NULLs read back as empty strings.
func sqliteScanImport(row rowScanner) (model.ImportRecord, error) {
	var (
		rec                                         model.ImportRecord
		filename, label, format, importedAt, status sql.NullString
		errorMessage                                sql.NullString
		eventCount                                  sql.NullInt64
	)
	err := row.Scan(&rec.ID, &filename, &label, &format, &importedAt, &eventCount, &status, &errorMessage)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return rec, err
		}
		return rec, fmt.Errorf("scanning import row: %w", err)
	}
	rec.Filename = filename.String
	rec.Label = label.String
	rec.Format = format.String
	rec.ImportedAt = importedAt.String
	rec.EventCount = eventCount.Int64
	rec.Status = status.String
	rec.ErrorMessage = errorMessage.String
	return rec, nil
}

// CreateImport records a new import run with status in_progress and returns its ID.
// PostgreSQL text values are passed through pgSanitizeString so null bytes
// cannot reach the database.
func (db *PostgresStore) CreateImport(rec model.ImportRecord) (int64, error) {
	importedAt, err := resolveImportedAt(rec.ImportedAt)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.conn.QueryRow(db.dialect.InsertImportSQL(),
		pgSanitizeString(rec.Filename), pgSanitizeString(rec.Label), pgSanitizeString(rec.Format),
		importedAt, 0, model.ImportStatusInProgress,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("inserting import record: %w", err)
	}
	return id, nil
}

// FinalizeImport sets the event count, final status (complete or failed), and
// error text on an import. errorMessage is stored only for a failed import;
// it is NULL for a complete one, and for a failed one with no text.
func (db *PostgresStore) FinalizeImport(id int64, eventCount int, status, errorMessage string) error {
	if err := validateFinalStatus(status); err != nil {
		return err
	}
	if err := validateEventCount(eventCount); err != nil {
		return err
	}
	ph := db.dialect.Placeholder
	result, err := db.conn.Exec(
		"UPDATE l2t_imports SET event_count = "+ph(1)+", status = "+ph(2)+", error_message = "+ph(3)+" WHERE id = "+ph(4),
		eventCount, status, importErrorArg(status, errorMessage), id)
	if err != nil {
		return fmt.Errorf("finalizing import %d: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("finalizing import %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("finalizing import %d: no such import", id)
	}
	return nil
}

// GetImports returns every import record ordered by ID.
func (db *PostgresStore) GetImports() ([]model.ImportRecord, error) {
	rows, err := db.conn.Query(importColumns + " ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("querying imports: %w", err)
	}
	defer rows.Close()

	var records []model.ImportRecord
	for rows.Next() {
		rec, err := pgScanImport(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// GetImport returns one import record, or nil with no error when the ID does not exist.
func (db *PostgresStore) GetImport(id int64) (*model.ImportRecord, error) {
	row := db.conn.QueryRow(importColumns+" WHERE id = "+db.dialect.Placeholder(1), id)
	rec, err := pgScanImport(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// pgScanImport scans one l2t_imports row. imported_at is a TIMESTAMP, so it is
// scanned as sql.NullTime and formatted to match the SQLite TEXT value.
func pgScanImport(row rowScanner) (model.ImportRecord, error) {
	var (
		rec                             model.ImportRecord
		filename, label, format, status sql.NullString
		errorMessage                    sql.NullString
		importedAt                      sql.NullTime
		eventCount                      sql.NullInt64
	)
	err := row.Scan(&rec.ID, &filename, &label, &format, &importedAt, &eventCount, &status, &errorMessage)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return rec, err
		}
		return rec, fmt.Errorf("scanning import row: %w", err)
	}
	rec.Filename = filename.String
	rec.Label = label.String
	rec.Format = format.String
	if importedAt.Valid {
		rec.ImportedAt = importedAt.Time.UTC().Format(importedAtLayout)
	}
	rec.EventCount = eventCount.Int64
	rec.Status = status.String
	rec.ErrorMessage = errorMessage.String
	return rec, nil
}
