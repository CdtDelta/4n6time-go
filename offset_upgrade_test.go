package main

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/query"
)

// bigOffset is the offset from the v0.18.0 failure report: past the 32-bit
// INT maximum of 2,147,483,647.
const bigOffset int64 = 5747310864

// oldPostgresDDL is log2timeline as v0.18.0 created it on PostgreSQL, with a
// 32-bit offset and SERIAL id.
const oldPostgresDDL = `CREATE TABLE log2timeline (
	id SERIAL PRIMARY KEY,
	timezone TEXT, MACB TEXT, source TEXT, sourcetype TEXT,
	type TEXT, "user" TEXT, host TEXT, "desc" TEXT, filename TEXT,
	inode TEXT, notes TEXT, format TEXT, extra TEXT,
	datetime TIMESTAMP, reportnotes TEXT, inreport TEXT,
	tag TEXT, color TEXT, "offset" INT, store_number INT,
	store_index INT, vss_store_number INT, URL TEXT,
	record_number TEXT, event_identifier TEXT, event_type TEXT,
	source_name TEXT, user_sid TEXT, computer_name TEXT,
	bookmark INT DEFAULT 0,
	import_id INT
)`

// pgColumnType returns the information_schema data_type of a log2timeline column.
func pgColumnType(t *testing.T, connStr, column string) string {
	t.Helper()
	conn, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("opening connection: %v", err)
	}
	defer conn.Close()
	var dataType string
	err = conn.QueryRow(`SELECT data_type FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'log2timeline' AND column_name = $1`, column).Scan(&dataType)
	if err != nil {
		t.Fatalf("reading type of %s: %v", column, err)
	}
	return dataType
}

// offsetsByDesc reads every evidence row (Build, so no notes) keyed by desc.
func offsetsByDesc(t *testing.T, store database.Store, d query.QueryDialect) map[string]int64 {
	t.Helper()
	q := query.New(1000)
	q.SetDialect(d)
	q.OrderBy("datetime")
	q.SetPage(1)
	sqlStr, args := q.Build()
	events, err := store.ExecuteQuery(sqlStr, args)
	if err != nil {
		t.Fatalf("reading events: %v", err)
	}
	got := map[string]int64{}
	for _, e := range events {
		got[e.Desc] = e.Offset
	}
	return got
}

// combinedOffsets runs the grid's notes-inclusive query and returns each
// row's offset keyed by desc. It checks that the UNION of the evidence offset
// column with the notes projection's cast resolves on the live schema.
func combinedOffsets(t *testing.T, store database.Store, d query.QueryDialect) map[string]int64 {
	t.Helper()
	q := query.New(1000)
	q.SetDialect(d)
	q.OrderBy("datetime")
	q.SetPage(1)
	sqlStr, args := q.BuildCombined()
	events, err := store.ExecuteQuery(sqlStr, args)
	if err != nil {
		t.Fatalf("combined query: %v", err)
	}
	got := map[string]int64{}
	for _, e := range events {
		got[e.Desc] = e.Offset
	}
	return got
}

func TestOffsetLargeValueNewPostgresSchema(t *testing.T) {
	connStr := mxCreatePGDatabase(t)

	// An empty database is not flagged; push and Create & Connect build it new.
	needs, err := database.PostgresOffsetNeedsUpgrade(connStr)
	if err != nil {
		t.Fatalf("PostgresOffsetNeedsUpgrade on empty database: %v", err)
	}
	if needs {
		t.Errorf("empty database flagged for offset upgrade")
	}

	pg, err := database.CreatePostgres(connStr, nil)
	if err != nil {
		t.Fatalf("CreatePostgres: %v", err)
	}
	t.Cleanup(func() { pg.Close() })

	if got := pgColumnType(t, connStr, "offset"); got != "bigint" {
		t.Errorf("new schema offset type = %q, want bigint", got)
	}
	if got := pgColumnType(t, connStr, "id"); got != "bigint" {
		t.Errorf("new schema id type = %q, want bigint", got)
	}
	if got := pgColumnType(t, connStr, "store_number"); got != "integer" {
		t.Errorf("new schema store_number type = %q, want integer (unchanged)", got)
	}
	if needs, err := pg.OffsetNeedsUpgrade(); err != nil || needs {
		t.Errorf("new schema OffsetNeedsUpgrade = %v, %v; want false, nil", needs, err)
	}

	ev := &model.Event{Datetime: "2025-01-10 10:00:00", Source: "FILE", Desc: "big", Offset: bigOffset}
	if _, err := pg.InsertEvents([]*model.Event{ev}, nil); err != nil {
		t.Fatalf("inserting large offset: %v", err)
	}
	if _, err := pg.InsertExaminerNote("2025-01-10 11:00:00", "note", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote: %v", err)
	}

	d := &database.PostgresDialect{}
	if got := offsetsByDesc(t, pg, d)["big"]; got != bigOffset {
		t.Errorf("offset read back = %d, want %d", got, bigOffset)
	}
	combined := combinedOffsets(t, pg, d)
	if got := combined["big"]; got != bigOffset {
		t.Errorf("combined offset = %d, want %d", got, bigOffset)
	}
	if _, ok := combined["note"]; !ok || combined["note"] != 0 {
		t.Errorf("combined query: note row missing or offset not NULL: %v", combined)
	}

	// The connect path reports the flag through DBInfo.
	app := &App{store: pg, driver: "postgres"}
	info, err := app.getDBInfo()
	if err != nil {
		t.Fatalf("getDBInfo: %v", err)
	}
	if info.OffsetNeedsUpgrade {
		t.Errorf("new schema DBInfo.OffsetNeedsUpgrade = true, want false")
	}
}

func TestOffsetUpgradeOldPostgresSchema(t *testing.T) {
	connStr := mxCreatePGDatabase(t)

	raw, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("opening connection: %v", err)
	}
	t.Cleanup(func() { raw.Close() })
	if _, err := raw.Exec(oldPostgresDDL); err != nil {
		t.Fatalf("creating old schema: %v", err)
	}

	// Opening runs the usual migrations; it must not change the offset type.
	pg, err := database.OpenPostgres(connStr)
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(func() { pg.Close() })

	if got := pgColumnType(t, connStr, "offset"); got != "integer" {
		t.Fatalf("old schema offset type after open = %q, want integer", got)
	}
	needs, err := pg.OffsetNeedsUpgrade()
	if err != nil || !needs {
		t.Fatalf("old schema OffsetNeedsUpgrade = %v, %v; want true, nil", needs, err)
	}
	if needs, err := database.PostgresOffsetNeedsUpgrade(connStr); err != nil || !needs {
		t.Errorf("PostgresOffsetNeedsUpgrade = %v, %v; want true, nil", needs, err)
	}
	app := &App{store: pg, driver: "postgres"}
	info, err := app.getDBInfo()
	if err != nil {
		t.Fatalf("getDBInfo: %v", err)
	}
	if !info.OffsetNeedsUpgrade {
		t.Errorf("old schema DBInfo.OffsetNeedsUpgrade = false, want true")
	}

	existing := []*model.Event{
		{Datetime: "2025-01-10 10:00:00", Source: "FILE", Desc: "small", Offset: 4096},
		{Datetime: "2025-01-10 10:01:00", Source: "FILE", Desc: "max", Offset: 2147483647},
		{Datetime: "2025-01-10 10:02:00", Source: "FILE", Desc: "negative", Offset: -1},
	}
	if _, err := pg.InsertEvents(existing, nil); err != nil {
		t.Fatalf("inserting existing rows: %v", err)
	}
	if _, err := pg.InsertExaminerNote("2025-01-10 11:00:00", "note", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote: %v", err)
	}

	d := &database.PostgresDialect{}
	// INT evidence UNION the BIGINT note cast must still resolve before upgrade.
	if got := combinedOffsets(t, pg, d); got["max"] != 2147483647 || len(got) != 4 {
		t.Errorf("combined query on INT offset = %v, want 3 events and 1 note", got)
	}

	// The reported failure: a large offset does not fit before the upgrade.
	big := &model.Event{Datetime: "2025-01-10 12:00:00", Source: "FILE", Desc: "big", Offset: bigOffset}
	if _, err := pg.InsertEvents([]*model.Event{big}, nil); err == nil {
		t.Fatalf("inserting %d into INT offset succeeded; want overflow error", bigOffset)
	}

	if err := app.UpgradeOffsetColumn(); err != nil {
		t.Fatalf("UpgradeOffsetColumn: %v", err)
	}
	if got := pgColumnType(t, connStr, "offset"); got != "bigint" {
		t.Errorf("offset type after upgrade = %q, want bigint", got)
	}
	if got := pgColumnType(t, connStr, "id"); got != "integer" {
		t.Errorf("id type after upgrade = %q, want integer (not migrated)", got)
	}
	if needs, err := app.NeedsOffsetUpgrade(); err != nil || needs {
		t.Errorf("NeedsOffsetUpgrade after upgrade = %v, %v; want false, nil", needs, err)
	}

	got := offsetsByDesc(t, pg, d)
	want := map[string]int64{"small": 4096, "max": 2147483647, "negative": -1}
	if len(got) != len(want) {
		t.Errorf("rows after upgrade = %v, want %v", got, want)
	}
	for desc, w := range want {
		if got[desc] != w {
			t.Errorf("%s offset after upgrade = %d, want %d", desc, got[desc], w)
		}
	}

	if _, err := pg.InsertEvents([]*model.Event{big}, nil); err != nil {
		t.Fatalf("inserting large offset after upgrade: %v", err)
	}
	if got := combinedOffsets(t, pg, d); got["big"] != bigOffset || len(got) != 5 {
		t.Errorf("combined query after upgrade = %v, want big=%d and 5 rows", got, bigOffset)
	}

	// A second upgrade is harmless.
	if err := database.UpgradePostgresOffset(connStr); err != nil {
		t.Errorf("repeat upgrade: %v", err)
	}
}

// TestOffsetLargeValuePush runs push's read and insert steps with a large
// offset, from SQLite into a new PostgreSQL schema. PushToPostgres itself
// needs the Wails runtime, so the test calls the store methods it calls.
func TestOffsetLargeValuePush(t *testing.T) {
	connStr := mxCreatePGDatabase(t)

	src, err := database.CreateSQLite(filepath.Join(t.TempDir(), "push.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { src.Close() })
	ev := &model.Event{Datetime: "2025-01-10 10:00:00", Source: "FILE", Desc: "big", Offset: bigOffset}
	if _, err := src.InsertEvents([]*model.Event{ev}, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}

	readSQL, readArgs := buildPushReadQuery()
	read, err := src.ExecuteQuery(readSQL, readArgs)
	if err != nil {
		t.Fatalf("reading SQLite events: %v", err)
	}
	if len(read) != 1 || read[0].Offset != bigOffset {
		t.Fatalf("push read = %+v, want one event with offset %d", read, bigOffset)
	}

	if needs, err := database.PostgresOffsetNeedsUpgrade(connStr); err != nil || needs {
		t.Fatalf("new push target flagged: %v, %v", needs, err)
	}
	pg, err := database.CreatePostgres(connStr, nil)
	if err != nil {
		t.Fatalf("CreatePostgres: %v", err)
	}
	t.Cleanup(func() { pg.Close() })
	if _, err := pg.InsertEvents(read, nil); err != nil {
		t.Fatalf("inserting into PostgreSQL: %v", err)
	}
	if got := offsetsByDesc(t, pg, &database.PostgresDialect{})["big"]; got != bigOffset {
		t.Errorf("pushed offset = %d, want %d", got, bigOffset)
	}
}

func TestOffsetLargeValueSQLite(t *testing.T) {
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "offset.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ev := &model.Event{Datetime: "2025-01-10 10:00:00", Source: "FILE", Desc: "big", Offset: bigOffset}
	if _, err := store.InsertEvents([]*model.Event{ev}, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	if _, err := store.InsertExaminerNote("2025-01-10 11:00:00", "note", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote: %v", err)
	}
	if got := offsetsByDesc(t, store, query.DefaultDialect)["big"]; got != bigOffset {
		t.Errorf("offset read back = %d, want %d", got, bigOffset)
	}
	if got := combinedOffsets(t, store, query.DefaultDialect); got["big"] != bigOffset || len(got) != 2 {
		t.Errorf("combined query = %v, want big=%d and a note", got, bigOffset)
	}

	if needs, err := store.OffsetNeedsUpgrade(); err != nil || needs {
		t.Errorf("SQLite OffsetNeedsUpgrade = %v, %v; want false, nil", needs, err)
	}
	app := &App{store: store, driver: "sqlite"}
	if err := app.UpgradeOffsetColumn(); err == nil {
		t.Errorf("UpgradeOffsetColumn on SQLite succeeded; want an error")
	}
	info, err := app.getDBInfo()
	if err != nil {
		t.Fatalf("getDBInfo: %v", err)
	}
	if info.OffsetNeedsUpgrade {
		t.Errorf("SQLite DBInfo.OffsetNeedsUpgrade = true, want false")
	}
}

func TestPostgresConnStr(t *testing.T) {
	got := postgresConnStr("", "", "cases", "analyst", "p@ss:w#rd/%1", "")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parsing %q: %v", got, err)
	}
	if u.Host != "localhost:5432" || u.Path != "cases" && u.Path != "/cases" {
		t.Errorf("host/path = %q %q, want localhost:5432 cases", u.Host, u.Path)
	}
	if pw, _ := u.User.Password(); pw != "p@ss:w#rd/%1" {
		t.Errorf("password round-trip = %q", pw)
	}
	if u.Query().Get("sslmode") != "disable" {
		t.Errorf("sslmode = %q, want disable", u.Query().Get("sslmode"))
	}
	if got := postgresConnStr("db.example.local", "6543", "x", "u", "p", "require"); got != "postgres://u:p@db.example.local:6543/x?sslmode=require" {
		t.Errorf("explicit values = %q", got)
	}
}
