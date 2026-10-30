package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/jackc/pgx/v5/pgconn"
)

// ofNotDatabase writes a file that is not a SQLite database at all.
func ofNotDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.txt")
	body := strings.Repeat("This is a text file, not a database.\n", 64)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// ofOtherSQLite writes a valid SQLite database without the 4n6time tables.
func ofOtherSQLite(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "other.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec("CREATE TABLE something_else (x TEXT)"); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestOpenFailureRecordsSQLite opens two kinds of non-4n6time file while a
// database is open: each records an OpenFailure with the friendly message and
// the raw error, the current database stays open, and no read-only failure
// is recorded. The next successful open clears it.
func TestOpenFailureRecordsSQLite(t *testing.T) {
	firstPath := currentDatabase(t)
	app := &App{}
	if _, err := app.loadDatabase(firstPath); err != nil {
		t.Fatal(err)
	}
	first := app.store
	defer func() { app.store.Close() }()

	for name, path := range map[string]string{
		"not a database": ofNotDatabase(t),
		"other sqlite":   ofOtherSQLite(t),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := app.loadDatabase(path)
			if err == nil {
				t.Fatalf("loadDatabase succeeded on %s", name)
			}
			f := app.GetOpenFailure()
			if f == nil {
				t.Fatalf("no open failure recorded")
			}
			if f.Backend != "sqlite" || f.Action != openActionOpen || f.Target != path {
				t.Errorf("failure = %+v, want sqlite open of %s", f, path)
			}
			if f.Message != openMsgNotDatabase {
				t.Errorf("message = %q, want %q (error %v)", f.Message, openMsgNotDatabase, err)
			}
			if f.Error != err.Error() {
				t.Errorf("error = %q, want the returned %q", f.Error, err.Error())
			}
			if f.StillOpen != firstPath {
				t.Errorf("stillOpen = %q, want %q", f.StillOpen, firstPath)
			}
			if app.GetReadOnlyOpenFailure() != nil {
				t.Errorf("read-only failure recorded for %s", name)
			}
			if app.store != first {
				t.Errorf("current database changed after a failed open")
			}
			if _, err := app.store.CountEvents("", nil); err != nil {
				t.Errorf("current database unusable: %v", err)
			}
		})
	}

	if _, err := app.loadDatabase(currentDatabase(t)); err != nil {
		t.Fatalf("second open: %v", err)
	}
	if f := app.GetOpenFailure(); f != nil {
		t.Errorf("open failure still recorded after a successful open: %+v", f)
	}
}

// TestOpenFailureRecordsPostgres connects and creates against a port with no
// server: both record the unreachable message, a masked target, and the
// current database as still open.
func TestOpenFailureRecordsPostgres(t *testing.T) {
	firstPath := currentDatabase(t)
	app := &App{}
	if _, err := app.loadDatabase(firstPath); err != nil {
		t.Fatal(err)
	}
	first := app.store
	defer first.Close()

	const password = "s3cret-pw"
	calls := map[string]func() (*DBInfo, error){
		openActionConnect: func() (*DBInfo, error) {
			return app.ConnectPostgres("127.0.0.1", "1", "nodb", "nouser", password, "disable")
		},
		openActionCreate: func() (*DBInfo, error) {
			return app.CreatePostgresDatabase("127.0.0.1", "1", "nodb", "nouser", password, "disable")
		},
	}
	for action, call := range calls {
		t.Run(action, func(t *testing.T) {
			_, err := call()
			if err == nil {
				t.Fatalf("%s to port 1 succeeded", action)
			}
			f := app.GetOpenFailure()
			if f == nil {
				t.Fatalf("no open failure recorded")
			}
			if f.Backend != "postgres" || f.Action != action {
				t.Errorf("failure = %+v, want postgres %s", f, action)
			}
			if f.Message != openMsgUnreachable {
				t.Errorf("message = %q, want %q (error %v)", f.Message, openMsgUnreachable, err)
			}
			if f.Error != err.Error() {
				t.Errorf("error = %q, want the returned %q", f.Error, err.Error())
			}
			if strings.Contains(f.Target, password) || strings.Contains(f.Error, password) {
				t.Errorf("password present in the recorded failure")
			}
			if !strings.Contains(f.Target, "nouser@127.0.0.1:1") {
				t.Errorf("target = %q, want the masked connection", f.Target)
			}
			if f.StillOpen != firstPath || app.store != first || app.driver != "sqlite" {
				t.Errorf("current database changed: stillOpen %q", f.StillOpen)
			}
		})
	}
}

// TestOpenFailureNoneOpen checks StillOpen is empty with no database open.
func TestOpenFailureNoneOpen(t *testing.T) {
	app := &App{}
	if _, err := app.loadDatabase(ofNotDatabase(t)); err == nil {
		t.Fatalf("loadDatabase succeeded on a text file")
	}
	if f := app.GetOpenFailure(); f == nil || f.StillOpen != "" {
		t.Errorf("failure = %+v, want one with no database still open", f)
	}
	if app.store != nil {
		t.Errorf("a store is open after a failed open")
	}
}

// TestOpenFailureReadOnlyRecordsOnlyReadOnly checks the read-only case keeps
// its own failure and records no general one.
func TestOpenFailureReadOnlyRecordsOnlyReadOnly(t *testing.T) {
	path, _ := roOldDatabase(t)
	app := &App{}
	// A general failure from an earlier attempt must not survive.
	app.setOpenFailure(&OpenFailure{Target: "stale"})
	if _, err := app.loadDatabase(path); err == nil {
		t.Fatalf("loadDatabase succeeded on a read-only older database")
	}
	if app.GetReadOnlyOpenFailure() == nil {
		t.Errorf("read-only failure not recorded")
	}
	if f := app.GetOpenFailure(); f != nil {
		t.Errorf("general failure recorded for the read-only case: %+v", f)
	}
}

// TestOpenDatabaseCancelClearsOpenFailure checks a cancelled file chooser
// clears an earlier failure.
func TestOpenDatabaseCancelClearsOpenFailure(t *testing.T) {
	app := &App{}
	app.setOpenFailure(&OpenFailure{Target: "stale"})
	empty := ""
	stubOpenDialog(t, &empty)
	if info, err := app.OpenDatabase(); info != nil || err != nil {
		t.Fatalf("cancelled OpenDatabase = %v, %v", info, err)
	}
	if f := app.GetOpenFailure(); f != nil {
		t.Errorf("failure survived a cancelled open: %+v", f)
	}
}

func TestOpenFailureMessage(t *testing.T) {
	wrap := func(err error) error { return fmt.Errorf("connecting to PostgreSQL: connecting to database: %w", err) }
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"bad password", wrap(&pgconn.PgError{Code: "28P01"}), openMsgAuth},
		{"no such database", wrap(&pgconn.PgError{Code: "3D000"}), openMsgNoPGDatabase},
		{"no schema", wrap(&pgconn.PgError{Code: "42P01"}), openMsgNoPGSchema},
		{"no privilege", wrap(&pgconn.PgError{Code: "42501"}), openMsgPermission},
		{"timeout", wrap(&database.ConnectTimeoutError{Timeout: 10 * time.Second, Err: &net.OpError{Op: "dial", Err: errors.New("i/o timeout")}}),
			"Could not reach the PostgreSQL server (no response after 10 seconds)."},
		{"timeout fractional", wrap(&database.ConnectTimeoutError{Timeout: 1500 * time.Millisecond, Err: errors.New("timeout")}),
			"Could not reach the PostgreSQL server (no response after 1.5s)."},
		{"refused", wrap(&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}), openMsgUnreachable},
		{"no host", wrap(&net.DNSError{Err: "no such host", Name: "nohost"}), openMsgUnreachable},
		{"refused text", errors.New("dial tcp 127.0.0.1:1: connect: connection refused"), openMsgUnreachable},
		{"not a database text", errors.New("opening database: connecting to database: file is not a database (26)"), openMsgNotDatabase},
		{"other sqlite text", errors.New("migrating database: SQL logic error: no such table: log2timeline (1)"), openMsgNotDatabase},
		{"pg schema text", errors.New(`migrating database: ERROR: relation "log2timeline" does not exist`), openMsgNoPGSchema},
		{"cannot open text", errors.New("unable to open database file: no such file or directory (14)"), openMsgCantOpenFile},
		{"unknown", errors.New("something else went wrong"), openMsgGeneric},
	}
	for _, c := range cases {
		if got := openFailureMessage(c.err); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
