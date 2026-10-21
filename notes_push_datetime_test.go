package main

import (
	"path/filepath"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/query"
)

// TestNotesMatrixPushDatetimes runs push's read and insert steps (the RFC 3339
// display strings from the SQLite scan path into PostgreSQL TIMESTAMP columns)
// and reads the rows back from PostgreSQL. PushToPostgres itself needs the
// Wails runtime, so the test calls the same store methods it does.
func TestNotesMatrixPushDatetimes(t *testing.T) {
	connStr := mxCreatePGDatabase(t)

	src, err := database.CreateSQLite(filepath.Join(t.TempDir(), "push.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { src.Close() })
	stored := map[string]string{
		"space":      "2025-01-10 10:00:00",
		"T":          "2025-01-10T11:00:00",
		"TZ":         "2025-01-10T12:00:00Z",
		"fractional": "2025-01-10 13:00:00.5",
		"junk":       "Not a time",
	}
	var events []*model.Event
	for desc, dt := range stored {
		events = append(events, &model.Event{Datetime: dt, Source: "FILE", Desc: desc})
	}
	if _, err := src.InsertEvents(events, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	if _, err := src.InsertExaminerNote("2025-06-30T18:00:00Z", "pasted note", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote: %v", err)
	}
	if _, err := src.InsertExaminerNote("2025-06-30 19:00:00", "typed note", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote: %v", err)
	}

	// What push reads, in the display form.
	readSQL, readArgs := buildPushReadQuery()
	read, err := src.ExecuteQuery(readSQL, readArgs)
	if err != nil {
		t.Fatalf("reading SQLite events: %v", err)
	}
	notes, err := src.GetExaminerNotes()
	if err != nil {
		t.Fatalf("GetExaminerNotes: %v", err)
	}
	wantEvents := map[string]string{
		"space":      "2025-01-10T10:00:00Z",
		"T":          "2025-01-10T11:00:00Z",
		"TZ":         "2025-01-10T12:00:00Z",
		"fractional": "2025-01-10T13:00:00.5Z",
		"junk":       "Not a time",
	}
	wantNotes := map[string]string{"pasted note": "2025-06-30T18:00:00Z", "typed note": "2025-06-30T19:00:00Z"}
	checkDatetimes(t, "SQLite push read", read, wantEvents)
	checkDatetimes(t, "SQLite GetExaminerNotes", notes, wantNotes)

	pg, err := database.CreatePostgres(connStr, nil)
	if err != nil {
		t.Fatalf("CreatePostgres: %v", err)
	}
	t.Cleanup(func() { pg.Close() })
	if _, err := pg.InsertEvents(read, nil); err != nil {
		t.Fatalf("inserting into PostgreSQL: %v", err)
	}
	for _, n := range notes {
		if _, err := pg.InsertExaminerNote(n.Datetime, n.Desc, n.Tag, n.Color); err != nil {
			t.Fatalf("inserting note into PostgreSQL: %v", err)
		}
	}

	q := query.New(1000)
	q.SetDialect(&database.PostgresDialect{})
	q.OrderBy("datetime")
	q.SetPage(1)
	pgSQL, pgArgs := q.Build()
	back, err := pg.ExecuteQuery("/* no-notes */ "+pgSQL, pgArgs)
	if err != nil {
		t.Fatalf("reading PostgreSQL events: %v", err)
	}
	backNotes, err := pg.GetExaminerNotes()
	if err != nil {
		t.Fatalf("PostgreSQL GetExaminerNotes: %v", err)
	}
	// pgSanitizeDatetime stores a non-datetime as NULL, as v0.17.0 did, so it
	// reads back empty. Every real datetime survives byte for byte.
	wantEvents["junk"] = ""
	checkDatetimes(t, "PostgreSQL events", back, wantEvents)
	checkDatetimes(t, "PostgreSQL notes", backNotes, wantNotes)
}

func checkDatetimes(t *testing.T, what string, events []*model.Event, want map[string]string) {
	t.Helper()
	got := map[string]string{}
	for _, e := range events {
		got[e.Desc] = e.Datetime
	}
	if len(got) != len(want) {
		t.Errorf("%s: %d rows %v, want %d", what, len(got), got, len(want))
	}
	for desc, w := range want {
		if g, ok := got[desc]; !ok || g != w {
			t.Errorf("%s: %s datetime = %q, want %q", what, desc, g, w)
		}
	}
}
