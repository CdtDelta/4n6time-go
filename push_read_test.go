package main

import (
	"path/filepath"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/query"
)

// newPushReadApp seeds six evidence rows and three examiner notes, so the push
// read has both kinds of row to separate.
func newPushReadApp(t *testing.T) *App {
	t.Helper()
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "pushread.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	evidence := []*model.Event{
		{Datetime: "2025-01-01 10:00:00", Source: "FILE", Host: "WS1", Desc: "malware.exe"},
		{Datetime: "2025-01-02 10:00:00", Source: "FILE", Host: "WS1", Desc: "benign.txt"},
		{Datetime: "2025-01-03 10:00:00", Source: "REG", Host: "WS1", Desc: "run key"},
		{Datetime: "2025-01-04 10:00:00", Source: "EVT", Host: "WS2", Desc: "logon"},
		{Datetime: "2025-01-05 10:00:00", Source: "EVT", Host: "WS2", Desc: "service install"},
		{Datetime: "2025-01-06 10:00:00", Source: "FILE", Host: "WS2", Desc: "report.docx"},
	}
	if _, err := store.InsertEvents(evidence, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	notes := [][4]string{
		{"2025-02-01 09:00:00", "beacon to c2", "examiner_entry", "RED"},
		{"2025-02-02 09:00:00", "user login", "examiner_entry", "BLUE"},
		{"2025-02-03 09:00:00", "cleanup", "examiner_entry", ""},
	}
	for _, n := range notes {
		if _, err := store.InsertExaminerNote(n[0], n[1], n[2], n[3]); err != nil {
			t.Fatalf("InsertExaminerNote: %v", err)
		}
	}
	return &App{store: store, driver: "sqlite"}
}

// The notes-inclusive builder, given the push read's settings, returns the
// notes. This shows the fixture has notes to leak, so the next test's
// evidence-only result comes from the push read itself.
func TestNotesInclusiveBuilderReturnsNotesPushReadDoesNot(t *testing.T) {
	app := newPushReadApp(t)
	q := query.New(999999999)
	q.SetDialect(query.DefaultDialect)
	q.OrderBy("datetime")
	q.SetPage(1)
	combinedSQL, combinedArgs := q.BuildCombined()

	combined, err := app.store.ExecuteQuery(combinedSQL, combinedArgs)
	if err != nil {
		t.Fatalf("ExecuteQuery (combined): %v", err)
	}
	if got := countNoteRows(combined); got != 3 {
		t.Fatalf("notes-inclusive builder: got %d note rows, want 3 (fixture has no notes to leak)", got)
	}

	pushSQL, pushArgs := buildPushReadQuery()
	pushed, err := app.store.ExecuteQuery(pushSQL, pushArgs)
	if err != nil {
		t.Fatalf("ExecuteQuery (push read): %v", err)
	}
	if got := countNoteRows(pushed); got != 0 {
		t.Errorf("push read: got %d note rows, want 0", got)
	}
}

func countNoteRows(events []*model.Event) int {
	n := 0
	for _, e := range events {
		if e.ID < 0 {
			n++
		}
	}
	return n
}

func TestPushReadQueryExcludesExaminerNotes(t *testing.T) {
	app := newPushReadApp(t)
	sqlStr, args := buildPushReadQuery()

	events, err := app.store.ExecuteQuery(sqlStr, args)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	const wantEvidence = 6
	if len(events) != wantEvidence {
		t.Errorf("event count = %d, want evidence count %d", len(events), wantEvidence)
	}
	for _, e := range events {
		if e.ID < 0 {
			t.Errorf("event %q has negative ID %d", e.Desc, e.ID)
		}
		if e.Source == "EXAMINER" {
			t.Errorf("event %q has source EXAMINER", e.Desc)
		}
	}
}

func TestEvidenceOnly(t *testing.T) {
	mixed := []*model.Event{
		{ID: 1, Desc: "a"},
		{ID: -1, Desc: "note one"},
		{ID: 2, Desc: "b"},
		{ID: -2, Desc: "note two"},
		{ID: 3, Desc: "c"},
	}
	got, dropped := evidenceOnly(mixed)
	if dropped != 2 {
		t.Errorf("dropped = %d, want 2", dropped)
	}
	if len(got) != 3 {
		t.Fatalf("kept %d events, want 3", len(got))
	}
	for i, want := range []string{"a", "b", "c"} {
		if got[i].Desc != want || got[i].ID < 0 {
			t.Errorf("kept[%d] = {ID %d, Desc %q}, want positive-ID %q", i, got[i].ID, got[i].Desc, want)
		}
	}
	if len(mixed) != 5 {
		t.Errorf("input slice length changed to %d", len(mixed))
	}

	allEvidence := []*model.Event{{ID: 1}, {ID: 2}}
	same, n := evidenceOnly(allEvidence)
	if n != 0 {
		t.Errorf("all-evidence dropped = %d, want 0", n)
	}
	if len(same) != 2 || &same[0] != &allEvidence[0] {
		t.Errorf("all-evidence input not returned unchanged")
	}

	empty, n := evidenceOnly(nil)
	if n != 0 || len(empty) != 0 {
		t.Errorf("nil input: got %d events, %d dropped; want 0, 0", len(empty), n)
	}
}
