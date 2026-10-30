package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/importer"
	"github.com/cdtdelta/4n6time/internal/model"
)

// ifOldImportsSQLite and ifOldImportsPostgres are l2t_imports as v0.16.0
// through v0.18.1 created it, before error_message.
const ifOldImportsSQLite = `CREATE TABLE l2t_imports (
	id INTEGER PRIMARY KEY,
	filename TEXT,
	label TEXT,
	format TEXT,
	imported_at TEXT,
	event_count INT DEFAULT 0,
	status TEXT
)`

const ifOldImportsPostgres = `CREATE TABLE l2t_imports (
	id SERIAL PRIMARY KEY,
	filename TEXT,
	label TEXT,
	format TEXT,
	imported_at TIMESTAMP,
	event_count INT DEFAULT 0,
	status TEXT
)`

// ifStore is a store whose raw connection a test can use.
type ifStore interface {
	database.Store
	Conn() *sql.DB
}

// ifBackend is one backend: its name, the old l2t_imports DDL, and a
// constructor for a fresh store.
type ifBackend struct {
	name     string
	oldDDL   string
	newStore func(t *testing.T) ifStore
}

func ifBackends() []ifBackend {
	return []ifBackend{
		{
			name:   "sqlite",
			oldDDL: ifOldImportsSQLite,
			newStore: func(t *testing.T) ifStore {
				store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "if.db"), nil)
				if err != nil {
					t.Fatalf("CreateSQLite: %v", err)
				}
				t.Cleanup(func() { store.Close() })
				return store
			},
		},
		{
			name:   "postgres",
			oldDDL: ifOldImportsPostgres,
			newStore: func(t *testing.T) ifStore {
				store, err := database.CreatePostgres(mxCreatePGDatabase(t), nil)
				if err != nil {
					t.Fatalf("CreatePostgres: %v", err)
				}
				t.Cleanup(func() { store.Close() })
				return store
			},
		},
	}
}

// ifRawError reads l2t_imports.error_message for id, so tests can tell NULL
// from an empty string.
func ifRawError(t *testing.T, store ifStore, id int64) sql.NullString {
	t.Helper()
	var msg sql.NullString
	ph := "?"
	if _, ok := store.(*database.PostgresStore); ok {
		ph = "$1"
	}
	if err := store.Conn().QueryRow("SELECT error_message FROM l2t_imports WHERE id = "+ph, id).Scan(&msg); err != nil {
		t.Fatalf("reading error_message for import %d: %v", id, err)
	}
	return msg
}

func ifImport(t *testing.T, store database.Store, id int64) model.ImportRecord {
	t.Helper()
	rec, err := store.GetImport(id)
	if err != nil {
		t.Fatalf("GetImport(%d): %v", id, err)
	}
	if rec == nil {
		t.Fatalf("GetImport(%d): no record", id)
	}
	return *rec
}

// ifToOldSchema replaces l2t_imports with the pre-v0.19.0 table holding one
// complete row, and returns that row's ID.
func ifToOldSchema(t *testing.T, store ifStore, oldDDL string) int64 {
	t.Helper()
	conn := store.Conn()
	if _, err := conn.Exec("DROP TABLE l2t_imports"); err != nil {
		t.Fatalf("dropping l2t_imports: %v", err)
	}
	if _, err := conn.Exec(oldDDL); err != nil {
		t.Fatalf("creating old l2t_imports: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO l2t_imports (filename, label, format, imported_at, event_count, status)
		VALUES ('old.csv', 'WS1', 'L2T CSV', '2026-01-02 03:04:05', 7, 'complete')`); err != nil {
		t.Fatalf("inserting old row: %v", err)
	}
	var id int64
	if err := conn.QueryRow("SELECT id FROM l2t_imports").Scan(&id); err != nil {
		t.Fatalf("reading old row id: %v", err)
	}
	return id
}

func TestImportErrorColumnMigration(t *testing.T) {
	for _, b := range ifBackends() {
		t.Run(b.name, func(t *testing.T) {
			store := b.newStore(t)
			id := ifToOldSchema(t, store, b.oldDDL)

			// GetImports selects error_message, so it fails before the migration.
			if _, err := store.GetImports(); err == nil {
				t.Fatalf("GetImports on the old schema succeeded; the test did not remove error_message")
			}

			for i := 1; i <= 2; i++ {
				if err := store.Migrate(); err != nil {
					t.Fatalf("Migrate run %d: %v", i, err)
				}
			}

			want := model.ImportRecord{
				ID: id, Filename: "old.csv", Label: "WS1", Format: "L2T CSV",
				ImportedAt: "2026-01-02 03:04:05", EventCount: 7, Status: model.ImportStatusComplete,
			}
			if got := ifImport(t, store, id); got != want {
				t.Errorf("old row after migration = %+v, want %+v", got, want)
			}
			if msg := ifRawError(t, store, id); msg.Valid {
				t.Errorf("old row error_message = %q, want NULL", msg.String)
			}

			// The migrated table takes error text like a new one.
			newID, err := store.CreateImport(model.ImportRecord{Filename: "bad.jsonl", Format: "Plaso JSONL"})
			if err != nil {
				t.Fatalf("CreateImport after migration: %v", err)
			}
			if err := store.FinalizeImport(newID, 0, model.ImportStatusFailed, "boom"); err != nil {
				t.Fatalf("FinalizeImport after migration: %v", err)
			}
			if got := ifImport(t, store, newID).ErrorMessage; got != "boom" {
				t.Errorf("error_message after migration = %q, want boom", got)
			}
		})
	}
}

// failingStore fails the operations whose error fields are set and passes
// everything else to the wrapped store.
type failingStore struct {
	database.Store
	insertErr   error
	finalizeErr error
	metadataErr error
}

func (s *failingStore) InsertEvents(events []*model.Event, onProgress func(int)) (int, error) {
	if s.insertErr != nil {
		return 0, s.insertErr
	}
	return s.Store.InsertEvents(events, onProgress)
}

func (s *failingStore) FinalizeImport(id int64, eventCount int, status, errorMessage string) error {
	if s.finalizeErr != nil {
		return s.finalizeErr
	}
	return s.Store.FinalizeImport(id, eventCount, status, errorMessage)
}

func (s *failingStore) UpdateMetadata() error {
	if s.metadataErr != nil {
		return s.metadataErr
	}
	return s.Store.UpdateMetadata()
}

// ifFormat is a test format that inserts two events through the store it is
// given, as every registry format does.
var ifFormat = importer.Format{
	Name: "Test",
	Import: func(path string, store database.Store, p *importer.Progress) (int, error) {
		events := []*model.Event{
			{Datetime: "2025-01-10 10:00:00", Source: "FILE", Desc: "one"},
			{Datetime: "2025-01-10 10:01:00", Source: "FILE", Desc: "two"},
		}
		n, err := store.InsertEvents(events, nil)
		if err != nil {
			return 0, fmt.Errorf("inserting events: %w", err)
		}
		return n, nil
	},
}

type ifLog struct{ lines []string }

func (l *ifLog) logf(level, msg string) { l.lines = append(l.lines, level+" "+msg) }

func (l *ifLog) has(level, substr string) bool {
	for _, line := range l.lines {
		if strings.HasPrefix(line, level+" ") && strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

func TestFailedImportStoresError(t *testing.T) {
	for _, b := range ifBackends() {
		t.Run(b.name, func(t *testing.T) {
			store := b.newStore(t)
			boom := errors.New("failed to encode args[18]: boom")
			var log ifLog
			res, err := importer.ImportFile("/evidence/case1/bad.jsonl", &ifFormat, "Test", "WS1",
				&failingStore{Store: store, insertErr: boom}, nil, log.logf)
			if !errors.Is(err, boom) {
				t.Fatalf("ImportFile error = %v, want the insert error", err)
			}
			if res.ID == 0 {
				t.Fatalf("ImportFile created no record")
			}

			rec := ifImport(t, store, res.ID)
			if rec.Status != model.ImportStatusFailed || rec.EventCount != 0 {
				t.Errorf("record status/count = %s/%d, want failed/0", rec.Status, rec.EventCount)
			}
			if want := "inserting events: failed to encode args[18]: boom"; rec.ErrorMessage != want {
				t.Errorf("ErrorMessage = %q, want %q", rec.ErrorMessage, want)
			}
			if n, err := store.CountEvents("", nil); err != nil || n != 0 {
				t.Errorf("events after failed import = %d, %v; want 0", n, err)
			}

			// GetImports returns the text too.
			all, err := store.GetImports()
			if err != nil || len(all) != 1 || all[0].ErrorMessage != rec.ErrorMessage {
				t.Errorf("GetImports = %+v, %v; want the one failed record with its error", all, err)
			}

			// The log has the error text, the file, and the import ID at ERROR.
			if !log.has("ERROR", fmt.Sprintf("Import %d failed: file=bad.jsonl", res.ID)) || !log.has("ERROR", "boom") {
				t.Errorf("log lines %q lack the ERROR line with file, import ID, and error text", log.lines)
			}
		})
	}
}

func TestSuccessfulImportLeavesErrorNull(t *testing.T) {
	for _, b := range ifBackends() {
		t.Run(b.name, func(t *testing.T) {
			store := b.newStore(t)
			res, err := importer.ImportFile("good.csv", &ifFormat, "Test", "", store, nil, nil)
			if err != nil {
				t.Fatalf("ImportFile: %v", err)
			}
			rec := ifImport(t, store, res.ID)
			if rec.Status != model.ImportStatusComplete || rec.EventCount != 2 || rec.ErrorMessage != "" {
				t.Errorf("record = %+v, want complete, 2 events, no error", rec)
			}
			if msg := ifRawError(t, store, res.ID); msg.Valid {
				t.Errorf("error_message = %q, want NULL", msg.String)
			}
		})
	}
}

func TestFinalizeImportErrorText(t *testing.T) {
	for _, b := range ifBackends() {
		t.Run(b.name, func(t *testing.T) {
			store := b.newStore(t)
			finalize := func(status, msg string) int64 {
				t.Helper()
				id, err := store.CreateImport(model.ImportRecord{Filename: "f", Format: "Test"})
				if err != nil {
					t.Fatalf("CreateImport: %v", err)
				}
				if err := store.FinalizeImport(id, 0, status, msg); err != nil {
					t.Fatalf("FinalizeImport(%s, %q): %v", status, msg, err)
				}
				return id
			}

			// A complete import never keeps error text.
			if msg := ifRawError(t, store, finalize(model.ImportStatusComplete, "ignored")); msg.Valid {
				t.Errorf("complete import error_message = %q, want NULL", msg.String)
			}
			// A failed import with no text stores NULL, not "".
			if msg := ifRawError(t, store, finalize(model.ImportStatusFailed, "")); msg.Valid {
				t.Errorf("failed import with no text: error_message = %q, want NULL", msg.String)
			}
			// Invalid UTF-8 and NUL bytes are cleaned so PostgreSQL accepts the UPDATE.
			got := ifImport(t, store, finalize(model.ImportStatusFailed, "bad \xff byte\x00 here")).ErrorMessage
			if want := "bad � byte here"; got != want {
				t.Errorf("cleaned error_message = %q, want %q", got, want)
			}
			// Long text is cut.
			got = ifImport(t, store, finalize(model.ImportStatusFailed, strings.Repeat("é", 5000))).ErrorMessage
			if len(got) > 4003 || !strings.HasSuffix(got, "...") || !strings.HasPrefix(got, "éé") {
				t.Errorf("long error_message has %d bytes, want at most 4003 starting with é and ending in ...", len(got))
			}
		})
	}
}

func TestCopyImportRecordsCarriesError(t *testing.T) {
	for _, b := range ifBackends() {
		t.Run(b.name, func(t *testing.T) {
			src := newSQLiteForPush(t, "src.db")
			failed, err := src.CreateImport(model.ImportRecord{Filename: "bad.jsonl", Format: "Plaso JSONL"})
			if err != nil {
				t.Fatal(err)
			}
			if err := src.FinalizeImport(failed, 0, model.ImportStatusFailed, "inserting events: boom"); err != nil {
				t.Fatal(err)
			}
			complete, err := src.CreateImport(model.ImportRecord{Filename: "good.csv", Format: "L2T CSV"})
			if err != nil {
				t.Fatal(err)
			}
			if err := src.FinalizeImport(complete, 3, model.ImportStatusComplete, ""); err != nil {
				t.Fatal(err)
			}

			dst := b.newStore(t)
			idMap, err := copyImportRecords(src, dst, nil)
			if err != nil {
				t.Fatalf("copyImportRecords: %v", err)
			}
			if got := ifImport(t, dst, idMap[failed]).ErrorMessage; got != "inserting events: boom" {
				t.Errorf("copied failed record error_message = %q, want the source text", got)
			}
			if msg := ifRawError(t, dst, idMap[complete]); msg.Valid {
				t.Errorf("copied complete record error_message = %q, want NULL", msg.String)
			}
		})
	}
}

// TestPushTargetWithoutErrorColumn pushes import records into a PostgreSQL
// database whose l2t_imports predates error_message. PushToPostgres opens the
// target with CreateStore, as here, and that adds the column first.
func TestPushTargetWithoutErrorColumn(t *testing.T) {
	connStr := mxCreatePGDatabase(t)
	raw, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("opening connection: %v", err)
	}
	t.Cleanup(func() { raw.Close() })
	for _, ddl := range []string{oldPostgresDDL, ifOldImportsPostgres} {
		if _, err := raw.Exec(ddl); err != nil {
			t.Fatalf("creating old schema: %v", err)
		}
	}

	src := newSQLiteForPush(t, "src.db")
	failed, err := src.CreateImport(model.ImportRecord{Filename: "bad.jsonl", Format: "Plaso JSONL"})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.FinalizeImport(failed, 0, model.ImportStatusFailed, "boom"); err != nil {
		t.Fatal(err)
	}

	dst, err := database.CreateStore("postgres", connStr, nil)
	if err != nil {
		t.Fatalf("CreateStore on the old target: %v", err)
	}
	t.Cleanup(func() { dst.Close() })
	idMap, err := copyImportRecords(src, dst, nil)
	if err != nil {
		t.Fatalf("copyImportRecords into the old target: %v", err)
	}
	if got := ifImport(t, dst, idMap[failed]).ErrorMessage; got != "boom" {
		t.Errorf("error_message on the old target = %q, want boom", got)
	}
}

// ifTLN writes a two-event TLN file and returns its path.
func ifTLN(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "host1.tln")
	body := "Time|Source|Host|User|Description\n1262304000|FILE|WS1|alice|one\n1262304060|FILE|WS1|alice|two\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing TLN fixture: %v", err)
	}
	return path
}

func TestImportCSVFailureStages(t *testing.T) {
	garbage := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(garbage, []byte("not a timeline\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		path        string
		wrap        func(database.Store) database.Store
		stage       string
		eventsAdded int
		inList      bool
		outcome     string
	}{
		{"detect", garbage, nil, importStageDetect, 0, false, "Nothing was added"},
		{"import", "", func(s database.Store) database.Store {
			return &failingStore{Store: s, insertErr: errors.New("boom")}
		}, importStageImport, 0, true, "No events from this file were added"},
		{"import and finalize", "", func(s database.Store) database.Store {
			return &failingStore{Store: s, insertErr: errors.New("boom"), finalizeErr: errors.New("record boom")}
		}, importStageImportUnrecorded, 0, true, "The import failed, and its record (import 1) could not be updated, so it may still show as in progress in the Imports list."},
		{"finalize", "", func(s database.Store) database.Store {
			return &failingStore{Store: s, finalizeErr: errors.New("boom")}
		}, importStageFinalize, 2, true, "All 2 events from this file were added"},
		{"metadata", "", func(s database.Store) database.Store {
			return &failingStore{Store: s, metadataErr: errors.New("boom")}
		}, importStageMetadata, 2, true, "filter panel values could not be updated"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sqlite, err := database.CreateSQLite(filepath.Join(t.TempDir(), "case.db"), nil)
			if err != nil {
				t.Fatalf("CreateSQLite: %v", err)
			}
			t.Cleanup(func() { sqlite.Close() })
			var store database.Store = sqlite
			if c.wrap != nil {
				store = c.wrap(sqlite)
			}
			app := &App{store: store, driver: "sqlite"}
			path := c.path
			if path == "" {
				path = ifTLN(t)
			}

			if _, err := app.ImportCSV(path, ""); err == nil {
				t.Fatalf("ImportCSV succeeded; want a %s failure", c.stage)
			}
			f := app.GetLastImportFailure()
			if f == nil {
				t.Fatalf("GetLastImportFailure = nil after a failed import")
			}
			if f.Stage != c.stage || f.EventsAdded != c.eventsAdded || f.InImportsList != c.inList {
				t.Errorf("failure = %+v, want stage %s, %d events, inImportsList %v", f, c.stage, c.eventsAdded, c.inList)
			}
			if f.File != filepath.Base(path) || f.Error == "" || f.Backend != "sqlite" {
				t.Errorf("failure file/error/backend = %q/%q/%q", f.File, f.Error, f.Backend)
			}
			if !strings.Contains(f.Outcome, c.outcome) {
				t.Errorf("outcome = %q, want it to contain %q", f.Outcome, c.outcome)
			}
			if n, err := sqlite.CountEvents("", nil); err != nil || n != int64(c.eventsAdded) {
				t.Errorf("events in database = %d, %v; want %d, as the outcome says", n, err, c.eventsAdded)
			}

			// A later successful import clears the failure.
			app.store = sqlite
			if _, err := app.ImportCSV(ifTLN(t), ""); err != nil {
				t.Fatalf("follow-up ImportCSV: %v", err)
			}
			if f := app.GetLastImportFailure(); f != nil {
				t.Errorf("GetLastImportFailure after success = %+v, want nil", f)
			}
		})
	}
}

func TestImportFailureOutcomeNewDatabase(t *testing.T) {
	got := importFailureOutcome(ImportFailure{Stage: importStageImport, ImportID: 1, NewDatabase: "/cases/new.db"})
	if !strings.Contains(got, "No events from this file were added") || !strings.Contains(got, "/cases/new.db was created but not opened") {
		t.Errorf("outcome = %q", got)
	}
	got = importFailureOutcome(ImportFailure{Stage: importStageCreateDB, NewDatabase: "/cases/new.db"})
	if strings.Contains(got, "created but not opened") {
		t.Errorf("create-stage outcome mentions the new file: %q", got)
	}
}
