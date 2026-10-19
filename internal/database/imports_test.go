package database

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/query"
)

// legacySchemaSQL is the log2timeline table as it existed before import
// provenance: no import_id column, and no l2t_imports table elsewhere in the file.
const legacySchemaSQL = `CREATE TABLE log2timeline (
	timezone TEXT, MACB TEXT, source TEXT, sourcetype TEXT,
	type TEXT, user TEXT, host TEXT, desc TEXT, filename TEXT,
	inode TEXT, notes TEXT, format TEXT, extra TEXT,
	datetime DATETIME, reportnotes TEXT, inreport TEXT,
	tag TEXT, color TEXT, offset INT, store_number INT,
	store_index INT, vss_store_number INT, URL TEXT,
	record_number TEXT, event_identifier TEXT, event_type TEXT,
	source_name TEXT, user_sid TEXT, computer_name TEXT,
	bookmark INT DEFAULT 0
)`

// buildAllEvents runs the query builder with no predicates, which produces the
// Pattern B SELECT (model.Fields order) that ExecuteQuery expects.
func buildAllEvents(t *testing.T, db interface {
	ExecuteQuery(string, []interface{}, *NotesFilter) ([]*model.Event, error)
}) []*model.Event {
	t.Helper()
	q := query.New(0)
	sqlStr, args := q.Build()
	events, err := db.ExecuteQuery(sqlStr, args, nil)
	if err != nil {
		t.Fatalf("ExecuteQuery failed: %v", err)
	}
	return events
}

func TestMigrationFromPreProvenanceSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.Exec(legacySchemaSQL); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	// Seed every column as a real pre-0.16 writer would: empty strings for text,
	// zero for integers. The SQLite scans read text columns into plain strings,
	// so NULLs here would fail for reasons unrelated to migration.
	if _, err := raw.Exec(`INSERT INTO log2timeline (
		timezone, MACB, source, sourcetype, type, user, host, desc, filename,
		inode, notes, format, extra, datetime, reportnotes, inreport, tag, color,
		offset, store_number, store_index, vss_store_number, URL, record_number,
		event_identifier, event_type, source_name, user_sid, computer_name, bookmark
	) VALUES (
		'', '', 'FILE', '', '', '', '', 'legacy row', '',
		'', '', '', '', '2025-01-01 00:00:00', '', '', '', '',
		0, 0, 0, 0, '', '',
		'', '', '', '', '', 0
	)`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	raw.Close()

	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite on legacy db failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var colCount int
	if err := db.conn.QueryRow(db.dialect.SchemaCheckColumnSQL("log2timeline", "import_id")).Scan(&colCount); err != nil {
		t.Fatalf("checking column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("import_id column count = %d, want 1", colCount)
	}

	var tableCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'l2t_imports'`).Scan(&tableCount); err != nil {
		t.Fatalf("checking table: %v", err)
	}
	if tableCount != 1 {
		t.Errorf("l2t_imports table count = %d, want 1", tableCount)
	}

	var indexCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'import_id_idx'`).Scan(&indexCount); err != nil {
		t.Fatalf("checking index: %v", err)
	}
	if indexCount != 1 {
		t.Errorf("import_id_idx count = %d, want 1", indexCount)
	}

	// The pre-existing row must scan with ImportID 0 through both patterns.
	events, err := db.QueryEvents("", nil, "", 0, 0)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].ImportID != 0 || events[0].Desc != "legacy row" {
		t.Fatalf("Pattern A legacy row = %+v, want one row with ImportID 0", events)
	}
	events = buildAllEvents(t, db)
	if len(events) != 1 || events[0].ImportID != 0 {
		t.Fatalf("Pattern B legacy row = %+v, want one row with ImportID 0", events)
	}

	// Migrate must be idempotent: a second run changes nothing and does not fail.
	if err := db.Migrate(); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}
	if err := db.conn.QueryRow(db.dialect.SchemaCheckColumnSQL("log2timeline", "import_id")).Scan(&colCount); err != nil || colCount != 1 {
		t.Fatalf("after second Migrate, import_id count = %d (err %v), want 1", colCount, err)
	}
}

func TestNewDatabaseHasImportProvenance(t *testing.T) {
	db := createTestDB(t)

	var indexCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'import_id_idx'`).Scan(&indexCount); err != nil {
		t.Fatalf("checking index: %v", err)
	}
	if indexCount != 1 {
		t.Errorf("import_id_idx count = %d, want 1", indexCount)
	}
	if _, err := db.GetImports(); err != nil {
		t.Errorf("GetImports on new db failed: %v", err)
	}
}

func TestImportIDRoundTrip(t *testing.T) {
	db := createTestDB(t)

	zero := sampleEvent()
	zero.Desc = "no import"
	withImport := sampleEvent()
	withImport.Desc = "with import"
	withImport.ImportID = 7

	if err := db.InsertEvent(zero); err != nil {
		t.Fatalf("InsertEvent (ImportID 0) failed: %v", err)
	}
	if _, err := db.InsertEvents([]*model.Event{withImport}, nil); err != nil {
		t.Fatalf("InsertEvents (ImportID 7) failed: %v", err)
	}

	// Zero is stored as SQL NULL, not as 0.
	var nullCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM log2timeline WHERE import_id IS NULL`).Scan(&nullCount); err != nil {
		t.Fatalf("counting NULL import_id: %v", err)
	}
	if nullCount != 1 {
		t.Errorf("NULL import_id rows = %d, want 1", nullCount)
	}

	// Pattern A.
	events, err := db.QueryEvents("", nil, "rowid", 0, 0)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("QueryEvents returned %d rows, want 2", len(events))
	}
	if events[0].ImportID != 0 || events[1].ImportID != 7 {
		t.Errorf("Pattern A ImportIDs = %d, %d; want 0, 7", events[0].ImportID, events[1].ImportID)
	}

	// Pattern B.
	events = buildAllEvents(t, db)
	byDesc := map[string]int64{}
	for _, e := range events {
		byDesc[e.Desc] = e.ImportID
	}
	if got, ok := byDesc["no import"]; !ok || got != 0 {
		t.Errorf("Pattern B ImportID for unattributed event = %d (present %v), want 0", got, ok)
	}
	if got := byDesc["with import"]; got != 7 {
		t.Errorf("Pattern B ImportID for attributed event = %d, want 7", got)
	}
}

func TestImportRecordRoundTrip(t *testing.T) {
	db := createTestDB(t)

	id1, err := db.CreateImport(model.ImportRecord{Filename: "plaso.jsonl", Label: "WS1", Format: "Plaso JSONL"})
	if err != nil {
		t.Fatalf("CreateImport failed: %v", err)
	}
	if id1 <= 0 {
		t.Fatalf("CreateImport returned id %d, want positive", id1)
	}

	rec, err := db.GetImport(id1)
	if err != nil || rec == nil {
		t.Fatalf("GetImport(%d) = %v, %v; want record", id1, rec, err)
	}
	if rec.Filename != "plaso.jsonl" || rec.Label != "WS1" || rec.Format != "Plaso JSONL" {
		t.Errorf("GetImport fields = %+v", rec)
	}
	if rec.Status != model.ImportStatusInProgress {
		t.Errorf("new import status = %q, want %q", rec.Status, model.ImportStatusInProgress)
	}
	if rec.EventCount != 0 {
		t.Errorf("new import event count = %d, want 0", rec.EventCount)
	}
	if _, err := time.Parse(importedAtLayout, rec.ImportedAt); err != nil {
		t.Errorf("empty ImportedAt not filled with layout %s: %q (%v)", importedAtLayout, rec.ImportedAt, err)
	}

	if err := db.FinalizeImport(id1, 42, model.ImportStatusComplete); err != nil {
		t.Fatalf("FinalizeImport failed: %v", err)
	}
	rec, err = db.GetImport(id1)
	if err != nil || rec == nil {
		t.Fatalf("GetImport after finalize = %v, %v", rec, err)
	}
	if rec.EventCount != 42 || rec.Status != model.ImportStatusComplete {
		t.Errorf("finalized import = count %d status %q, want 42 complete", rec.EventCount, rec.Status)
	}

	// An explicit timestamp is stored exactly as given.
	id2, err := db.CreateImport(model.ImportRecord{
		Filename: "broken.csv", Format: "EZ Tools CSV", ImportedAt: "2026-01-02 03:04:05", Status: "complete",
	})
	if err != nil {
		t.Fatalf("CreateImport with ImportedAt failed: %v", err)
	}
	if pending, err := db.GetImport(id2); err != nil || pending == nil || pending.Status != model.ImportStatusInProgress {
		t.Fatalf("CreateImport(Status: complete) stored %+v (err %v), want in_progress", pending, err)
	}
	if err := db.FinalizeImport(id2, 0, model.ImportStatusFailed); err != nil {
		t.Fatalf("FinalizeImport failed: %v", err)
	}

	list, err := db.GetImports()
	if err != nil {
		t.Fatalf("GetImports failed: %v", err)
	}
	if len(list) != 2 || list[0].ID != id1 || list[1].ID != id2 {
		t.Fatalf("GetImports = %+v, want two records ordered by id", list)
	}
	if list[1].ImportedAt != "2026-01-02 03:04:05" {
		t.Errorf("explicit ImportedAt = %q, want 2026-01-02 03:04:05", list[1].ImportedAt)
	}
	if list[1].Status != model.ImportStatusFailed {
		t.Errorf("failed import status = %q, want failed", list[1].Status)
	}
}

func TestImportMissingAndInvalid(t *testing.T) {
	db := createTestDB(t)

	rec, err := db.GetImport(9999)
	if err != nil || rec != nil {
		t.Errorf("GetImport(missing) = %v, %v; want nil, nil", rec, err)
	}

	if err := db.FinalizeImport(9999, 1, model.ImportStatusComplete); err == nil {
		t.Error("FinalizeImport on missing id returned nil error")
	}

	id, err := db.CreateImport(model.ImportRecord{Filename: "x.csv"})
	if err != nil {
		t.Fatalf("CreateImport failed: %v", err)
	}
	if err := db.FinalizeImport(id, 1, model.ImportStatusInProgress); err == nil {
		t.Error("FinalizeImport accepted in_progress as a final status")
	}
	if err := db.FinalizeImport(id, 1, "bogus"); err == nil {
		t.Error("FinalizeImport accepted an unknown status")
	}
	if err := db.FinalizeImport(id, -1, model.ImportStatusComplete); err == nil {
		t.Error("FinalizeImport accepted a negative event count")
	}
	if _, err := db.CreateImport(model.ImportRecord{Filename: "x.csv", ImportedAt: "yesterday"}); err == nil {
		t.Error("CreateImport accepted a malformed ImportedAt")
	}
}

func TestExaminerNotesUnionCarriesImportID(t *testing.T) {
	db := createTestDB(t)

	attributed := sampleEvent()
	attributed.Desc = "attributed"
	attributed.ImportID = 3
	if _, err := db.InsertEvents([]*model.Event{attributed}, nil); err != nil {
		t.Fatalf("InsertEvents failed: %v", err)
	}
	if _, err := db.InsertExaminerNote("2025-01-20 10:00:00", "examiner note", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote failed: %v", err)
	}

	// No filter: both sides of the UNION must align without a column-count error.
	events := buildAllEvents(t, db)
	if len(events) != 2 {
		t.Fatalf("UNION returned %d rows, want 2", len(events))
	}
	var sawNote bool
	for _, e := range events {
		if e.ID < 0 {
			sawNote = true
			if e.ImportID != 0 {
				t.Errorf("examiner note ImportID = %d, want 0", e.ImportID)
			}
		} else if e.ImportID != 3 {
			t.Errorf("evidence event ImportID = %d, want 3", e.ImportID)
		}
	}
	if !sawNote {
		t.Error("UNION did not include the examiner note")
	}

	// With a notes filter, the filtered side must still align.
	q := query.New(0)
	sqlStr, args := q.Build()
	filtered, err := db.ExecuteQuery(sqlStr, args, &NotesFilter{SQL: "bookmark = 0"})
	if err != nil {
		t.Fatalf("filtered ExecuteQuery failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("filtered UNION returned %d rows, want 2", len(filtered))
	}

	count, err := db.ExecuteCountQuery("SELECT COUNT(rowid) FROM log2timeline", nil, nil)
	if err != nil || count != 2 {
		t.Errorf("ExecuteCountQuery = %d, %v; want 2", count, err)
	}
}

func TestRebuildIndexesKeepsImportIDIndex(t *testing.T) {
	db := createTestDB(t)

	if err := db.RebuildIndexes([]string{"host"}); err != nil {
		t.Fatalf("RebuildIndexes failed: %v", err)
	}
	var indexCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'import_id_idx'`).Scan(&indexCount); err != nil {
		t.Fatalf("checking index: %v", err)
	}
	if indexCount != 1 {
		t.Errorf("import_id_idx count after RebuildIndexes = %d, want 1", indexCount)
	}
}

func TestValidateFinalStatus(t *testing.T) {
	for _, s := range []string{model.ImportStatusComplete, model.ImportStatusFailed} {
		if err := validateFinalStatus(s); err != nil {
			t.Errorf("validateFinalStatus(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range []string{"", model.ImportStatusInProgress, "COMPLETE"} {
		if err := validateFinalStatus(s); err == nil {
			t.Errorf("validateFinalStatus(%q) = nil, want error", s)
		}
	}
}

func TestResolveImportedAt(t *testing.T) {
	got, err := resolveImportedAt("")
	if err != nil {
		t.Fatalf("resolveImportedAt(\"\") failed: %v", err)
	}
	if _, err := time.Parse(importedAtLayout, got); err != nil || strings.Contains(got, "T") {
		t.Errorf("resolveImportedAt(\"\") = %q, want %s format", got, importedAtLayout)
	}
	if _, err := resolveImportedAt("2026-02-03T04:05:06Z"); err == nil {
		t.Error("resolveImportedAt accepted RFC 3339 input; want the storage layout only")
	}
}

func TestImportIDArg(t *testing.T) {
	if v := importIDArg(0); v != nil {
		t.Errorf("importIDArg(0) = %v, want nil", v)
	}
	if v := importIDArg(5); v != int64(5) {
		t.Errorf("importIDArg(5) = %v, want 5", v)
	}
}

func TestUpdateEventRejectsImportID(t *testing.T) {
	db := createTestDB(t)

	ev := sampleEvent()
	ev.ImportID = 5
	if err := db.InsertEvent(ev); err != nil {
		t.Fatalf("InsertEvent failed: %v", err)
	}
	events, err := db.QueryEvents("", nil, "", 0, 0)
	if err != nil || len(events) != 1 {
		t.Fatalf("QueryEvents = %d rows, err %v; want 1", len(events), err)
	}
	id := events[0].ID

	importIDOf := func() int64 {
		t.Helper()
		var v sql.NullInt64
		if err := db.conn.QueryRow(`SELECT import_id FROM log2timeline WHERE rowid = ?`, id).Scan(&v); err != nil {
			t.Fatalf("reading import_id: %v", err)
		}
		return v.Int64
	}

	// Alone, import_id is refused and the stored value is unchanged.
	err = db.UpdateEvent(id, map[string]interface{}{"import_id": int64(9)})
	if !errors.Is(err, ErrReadOnlyField) {
		t.Fatalf("UpdateEvent(import_id) error = %v, want ErrReadOnlyField", err)
	}
	if got := importIDOf(); got != 5 {
		t.Errorf("import_id after rejected update = %d, want 5", got)
	}

	// Mixed with an editable field, nothing is written: the check runs before SQL is built.
	err = db.UpdateEvent(id, map[string]interface{}{"tag": "should-not-stick", "import_id": int64(9)})
	if !errors.Is(err, ErrReadOnlyField) {
		t.Fatalf("mixed UpdateEvent error = %v, want ErrReadOnlyField", err)
	}
	var tag string
	if err := db.conn.QueryRow(`SELECT tag FROM log2timeline WHERE rowid = ?`, id).Scan(&tag); err != nil {
		t.Fatalf("reading tag: %v", err)
	}
	if tag != "" {
		t.Errorf("tag = %q after rejected mixed update, want unchanged empty string", tag)
	}
	if got := importIDOf(); got != 5 {
		t.Errorf("import_id after rejected mixed update = %d, want 5", got)
	}

	// Unknown fields are still rejected, and not as read-only.
	err = db.UpdateEvent(id, map[string]interface{}{"no_such_column": "x"})
	if err == nil || errors.Is(err, ErrReadOnlyField) {
		t.Errorf("UpdateEvent(unknown) error = %v, want invalid field error", err)
	}

	// Editable fields keep working.
	err = db.UpdateEvent(id, map[string]interface{}{
		"tag": "suspicious", "color": "red", "notes": "examiner note",
		"reportnotes": "report text", "bookmark": int64(1),
	})
	if err != nil {
		t.Fatalf("UpdateEvent on editable fields failed: %v", err)
	}
	var color, notes, reportNotes string
	var bookmark int64
	if err := db.conn.QueryRow(
		`SELECT tag, color, notes, reportnotes, bookmark FROM log2timeline WHERE rowid = ?`, id,
	).Scan(&tag, &color, &notes, &reportNotes, &bookmark); err != nil {
		t.Fatalf("reading editable fields: %v", err)
	}
	if tag != "suspicious" || color != "red" || notes != "examiner note" || reportNotes != "report text" || bookmark != 1 {
		t.Errorf("editable fields = tag %q color %q notes %q reportnotes %q bookmark %d",
			tag, color, notes, reportNotes, bookmark)
	}
	if got := importIDOf(); got != 5 {
		t.Errorf("import_id after editable update = %d, want 5", got)
	}
}

// assertImportProvenance checks the three provenance objects that schema
// creation must leave behind: the import_id column, l2t_imports, and the index.
func assertImportProvenance(t *testing.T, db *SQLiteStore) {
	t.Helper()
	var colCount int
	if err := db.conn.QueryRow(db.dialect.SchemaCheckColumnSQL("log2timeline", "import_id")).Scan(&colCount); err != nil {
		t.Fatalf("checking import_id column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("import_id column count = %d, want 1", colCount)
	}
	var tableCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'l2t_imports'`).Scan(&tableCount); err != nil {
		t.Fatalf("checking l2t_imports: %v", err)
	}
	if tableCount != 1 {
		t.Errorf("l2t_imports table count = %d, want 1", tableCount)
	}
	var indexCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'import_id_idx'`).Scan(&indexCount); err != nil {
		t.Fatalf("checking import_id_idx: %v", err)
	}
	if indexCount != 1 {
		t.Errorf("import_id_idx count = %d, want 1", indexCount)
	}
}

// TestCreateSQLiteOnPreProvenanceTable runs the schema creation path against a
// file that already has a pre-0.16 log2timeline table. CREATE TABLE IF NOT
// EXISTS leaves that table alone, so the migration step inside schema creation
// has to add import_id before the index that names it.
func TestCreateSQLiteOnPreProvenanceTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-create.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.Exec(legacySchemaSQL); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	raw.Close()

	db, err := CreateSQLite(path, nil)
	if err != nil {
		t.Fatalf("CreateSQLite on pre-provenance table failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	assertImportProvenance(t, db)
}

// TestCreateSQLiteEmptyHasFullSchema checks that schema creation on a new,
// empty database produces the complete provenance schema.
func TestCreateSQLiteEmptyHasFullSchema(t *testing.T) {
	db := createTestDB(t)
	assertImportProvenance(t, db)
}
