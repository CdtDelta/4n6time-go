package database

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

func statEvents(descs ...string) []*model.Event {
	var events []*model.Event
	for _, d := range descs {
		events = append(events, &model.Event{Datetime: "2025-01-10 10:00:00", Source: "FILE", Desc: d})
	}
	return events
}

func mustStat(t *testing.T, db Store) (int64, bool) {
	t.Helper()
	v, ok, err := db.GetStat(StatEventCount)
	if err != nil {
		t.Fatalf("GetStat: %v", err)
	}
	return v, ok
}

// dropEventCount removes the event_count row, as in a database that was never
// seeded or counted.
func dropEventCount(t *testing.T, conn *sql.DB) {
	t.Helper()
	if _, err := conn.Exec("DELETE FROM l2t_stats WHERE key = 'event_count'"); err != nil {
		t.Fatalf("removing event_count: %v", err)
	}
}

func TestEventCountIncrementsInInsertTransaction(t *testing.T) {
	db := createTestDB(t)
	dropEventCount(t, db.conn)

	// No row: inserts leave it missing rather than inventing a value.
	if _, err := db.InsertEvents(statEvents("a", "b"), nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	if _, ok := mustStat(t, db); ok {
		t.Fatalf("event_count row appeared without a rebuild")
	}

	if err := db.InsertStatIfMissing(StatEventCount, 2); err != nil {
		t.Fatalf("InsertStatIfMissing: %v", err)
	}
	if _, err := db.InsertEvents(statEvents("c", "d", "e"), nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	if err := db.InsertEvent(statEvents("f")[0]); err != nil {
		t.Fatalf("InsertEvent: %v", err)
	}
	if v, ok := mustStat(t, db); !ok || v != 6 {
		t.Errorf("event_count = %d (present %v), want 6", v, ok)
	}
	if live, _ := db.CountEvents("", nil); live != 6 {
		t.Errorf("live count = %d, want 6", live)
	}

	// Notes are not evidence and do not change the count.
	if _, err := db.InsertExaminerNote("2025-01-10 11:00:00", "note", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote: %v", err)
	}
	if v, _ := mustStat(t, db); v != 6 {
		t.Errorf("event_count after a note = %d, want 6", v)
	}
}

// TestEventCountUnchangedByFailedInsert makes the third event of a batch fail
// and checks that the rows and the increment roll back together.
func TestEventCountUnchangedByFailedInsert(t *testing.T) {
	db := createTestDB(t)
	if _, err := db.InsertEvents(statEvents("a"), nil); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertStatIfMissing(StatEventCount, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.conn.Exec(`CREATE TRIGGER fail_boom BEFORE INSERT ON log2timeline
		WHEN NEW."desc" = 'boom' BEGIN SELECT RAISE(ABORT, 'boom'); END`); err != nil {
		t.Fatalf("creating trigger: %v", err)
	}
	if _, err := db.InsertEvents(statEvents("b", "c", "boom", "d"), nil); err == nil {
		t.Fatalf("InsertEvents succeeded past the failing row")
	}
	if v, _ := mustStat(t, db); v != 1 {
		t.Errorf("event_count after a failed insert = %d, want 1", v)
	}
	if live, _ := db.CountEvents("", nil); live != 1 {
		t.Errorf("live count after a failed insert = %d, want 1 (rolled back)", live)
	}
}

func TestStatWrites(t *testing.T) {
	db := createTestDB(t)
	dropEventCount(t, db.conn)
	if err := db.InsertStatIfMissing(StatEventCount, 5); err != nil {
		t.Fatal(err)
	}
	// An existing row is not overwritten by InsertStatIfMissing.
	if err := db.InsertStatIfMissing(StatEventCount, 9); err != nil {
		t.Fatal(err)
	}
	if v, _ := mustStat(t, db); v != 5 {
		t.Errorf("after InsertStatIfMissing on an existing row = %d, want 5", v)
	}
	// SetStat overwrites, including a value past 32 bits.
	if err := db.SetStat(StatEventCount, 77_500_000_000); err != nil {
		t.Fatal(err)
	}
	if v, _ := mustStat(t, db); v != 77_500_000_000 {
		t.Errorf("after SetStat = %d, want 77500000000", v)
	}
}

// TestStatsMigration opens a database without l2t_stats and checks the
// migration adds it, and that running it again is harmless. The database has
// no events, so the migration also seeds event_count = 0.
func TestStatsMigration(t *testing.T) {
	path := tempDBPath(t)
	db, err := CreateSQLite(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.conn.Exec("DROP TABLE l2t_stats"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if v, ok, err := db.GetStat(StatEventCount); err != nil || !ok || v != 0 {
		t.Errorf("GetStat on a migrated empty database = %d, %v, %v; want 0, true, nil", v, ok, err)
	}
}

// TestSeedEventCountNewDatabase checks a new database starts with
// event_count = 0, so its first import is counted without a rebuild.
func TestSeedEventCountNewDatabase(t *testing.T) {
	db := createTestDB(t)
	if v, ok := mustStat(t, db); !ok || v != 0 {
		t.Fatalf("new database event_count = %d (present %v), want 0", v, ok)
	}
	if _, err := db.InsertEvents(statEvents("a", "b", "c"), nil); err != nil {
		t.Fatal(err)
	}
	if v, _ := mustStat(t, db); v != 3 {
		t.Errorf("event_count after the first insert = %d, want 3", v)
	}
}

// TestSeedEventCountSkipsNonEmpty opens a database that has evidence but no
// event_count row: the open must not seed or count it.
func TestSeedEventCountSkipsNonEmpty(t *testing.T) {
	path := tempDBPath(t)
	db, err := CreateSQLite(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertEvents(statEvents("a", "b"), nil); err != nil {
		t.Fatal(err)
	}
	dropEventCount(t, db.conn)
	db.Close()

	db, err = OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer db.Close()
	if v, ok := mustStat(t, db); ok {
		t.Errorf("non-empty database seeded at open with %d", v)
	}
}

// TestSeedEventCountIdempotent runs the seed again on a database whose row
// already holds a value; it must not reset it, empty or not.
func TestSeedEventCountIdempotent(t *testing.T) {
	db := createTestDB(t)
	if err := seedEventCount(db.conn, db.dialect); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if v, _ := mustStat(t, db); v != 0 {
		t.Errorf("after a second seed = %d, want 0", v)
	}
	// A wrong value on an empty database stays: the seed only fills a missing row.
	if err := db.SetStat(StatEventCount, 7); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if v, _ := mustStat(t, db); v != 7 {
		t.Errorf("after Migrate = %d, want the existing 7", v)
	}
}

// TestSeedEventCountReadOnlyStillOpens checks that a current, empty database
// with no event_count row opens from read-only storage: the seed cannot
// write, and that must not fail the open.
func TestSeedEventCountReadOnlyStillOpens(t *testing.T) {
	skipUnlessPermissionsApply(t)
	path := roDatabase(t, func(t *testing.T, conn *sql.DB) {
		if _, err := conn.Exec("DELETE FROM log2timeline"); err != nil {
			t.Fatal(err)
		}
		dropEventCount(t, conn)
	})
	before := readFile(t, path)
	makeReadOnly(t, path, 0o444, 0o555)

	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite on read-only storage: %v", err)
	}
	defer db.Close()
	if _, ok := mustStat(t, db); ok {
		t.Errorf("event_count written to a read-only file")
	}
	if string(readFile(t, path)) != string(before) {
		t.Errorf("database file changed by the open")
	}
}

// preStats removes l2t_stats, as in a database from before pass 4a.
func preStats(t *testing.T, conn *sql.DB) {
	t.Helper()
	if _, err := conn.Exec("DROP TABLE l2t_stats"); err != nil {
		t.Fatalf("dropping l2t_stats: %v", err)
	}
}

// TestOpenSQLiteReadOnlyMissingStats checks that a database whose only gap is
// l2t_stats still gets the read-only upgrade error on read-only storage.
func TestOpenSQLiteReadOnlyMissingStats(t *testing.T) {
	skipUnlessPermissionsApply(t)
	path := roDatabase(t, preStats)
	before := readFile(t, path)
	makeReadOnly(t, path, 0o444, 0o555)

	db, err := OpenSQLite(path)
	if err == nil {
		db.Close()
		t.Fatalf("OpenSQLite succeeded without creating l2t_stats")
	}
	if !errors.Is(err, ErrReadOnlyUpgrade) {
		t.Errorf("error = %v; want it to wrap ErrReadOnlyUpgrade", err)
	}
	if string(readFile(t, path)) != string(before) {
		t.Errorf("database file changed by the failed open")
	}
}

func TestUpdateEventAllowlist(t *testing.T) {
	db := createTestDB(t)
	if _, err := db.InsertEvents([]*model.Event{{Datetime: "2025-01-10 10:00:00", Source: "FILE", Host: "WS1", ImportID: 3}}, nil); err != nil {
		t.Fatal(err)
	}
	events, _ := db.QueryEvents("", nil, "", 0, 0)
	id := events[0].ID

	allowed := map[string]interface{}{
		"tag": "t1", "color": "RED", "notes": "n", "reportnotes": "r", "bookmark": int64(1),
	}
	for field, value := range allowed {
		if err := db.UpdateEvent(id, map[string]interface{}{field: value}); err != nil {
			t.Errorf("UpdateEvent(%s) = %v, want nil", field, err)
		}
	}
	got, _ := db.QueryEvents("rowid = ?", []interface{}{id}, "", 0, 0)
	e := got[0]
	if e.Tag != "t1" || e.Color != "RED" || e.Notes != "n" || e.ReportNotes != "r" || e.Bookmark != 1 {
		t.Errorf("after allowed updates: %+v", e)
	}

	for field, value := range map[string]interface{}{
		"source": "REG", "host": "WS2", "import_id": int64(9), "datetime": "2030-01-01 00:00:00",
	} {
		err := db.UpdateEvent(id, map[string]interface{}{field: value})
		if !errors.Is(err, ErrReadOnlyField) {
			t.Errorf("UpdateEvent(%s) = %v, want ErrReadOnlyField", field, err)
		}
	}
	got, _ = db.QueryEvents("rowid = ?", []interface{}{id}, "", 0, 0)
	e = got[0]
	if e.Source != "FILE" || e.Host != "WS1" || e.ImportID != 3 {
		t.Errorf("rejected fields changed: source %q host %q import_id %d", e.Source, e.Host, e.ImportID)
	}

	if err := db.UpdateEvent(id, map[string]interface{}{"no_such_column": "x"}); err == nil || errors.Is(err, ErrReadOnlyField) {
		t.Errorf("unknown field error = %v, want the invalid field error", err)
	}
}
