package database

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// roOldImportsDDL is l2t_imports as v0.16.0 through v0.18.1 created it,
// before error_message.
const roOldImportsDDL = `CREATE TABLE l2t_imports (
	id INTEGER PRIMARY KEY, filename TEXT, label TEXT, format TEXT,
	imported_at TEXT, event_count INT DEFAULT 0, status TEXT
)`

// skipUnlessPermissionsApply skips tests that rely on file permissions, which
// root ignores and Windows does not express with chmod.
func skipUnlessPermissionsApply(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("file permission tests do not apply on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not apply, so read-only storage cannot be simulated")
	}
}

// roDatabase creates a SQLite database in its own directory with one event,
// lets shape turn it into an older schema, and returns its path.
func roDatabase(t *testing.T, shape func(*testing.T, *sql.DB)) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "media")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "case.db")
	db, err := CreateSQLite(path, nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	if _, err := db.conn.Exec(`INSERT INTO log2timeline (datetime, source, "desc") VALUES ('2025-01-10 10:00:00', 'FILE', 'one')`); err != nil {
		t.Fatalf("inserting event: %v", err)
	}
	if shape != nil {
		shape(t, db.conn)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return path
}

// pre019 replaces l2t_imports with the table v0.16.0 to v0.18.1 created.
func pre019(t *testing.T, conn *sql.DB) {
	t.Helper()
	for _, stmt := range []string{"DROP TABLE l2t_imports", roOldImportsDDL} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// pre016 removes import provenance entirely, as before v0.16.0.
func pre016(t *testing.T, conn *sql.DB) {
	t.Helper()
	for _, stmt := range []string{
		"DROP INDEX import_id_idx",
		"ALTER TABLE log2timeline DROP COLUMN import_id",
		"DROP TABLE l2t_imports",
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// makeReadOnly sets the given modes on the database file and its directory,
// and restores them when the test ends so TempDir can clean up.
func makeReadOnly(t *testing.T, path string, fileMode, dirMode os.FileMode) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.Chmod(path, fileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, dirMode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(dir, 0o755)
		os.Chmod(path, 0o644)
	})
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestOpenSQLiteReadOnlyNeedsUpgrade(t *testing.T) {
	skipUnlessPermissionsApply(t)
	cases := []struct {
		name     string
		shape    func(*testing.T, *sql.DB)
		fileMode os.FileMode
		dirMode  os.FileMode
	}{
		{"pre-0.19 read-only file and directory", pre019, 0o444, 0o555},
		{"pre-0.16 read-only file and directory", pre016, 0o444, 0o555},
		{"pre-0.19 read-only directory only", pre019, 0o644, 0o555},
		{"pre-0.19 read-only file only", pre019, 0o444, 0o755},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := roDatabase(t, c.shape)
			before := readFile(t, path)
			makeReadOnly(t, path, c.fileMode, c.dirMode)

			db, err := OpenSQLite(path)
			if err == nil {
				db.Close()
				t.Fatalf("OpenSQLite succeeded; want ErrReadOnlyUpgrade")
			}
			if !errors.Is(err, ErrReadOnlyUpgrade) {
				t.Fatalf("OpenSQLite error = %v; want it to wrap ErrReadOnlyUpgrade", err)
			}
			if after := readFile(t, path); !bytes.Equal(before, after) {
				t.Errorf("database file changed by the failed open")
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if e.Name() != "case.db" {
					t.Errorf("unexpected file %s left beside the database", e.Name())
				}
			}
		})
	}
}

// TestOpenSQLiteReadOnlyCurrent confirms Step 0: a database that needs no
// migration opens and reads on read-only storage.
func TestOpenSQLiteReadOnlyCurrent(t *testing.T) {
	skipUnlessPermissionsApply(t)
	path := roDatabase(t, nil)
	before := readFile(t, path)
	makeReadOnly(t, path, 0o444, 0o555)

	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite on read-only storage: %v", err)
	}
	n, err := db.CountEvents("", nil)
	db.Close()
	if err != nil || n != 1 {
		t.Errorf("CountEvents = %d, %v; want 1, nil", n, err)
	}
	if after := readFile(t, path); !bytes.Equal(before, after) {
		t.Errorf("database file changed by a read-only open")
	}
}

// TestOpenSQLiteOtherMigrationFailure keeps the existing error path for a
// migration that fails on writable storage.
func TestOpenSQLiteOtherMigrationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notatimeline.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("CREATE TABLE something_else (x TEXT)"); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	db, err := OpenSQLite(path)
	if err == nil {
		db.Close()
		t.Fatalf("OpenSQLite succeeded on a database with no log2timeline table")
	}
	if errors.Is(err, ErrReadOnlyUpgrade) {
		t.Errorf("error %v wraps ErrReadOnlyUpgrade; want the ordinary migration error", err)
	}
}

func TestIsReadOnlyMigrationErrorNonSQLite(t *testing.T) {
	if isReadOnlyMigrationError(errors.New("attempt to write a readonly database"), "/tmp/x.db") {
		t.Errorf("a plain error with read-only text counted as read-only; only SQLite codes should")
	}
}
