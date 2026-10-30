package main

import (
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// countingStore counts live evidence counts (CountEvents) so tests can tell
// whether the cache was used, and can fail InsertStatIfMissing.
type countingStore struct {
	database.Store
	liveCounts int
	statErr    error
}

func (s *countingStore) CountEvents(where string, args []interface{}) (int64, error) {
	s.liveCounts++
	return s.Store.CountEvents(where, args)
}

func (s *countingStore) InsertStatIfMissing(key string, value int64) error {
	if s.statErr != nil {
		return s.statErr
	}
	return s.Store.InsertStatIfMissing(key, value)
}

func ccEvents(n int, host string) []*model.Event {
	events := make([]*model.Event, n)
	for i := range events {
		events[i] = &model.Event{Datetime: "2025-01-10 10:00:00", Source: "FILE", Host: host, Desc: "e"}
	}
	return events
}

func ccStat(t *testing.T, store database.Store) (int64, bool) {
	t.Helper()
	v, ok, err := store.GetStat(database.StatEventCount)
	if err != nil {
		t.Fatalf("GetStat: %v", err)
	}
	return v, ok
}

// ccDropStat removes the event_count row, as in a database from before the
// cached count or one never seeded.
func ccDropStat(t *testing.T, store interface{ Conn() *sql.DB }) {
	t.Helper()
	if _, err := store.Conn().Exec("DELETE FROM l2t_stats WHERE key = 'event_count'"); err != nil {
		t.Fatalf("removing event_count: %v", err)
	}
}

func ccLive(t *testing.T, store database.Store) int64 {
	t.Helper()
	n, err := store.CountEvents("", nil)
	if err != nil {
		t.Fatalf("CountEvents: %v", err)
	}
	return n
}

func TestCachedEvidenceCount(t *testing.T) {
	for _, b := range ifBackends() {
		t.Run(b.name, func(t *testing.T) {
			raw := b.newStore(t)
			if _, err := raw.InsertEvents(ccEvents(3, "WS1"), nil); err != nil {
				t.Fatal(err)
			}
			ccDropStat(t, raw)
			store := &countingStore{Store: raw}
			var warnings []string
			warn := func(m string) { warnings = append(warnings, m) }

			// Missing row: one live count, and the row is written.
			n, err := cachedEvidenceCount(store, warn)
			if err != nil || n != 3 {
				t.Fatalf("rebuild = %d, %v; want 3", n, err)
			}
			if store.liveCounts != 1 {
				t.Errorf("live counts during rebuild = %d, want 1", store.liveCounts)
			}
			if v, ok := ccStat(t, raw); !ok || v != 3 {
				t.Errorf("event_count after rebuild = %d (present %v), want 3", v, ok)
			}

			// Row present: matches a live count, and no live count runs.
			store.liveCounts = 0
			n, err = cachedEvidenceCount(store, warn)
			if err != nil || n != ccLive(t, raw) {
				t.Errorf("cached = %d, %v; want the live count %d", n, err, ccLive(t, raw))
			}
			if store.liveCounts != 0 {
				t.Errorf("live counts with the row present = %d, want 0", store.liveCounts)
			}
			if len(warnings) != 0 {
				t.Errorf("warnings = %q, want none", warnings)
			}
		})
	}
}

// TestCachedEvidenceCountWriteFails covers storage that cannot take the
// rebuilt row: the live value is returned with a warning, not an error.
func TestCachedEvidenceCountWriteFails(t *testing.T) {
	raw := newSQLiteForPush(t, "ro.db")
	if _, err := raw.InsertEvents(ccEvents(2, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	ccDropStat(t, raw)
	store := &countingStore{Store: raw, statErr: errors.New("attempt to write a readonly database")}
	var warnings []string
	n, err := cachedEvidenceCount(store, func(m string) { warnings = append(warnings, m) })
	if err != nil || n != 2 {
		t.Errorf("count = %d, %v; want 2, nil", n, err)
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %q, want one", warnings)
	}
}

// TestCachedEvidenceCountReadOnlyFile opens a current database from
// read-only storage and checks the count falls back to live without failing.
func TestCachedEvidenceCountReadOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permissions do not apply here")
	}
	dir := filepath.Join(t.TempDir(), "media")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "current.db")
	store, err := database.CreateSQLite(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertEvents(ccEvents(4, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	ccDropStat(t, store)
	store.Close()
	os.Chmod(path, 0o444)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755); os.Chmod(path, 0o644) })

	app := &App{}
	info, err := app.loadDatabase(path)
	if err != nil {
		t.Fatalf("loadDatabase on read-only storage: %v", err)
	}
	defer app.store.Close()
	if info.EventCount != 4 {
		t.Errorf("EventCount = %d, want 4", info.EventCount)
	}
	if _, ok := ccStat(t, app.store); ok {
		t.Errorf("event_count row written to a read-only file")
	}
}

// TestCountRouting checks which grid counts use the cache: an unfiltered
// QueryEvents and getDBInfo read it, and every constrained count stays live.
func TestCountRouting(t *testing.T) {
	store := newSQLiteForPush(t, "routing.db")
	if _, err := store.InsertEvents(append(ccEvents(3, "WS1"), ccEvents(2, "WS2")...), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertExaminerNote("2025-01-10 11:00:00", "note", "", ""); err != nil {
		t.Fatal(err)
	}
	// A deliberately wrong cached value shows which path read it.
	if err := store.SetStat(database.StatEventCount, 1000); err != nil {
		t.Fatal(err)
	}
	app := &App{store: store, driver: "sqlite"}

	info, err := app.getDBInfo()
	if err != nil || info.EventCount != 1000 {
		t.Errorf("getDBInfo EventCount = %v, %v; want the cached 1000", info, err)
	}
	resp, err := app.QueryEvents(QueryRequest{})
	if err != nil || resp.TotalCount != 1001 {
		t.Errorf("unfiltered total = %v, %v; want cached 1000 + 1 live note", resp, err)
	}

	constrained := map[string]QueryRequest{
		"filter":        {Filters: []FilterItem{{Field: "host", Operator: "=", Value: "WS1"}}},
		"search":        {SearchText: "e"},
		"bookmark-only": {BookmarkOnly: true},
		"date range":    {DateFrom: "2025-01-01", DateTo: "2025-12-31"},
		"tab base":      {BaseField: "host", BaseOp: "=", BaseValue: "WS2"},
	}
	want := map[string]int64{"filter": 3, "search": 6, "bookmark-only": 0, "date range": 6, "tab base": 2}
	for name, req := range constrained {
		resp, err := app.QueryEvents(req)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if resp.TotalCount != want[name] {
			t.Errorf("%s total = %d, want the live %d", name, resp.TotalCount, want[name])
		}
	}
}

func TestRecountEventsCorrectsWrongValue(t *testing.T) {
	store := newSQLiteForPush(t, "recount.db")
	if _, err := store.InsertEvents(ccEvents(5, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStat(database.StatEventCount, 42); err != nil {
		t.Fatal(err)
	}
	app := &App{store: store, driver: "sqlite"}
	res, err := app.RecountEvents()
	if err != nil {
		t.Fatalf("RecountEvents: %v", err)
	}
	if res.Count != 5 || !res.HadPrevious || res.Previous != 42 {
		t.Errorf("result = %+v, want count 5, previous 42", res)
	}
	if v, _ := ccStat(t, store); v != 5 {
		t.Errorf("event_count after recount = %d, want 5", v)
	}
	if _, err := (&App{}).RecountEvents(); err == nil {
		t.Errorf("RecountEvents with no database: want an error")
	}
}

// TestCachedCountAfterImports runs single-file and recursive import into a
// database with a cached count and checks it equals a live count afterward,
// and that a failed import leaves it unchanged.
func TestCachedCountAfterImports(t *testing.T) {
	store := newSQLiteForPush(t, "imports.db")
	if _, err := store.InsertEvents(ccEvents(2, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	app := &App{store: store, driver: "sqlite"}
	if _, err := app.getDBInfo(); err != nil { // builds the cached row
		t.Fatal(err)
	}

	if _, err := app.ImportCSV(ifTLN(t), ""); err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if v, _ := ccStat(t, store); v != ccLive(t, store) || v != 4 {
		t.Errorf("after single-file import: cached %d, live %d, want 4", v, ccLive(t, store))
	}

	dir := t.TempDir()
	lecmd := "SourceFile,TargetCreated,TargetModified,LocalPath,DriveType\n" +
		"test.lnk,2026-01-01 00:00:00,2026-01-02 00:00:00,C:\\path\\to\\file,Fixed\n"
	if err := os.WriteFile(filepath.Join(dir, "a_LECmd.csv"), []byte(lecmd), 0o600); err != nil {
		t.Fatal(err)
	}
	summary, err := app.ImportFolderRecursive(dir, "")
	if err != nil || summary.TotalEvents == 0 {
		t.Fatalf("ImportFolderRecursive: %+v, %v", summary, err)
	}
	if v, _ := ccStat(t, store); v != ccLive(t, store) {
		t.Errorf("after recursive import: cached %d, live %d", v, ccLive(t, store))
	}

	before, _ := ccStat(t, store)
	app.store = &failingStore{Store: store, insertErr: errors.New("boom")}
	if _, err := app.ImportCSV(ifTLN(t), ""); err == nil {
		t.Fatalf("ImportCSV through a failing store succeeded")
	}
	if v, ok := ccStat(t, store); !ok || v != before || v != ccLive(t, store) {
		t.Errorf("after a failed import: cached %d (present %v), want unchanged %d = live", v, ok, before)
	}
}

// TestCachedCountPushTarget does what Push to PostgreSQL does on the target:
// open it with CreateStore and insert the events with InsertEvents. A target
// that already has events and a cached count ends with a cached count equal
// to a live count.
func TestCachedCountPushTarget(t *testing.T) {
	connStr := mxCreatePGDatabase(t)
	target, err := database.CreateStore("postgres", connStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close() })
	if _, err := target.InsertEvents(ccEvents(3, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := cachedEvidenceCount(target, func(string) {}); err != nil {
		t.Fatal(err)
	}

	src := newSQLiteForPush(t, "src.db")
	if _, err := src.InsertEvents(ccEvents(4, "WS2"), nil); err != nil {
		t.Fatal(err)
	}
	readSQL, readArgs := buildPushReadQuery()
	events, err := src.ExecuteQuery(readSQL, readArgs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.InsertEvents(events, nil); err != nil {
		t.Fatalf("pushing events: %v", err)
	}
	if v, _ := ccStat(t, target); v != 7 || v != ccLive(t, target) {
		t.Errorf("target cached %d, live %d, want 7", v, ccLive(t, target))
	}
}

// TestCachedCountConcurrentPostgres runs two imports into one PostgreSQL
// database at once; the increments must add up.
func TestCachedCountConcurrentPostgres(t *testing.T) {
	connStr := mxCreatePGDatabase(t)
	setup, err := database.CreatePostgres(connStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.SetStat(database.StatEventCount, 0); err != nil {
		t.Fatal(err)
	}
	setup.Close()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Separate connections, as two examiners' apps would have.
			store, err := database.OpenPostgres(connStr)
			if err != nil {
				errs[i] = err
				return
			}
			defer store.Close()
			for j := 0; j < 5; j++ {
				if _, err := store.InsertEvents(ccEvents(200, "WS1"), nil); err != nil {
					errs[i] = err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent import: %v", err)
		}
	}

	check, err := database.OpenPostgres(connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	if v, _ := ccStat(t, check); v != 2000 || v != ccLive(t, check) {
		t.Errorf("cached %d, live %d, want 2000", v, ccLive(t, check))
	}
}

// TestCachedCountFailedInsertPostgres overflows an old INT offset column part
// way through a batch: the rows and the increment roll back together.
func TestCachedCountFailedInsertPostgres(t *testing.T) {
	connStr := mxCreatePGDatabase(t)
	raw, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	if _, err := raw.Exec(oldPostgresDDL); err != nil {
		t.Fatal(err)
	}
	pg, err := database.OpenPostgres(connStr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Close() })
	if _, err := pg.InsertEvents(ccEvents(2, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	if err := pg.SetStat(database.StatEventCount, 2); err != nil {
		t.Fatal(err)
	}
	batch := ccEvents(3, "WS1")
	batch[2].Offset = bigOffset
	if _, err := pg.InsertEvents(batch, nil); err == nil {
		t.Fatalf("InsertEvents into an INT offset succeeded")
	}
	if v, _ := ccStat(t, pg); v != 2 || ccLive(t, pg) != 2 {
		t.Errorf("after failed insert: cached %d, live %d, want 2 and 2", v, ccLive(t, pg))
	}
}

// TestOpenKeepsCurrentDatabaseOnFailure opens a second database while one is
// open. Failures (not a 4n6time database, an unreachable PostgreSQL server)
// leave the first open and usable; a success after the frontend saved the
// tab session leaves that session in the first database.
func TestOpenKeepsCurrentDatabaseOnFailure(t *testing.T) {
	firstPath := currentDatabase(t)
	app := &App{}
	if _, err := app.loadDatabase(firstPath); err != nil {
		t.Fatal(err)
	}
	first := app.store

	other := filepath.Join(t.TempDir(), "other.db")
	conn, err := sql.Open("sqlite", other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("CREATE TABLE something_else (x TEXT)"); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	if _, err := app.loadDatabase(other); err == nil {
		t.Fatalf("loadDatabase succeeded on a non-4n6time database")
	}
	if app.store != first || app.GetReadOnlyOpenFailure() != nil {
		t.Errorf("after a non-read-only failure: store changed (%v) or read-only failure recorded", app.store != first)
	}
	if _, err := app.ConnectPostgres("127.0.0.1", "1", "nodb", "nouser", "x", "disable"); err == nil {
		t.Fatalf("ConnectPostgres to port 1 succeeded")
	}
	if app.store != first || app.driver != "sqlite" {
		t.Errorf("after a failed connect: store or driver changed")
	}
	if _, err := app.store.CountEvents("", nil); err != nil {
		t.Errorf("first database unusable: %v", err)
	}

	// The frontend saves the current tab session, then opens. After the
	// switch, the first database keeps that session.
	const session = `{"tabs":[{"id":"t1","label":"host: WS1"}],"activeTabId":"t1"}`
	if err := app.SaveTabSession(session); err != nil {
		t.Fatalf("SaveTabSession: %v", err)
	}
	if _, err := app.loadDatabase(currentDatabase(t)); err != nil {
		t.Fatalf("opening the second database: %v", err)
	}
	if app.store == first {
		t.Fatalf("store did not switch after a successful open")
	}
	app.store.Close()

	reopened, err := database.OpenSQLite(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got, _ := reopened.LoadTabSession(); got != session {
		t.Errorf("first database session = %q, want the saved one", got)
	}
}

// ccPGParts splits a test connection string into the fields the PostgreSQL
// dialog sends. It accepts a postgres:// URL and the libpq keyword form. The
// password is never printed.
func ccPGParts(t *testing.T, connStr string) (host, port, dbName, user, password, sslMode string) {
	t.Helper()
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		u, err := url.Parse(connStr)
		if err != nil {
			t.Fatalf("parsing the test connection string failed")
		}
		if u.User != nil {
			user = u.User.Username()
			password, _ = u.User.Password()
		}
		return u.Hostname(), u.Port(), strings.TrimPrefix(u.Path, "/"), user, password, u.Query().Get("sslmode")
	}
	kv := map[string]string{}
	for _, f := range strings.Fields(connStr) {
		if k, v, ok := strings.Cut(f, "="); ok {
			kv[k] = strings.Trim(v, "'")
		}
	}
	return kv["host"], kv["port"], kv["dbname"], kv["user"], kv["password"], kv["sslmode"]
}

// TestSeedCreateAndConnect checks Create & Connect leaves event_count = 0.
func TestSeedCreateAndConnect(t *testing.T) {
	host, port, dbName, user, password, sslMode := ccPGParts(t, mxCreatePGDatabase(t))
	app := &App{}
	info, err := app.CreatePostgresDatabase(host, port, dbName, user, password, sslMode)
	if err != nil {
		t.Fatalf("CreatePostgresDatabase failed")
	}
	defer app.store.Close()
	if v, ok := ccStat(t, app.store); !ok || v != 0 {
		t.Errorf("event_count after Create & Connect = %d (present %v), want 0", v, ok)
	}
	if info.EventCount != 0 {
		t.Errorf("EventCount = %d, want 0", info.EventCount)
	}
}

// TestSeedNewSQLiteImport creates a SQLite database the way ImportCSV does
// when none is open (CreateStore), imports into it, and checks event_count
// equals the imported count with no live count at any point.
func TestSeedNewSQLiteImport(t *testing.T) {
	raw, err := database.CreateStore("sqlite", filepath.Join(t.TempDir(), "new.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	store := &countingStore{Store: raw}
	app := &App{store: store, driver: "sqlite"}

	info, err := app.ImportCSV(ifTLN(t), "")
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	live := ccLive(t, raw)
	if v, ok := ccStat(t, raw); !ok || v != live || v != 2 {
		t.Errorf("event_count = %d (present %v), want the imported 2 = live %d", v, ok, live)
	}
	if info.EventCount != 2 {
		t.Errorf("EventCount = %d, want 2", info.EventCount)
	}
	if store.liveCounts != 0 {
		t.Errorf("live counts during import into a new database = %d, want 0 (no rebuild)", store.liveCounts)
	}
}

// TestSeedPushTarget does what Push to PostgreSQL does on a new target
// (CreateStore, then InsertEvents) and checks event_count equals the pushed
// count without a rebuild. A second target that already has events but no
// row is not seeded when push opens it.
func TestSeedPushTarget(t *testing.T) {
	src := newSQLiteForPush(t, "src.db")
	if _, err := src.InsertEvents(ccEvents(4, "WS2"), nil); err != nil {
		t.Fatal(err)
	}
	readSQL, readArgs := buildPushReadQuery()
	events, err := src.ExecuteQuery(readSQL, readArgs)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("new target", func(t *testing.T) {
		raw, err := database.CreateStore("postgres", mxCreatePGDatabase(t), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { raw.Close() })
		target := &countingStore{Store: raw}
		if _, err := target.InsertEvents(events, nil); err != nil {
			t.Fatalf("pushing events: %v", err)
		}
		n, err := cachedEvidenceCount(target, func(string) {})
		if err != nil || n != 4 || n != ccLive(t, raw) {
			t.Errorf("target cached %d, %v; want 4 = live", n, err)
		}
		// One live count above is ccLive's own, on raw; none through target.
		if target.liveCounts != 0 {
			t.Errorf("live counts on the target = %d, want 0 (no rebuild)", target.liveCounts)
		}
	})

	t.Run("existing non-empty target without the row", func(t *testing.T) {
		connStr := mxCreatePGDatabase(t)
		first, err := database.CreatePostgres(connStr, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := first.InsertEvents(ccEvents(3, "WS1"), nil); err != nil {
			t.Fatal(err)
		}
		ccDropStat(t, first)
		first.Close()

		target, err := database.CreateStore("postgres", connStr, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer target.Close()
		if v, ok := ccStat(t, target); ok {
			t.Errorf("non-empty push target seeded at open with %d", v)
		}
	})
}
