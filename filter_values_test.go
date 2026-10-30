package main

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// fvStore is a store whose raw connection the tests can use to empty the
// metadata tables. Both SQLiteStore and PostgresStore satisfy it.
type fvStore interface {
	database.Store
	Conn() *sql.DB
}

// fvMetadataTables are the l2t_* tables UpdateMetadata fills, as it names them.
var fvMetadataTables = []string{
	"l2t_sourcetypes", "l2t_sources", "l2t_users", "l2t_hosts",
	"l2t_MACBs", "l2t_colors", "l2t_types", "l2t_record_numbers",
}

// fvFixture loads events covering what the value sets must agree on: empty
// strings in every panel field, an evidence row whose source is already
// EXAMINER, examiner notes, a complete import with events, a failed import
// with none, and pre-provenance events (no import). It then runs
// UpdateMetadata, as every import does.
func fvFixture(t *testing.T, store database.Store) (completeID int64) {
	t.Helper()
	completeID, err := store.CreateImport(model.ImportRecord{Filename: "a.csv", Format: "L2T CSV"})
	if err != nil {
		t.Fatalf("CreateImport: %v", err)
	}
	failedID, err := store.CreateImport(model.ImportRecord{Filename: "bad.jsonl", Format: "JSONL"})
	if err != nil {
		t.Fatalf("CreateImport: %v", err)
	}

	events := []*model.Event{
		{Datetime: "2025-01-10 10:00:00", Source: "FILE", SourceType: "NTFS", Type: "Modified", User: "alice", Host: "WS1", ImportID: completeID},
		{Datetime: "2025-01-10 10:01:00", Source: "REG", SourceType: "Registry Key", Type: "Last Written", User: "bob", Host: "WS1", ImportID: completeID},
		{Datetime: "2025-01-10 10:02:00", Source: "FILE", SourceType: "NTFS", Type: "Created", User: "", Host: "WS2", ImportID: completeID},
		{Datetime: "2025-01-10 10:03:00", Source: "EXAMINER", SourceType: "", Type: "", User: "", Host: "", ImportID: completeID},
		{Datetime: "2025-01-10 10:04:00", Source: "evt", SourceType: "Event Log", Type: "a", User: "Alice", Host: "ws1"},
		{Datetime: "2025-01-10 10:05:00", Source: "", SourceType: "", Type: "", User: "", Host: ""},
	}
	if _, err := store.InsertEvents(events, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	if err := store.FinalizeImport(completeID, 4, model.ImportStatusComplete, ""); err != nil {
		t.Fatalf("FinalizeImport: %v", err)
	}
	if err := store.FinalizeImport(failedID, 0, model.ImportStatusFailed, ""); err != nil {
		t.Fatalf("FinalizeImport: %v", err)
	}
	for _, n := range []string{"first note", "second note"} {
		if _, err := store.InsertExaminerNote("2025-01-10 11:00:00", n, "", ""); err != nil {
			t.Fatalf("InsertExaminerNote: %v", err)
		}
	}
	if err := store.UpdateMetadata(); err != nil {
		t.Fatalf("UpdateMetadata: %v", err)
	}
	return completeID
}

// fvLiveUnscoped is what the panel showed before GetFilterValues: one
// GetDistinctValues call per field, keys only, sorted.
func fvLiveUnscoped(t *testing.T, app *App) map[string][]string {
	t.Helper()
	want := map[string][]string{}
	for _, f := range filterPanelFields {
		m, err := app.GetDistinctValues(f)
		if err != nil {
			t.Fatalf("GetDistinctValues(%s): %v", f, err)
		}
		want[f] = fvKeys(m)
	}
	return want
}

// fvLiveScoped is the pre-GetFilterValues scoped panel load.
func fvLiveScoped(t *testing.T, app *App, field, op, value string) map[string][]string {
	t.Helper()
	want := map[string][]string{}
	for _, f := range filterPanelFields {
		m, err := app.GetFilteredDistinctValues(f, field, op, value)
		if err != nil {
			t.Fatalf("GetFilteredDistinctValues(%s, %s %s %s): %v", f, field, op, value, err)
		}
		want[f] = fvKeys(m)
	}
	return want
}

func fvKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fvGet(t *testing.T, app *App, field, op, value string) map[string][]string {
	t.Helper()
	got, err := app.GetFilterValues(field, op, value)
	if err != nil {
		t.Fatalf("GetFilterValues(%q, %q, %q): %v", field, op, value, err)
	}
	return got
}

func fvEqual(t *testing.T, what string, got, want map[string][]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n got  %v\n want %v", what, got, want)
	}
}

func fvClearMetadata(t *testing.T, store fvStore) {
	t.Helper()
	for _, table := range fvMetadataTables {
		if _, err := store.Conn().Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("clearing %s: %v", table, err)
		}
	}
}

// fvRun runs the GetFilterValues checks against one backend. Each check
// that changes data gets its own fresh store from newStore.
func fvRun(t *testing.T, driver string, newStore func(t *testing.T) fvStore) {
	t.Run("unscoped matches live", func(t *testing.T) {
		store := newStore(t)
		completeID := fvFixture(t, store)
		app := &App{store: store, driver: driver}

		got := fvGet(t, app, "", "", "")
		fvEqual(t, "unscoped metadata vs live", got, fvLiveUnscoped(t, app))

		// Spot checks, so equal-but-wrong results cannot pass.
		if want := []string{"EXAMINER", "FILE", "REG", "evt"}; !reflect.DeepEqual(got["source"], want) {
			t.Errorf("source = %v, want %v (EXAMINER once, no empty value)", got["source"], want)
		}
		if want := []string{"Event Log", "Examiner Note", "NTFS", "Registry Key"}; !reflect.DeepEqual(got["sourcetype"], want) {
			t.Errorf("sourcetype = %v, want %v", got["sourcetype"], want)
		}
		if want := []string{fvItoa(completeID)}; !reflect.DeepEqual(got["import_id"], want) {
			t.Errorf("import_id = %v, want %v (complete import only)", got["import_id"], want)
		}
	})

	t.Run("unscoped reads metadata, not log2timeline", func(t *testing.T) {
		store := newStore(t)
		fvFixture(t, store)
		app := &App{store: store, driver: driver}
		// A row added without UpdateMetadata is not in the tables yet.
		extra := &model.Event{Datetime: "2025-01-10 12:00:00", Source: "NEWSRC", Host: "NEWHOST"}
		if _, err := store.InsertEvents([]*model.Event{extra}, nil); err != nil {
			t.Fatalf("InsertEvents: %v", err)
		}
		got := fvGet(t, app, "", "", "")
		for _, v := range got["source"] {
			if v == "NEWSRC" {
				t.Errorf("source values include NEWSRC; GetFilterValues scanned log2timeline")
			}
		}
		if err := store.UpdateMetadata(); err != nil {
			t.Fatalf("UpdateMetadata: %v", err)
		}
		fvEqual(t, "after UpdateMetadata", fvGet(t, app, "", "", ""), fvLiveUnscoped(t, app))
	})

	t.Run("fallback when metadata never built", func(t *testing.T) {
		store := newStore(t)
		fvFixture(t, store)
		app := &App{store: store, driver: driver}
		fvClearMetadata(t, store)
		got := fvGet(t, app, "", "", "")
		fvEqual(t, "fallback vs live", got, fvLiveUnscoped(t, app))
		if len(got["host"]) == 0 {
			t.Errorf("host is empty; the fallback did not engage")
		}
	})

	t.Run("no fallback once metadata is built", func(t *testing.T) {
		store := newStore(t)
		fvFixture(t, store)
		app := &App{store: store, driver: driver}
		// Other tables still have rows, so an empty l2t_hosts means no hosts
		// were recorded; the live scan is not repeated for it.
		if _, err := store.Conn().Exec("DELETE FROM l2t_hosts"); err != nil {
			t.Fatalf("clearing l2t_hosts: %v", err)
		}
		got := fvGet(t, app, "", "", "")
		if len(got["host"]) != 0 {
			t.Errorf("host = %v, want none from the empty l2t_hosts", got["host"])
		}
		if len(got["source"]) == 0 {
			t.Errorf("source is empty; want the l2t_sources values")
		}
	})

	t.Run("empty database", func(t *testing.T) {
		store := newStore(t)
		app := &App{store: store, driver: driver}
		got := fvGet(t, app, "", "", "")
		fvEqual(t, "empty database vs live", got, fvLiveUnscoped(t, app))
		for _, f := range filterPanelFields {
			if v, ok := got[f]; !ok || len(v) != 0 {
				t.Errorf("%s = %v (present %v), want an empty list", f, v, ok)
			}
		}
		// Notes alone add the note entries, as GetDistinctValues does.
		if _, err := store.InsertExaminerNote("2025-01-10 11:00:00", "note", "", ""); err != nil {
			t.Fatalf("InsertExaminerNote: %v", err)
		}
		fvEqual(t, "notes only vs live", fvGet(t, app, "", "", ""), fvLiveUnscoped(t, app))
	})

	t.Run("scoped matches existing scoped queries", func(t *testing.T) {
		store := newStore(t)
		completeID := fvFixture(t, store)
		app := &App{store: store, driver: driver}
		for _, base := range [][3]string{
			{"host", "=", "WS1"},
			{"source", "LIKE", "FI"},
			{"user", "!=", "alice"},
			{"source", "NOT LIKE", "E"},
			{"import_id", "=", fvItoa(completeID)},
			{"host", "=", "no such host"},
		} {
			got := fvGet(t, app, base[0], base[1], base[2])
			fvEqual(t, "scoped "+base[0]+" "+base[1]+" "+base[2], got, fvLiveScoped(t, app, base[0], base[1], base[2]))
		}
	})
}

func fvItoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestGetFilterValuesSQLite(t *testing.T) {
	fvRun(t, "sqlite", func(t *testing.T) fvStore {
		store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "fv.db"), nil)
		if err != nil {
			t.Fatalf("CreateSQLite: %v", err)
		}
		t.Cleanup(func() { store.Close() })
		return store
	})
}

func TestGetFilterValuesPostgres(t *testing.T) {
	fvRun(t, "postgres", func(t *testing.T) fvStore {
		store, err := database.CreatePostgres(mxCreatePGDatabase(t), nil)
		if err != nil {
			t.Fatalf("CreatePostgres: %v", err)
		}
		t.Cleanup(func() { store.Close() })
		return store
	})
}

func TestGetFilterValuesNoDatabase(t *testing.T) {
	if _, err := (&App{}).GetFilterValues("", "", ""); err == nil {
		t.Errorf("GetFilterValues with no database: want an error")
	}
}
