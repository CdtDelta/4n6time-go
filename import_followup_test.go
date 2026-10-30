package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// roOldDatabase creates a pre-v0.19.0 SQLite database (old l2t_imports) in
// its own folder, makes the file 444 and the folder 555, and returns its path
// and contents. It skips where permissions do not apply.
func roOldDatabase(t *testing.T) (string, []byte) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("file permission tests do not apply on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not apply, so read-only storage cannot be simulated")
	}

	dir := filepath.Join(t.TempDir(), "media")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "old.db")
	store, err := database.CreateSQLite(path, nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	for _, stmt := range []string{"DROP TABLE l2t_imports", ifOldImportsSQLite} {
		if _, err := store.Conn().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	store.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(dir, 0o755)
		os.Chmod(path, 0o644)
	})
	return path, before
}

// currentDatabase creates a writable, current SQLite database and returns its path.
func currentDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "current.db")
	store, err := database.CreateSQLite(path, nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	store.Close()
	return path
}

// stubOpenDialog makes OpenDatabase's file chooser return *path.
func stubOpenDialog(t *testing.T, path *string) {
	t.Helper()
	saved := openDatabaseDialog
	openDatabaseDialog = func(context.Context, wruntime.OpenDialogOptions) (string, error) { return *path, nil }
	t.Cleanup(func() { openDatabaseDialog = saved })
}

// openEntryPoints are the App methods that open a SQLite database. Every
// frontend open path (welcome screen Open Database, toolbar Open, File > Open
// Database) calls OpenDatabase, which calls loadDatabase.
func openEntryPoints(t *testing.T, chosen *string) map[string]func(*App, string) (*DBInfo, error) {
	stubOpenDialog(t, chosen)
	return map[string]func(*App, string) (*DBInfo, error){
		"OpenDatabase": func(a *App, path string) (*DBInfo, error) {
			*chosen = path
			return a.OpenDatabase()
		},
		"loadDatabase": func(a *App, path string) (*DBInfo, error) {
			return a.loadDatabase(path)
		},
	}
}

func TestOpenEntryPointsRecordReadOnlyFailure(t *testing.T) {
	var chosen string
	for name, open := range openEntryPoints(t, &chosen) {
		t.Run(name, func(t *testing.T) {
			path, before := roOldDatabase(t)

			// A database is already open, as when opening from the toolbar.
			app := &App{}
			if _, err := app.loadDatabase(currentDatabase(t)); err != nil {
				t.Fatalf("opening the first database: %v", err)
			}

			_, err := open(app, path)
			if err == nil {
				t.Fatalf("%s succeeded on a read-only database that needs an upgrade", name)
			}
			if !errors.Is(err, database.ErrReadOnlyUpgrade) {
				t.Errorf("%s error = %v; want it to wrap ErrReadOnlyUpgrade", name, err)
			}
			f := app.GetReadOnlyOpenFailure()
			if f == nil || f.Path != path || f.File != "old.db" || f.Error == "" {
				t.Errorf("GetReadOnlyOpenFailure = %+v; want the path, file name, and error", f)
			}
			if app.store != nil {
				t.Errorf("a store is open after the failed open; the UI resets to no database")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Errorf("database file changed by the failed open")
			}

			// A successful open through the same entry point clears it.
			info, err := open(app, currentDatabase(t))
			if err != nil || info == nil {
				t.Fatalf("%s on a current database: %v, %v", name, info, err)
			}
			if f := app.GetReadOnlyOpenFailure(); f != nil {
				t.Errorf("GetReadOnlyOpenFailure after a successful open = %+v, want nil", f)
			}
			app.store.Close()
		})
	}
}

// TestOpenDatabaseCancelClearsReadOnlyFailure checks that cancelling the file
// dialog does not leave an earlier failure behind.
func TestOpenDatabaseCancelClearsReadOnlyFailure(t *testing.T) {
	chosen := ""
	stubOpenDialog(t, &chosen)
	app := &App{}
	app.setReadOnlyOpenFailure(&ReadOnlyOpenFailure{Path: "/media/old.db"})
	info, err := app.OpenDatabase()
	if info != nil || err != nil {
		t.Fatalf("cancelled OpenDatabase = %v, %v; want nil, nil", info, err)
	}
	if f := app.GetReadOnlyOpenFailure(); f != nil {
		t.Errorf("GetReadOnlyOpenFailure after cancel = %+v, want nil", f)
	}
}

// TestLoadDatabaseOtherFailureNotReadOnly keeps other open failures off the
// read-only dialog.
func TestLoadDatabaseOtherFailureNotReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "other.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("CREATE TABLE something_else (x TEXT)"); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	app := &App{}
	if _, err := app.loadDatabase(path); err == nil {
		t.Fatalf("loadDatabase succeeded on a database with no log2timeline table")
	}
	if f := app.GetReadOnlyOpenFailure(); f != nil {
		t.Errorf("GetReadOnlyOpenFailure = %+v after a non-read-only failure, want nil", f)
	}
}

// TestPostgresOffsetOverflowNamesColumn inserts an offset past 32 bits into a
// pre-v0.18.1 table and checks the error names the column, the event, and the
// value, with pgx's original text kept.
func TestPostgresOffsetOverflowNamesColumn(t *testing.T) {
	connStr := mxCreatePGDatabase(t)
	raw, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("opening connection: %v", err)
	}
	t.Cleanup(func() { raw.Close() })
	if _, err := raw.Exec(oldPostgresDDL); err != nil {
		t.Fatalf("creating old schema: %v", err)
	}
	pg, err := database.OpenPostgres(connStr)
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(func() { pg.Close() })

	events := []*model.Event{
		{Datetime: "2025-01-10 10:00:00", Source: "FILE", Desc: "fits", Offset: 4096},
		{Datetime: "2025-01-10 10:01:00", Source: "FILE", Desc: "too big", Offset: bigOffset},
	}
	_, err = pg.InsertEvents(events, nil)
	if err == nil {
		t.Fatalf("InsertEvents into an INT offset succeeded with offset %d", bigOffset)
	}
	want := `inserting event 2: column "offset" value 5747310864: failed to encode args[18]`
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %q\nwant prefix %q", err.Error(), want)
	}
	if n, cerr := pg.CountEvents("", nil); cerr != nil || n != 0 {
		t.Errorf("events after the failed insert = %d, %v; want 0 (one transaction)", n, cerr)
	}
}
