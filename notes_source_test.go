package main

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// newNotesSourceApp seeds four evidence rows with several sources and hosts, and
// two examiner notes with different colors. Note B is bookmarked.
//
//	evidence: E1 FILE WS1, E2 FILE WS1, E3 REG WS1, E4 FILE WS2
//	note A: color RED, not bookmarked
//	note B: color BLUE, bookmarked
func newNotesSourceApp(t *testing.T) *App {
	t.Helper()
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "notes.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	evidence := []*model.Event{
		{Datetime: "2025-01-01 10:00:00", Source: "FILE", Host: "WS1", Desc: "f1"},
		{Datetime: "2025-01-02 10:00:00", Source: "FILE", Host: "WS1", Desc: "f2"},
		{Datetime: "2025-01-03 10:00:00", Source: "REG", Host: "WS1", Desc: "r1"},
		{Datetime: "2025-01-04 10:00:00", Source: "FILE", Host: "WS2", Desc: "f3"},
	}
	if _, err := store.InsertEvents(evidence, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}

	if _, err := store.InsertExaminerNote("2025-01-05 10:00:00", "note A", "", "RED"); err != nil {
		t.Fatalf("InsertExaminerNote A: %v", err)
	}
	idB, err := store.InsertExaminerNote("2025-01-06 10:00:00", "note B", "", "BLUE")
	if err != nil {
		t.Fatalf("InsertExaminerNote B: %v", err)
	}
	// Store IDs are positive; InsertExaminerNote returns the negated grid ID.
	if _, err := store.ToggleExaminerNoteBookmark(-idB); err != nil {
		t.Fatalf("bookmarking note B: %v", err)
	}
	return &App{store: store, driver: "sqlite"}
}

// gridSplit runs req through the grid and splits the rows into evidence count
// and the sorted descriptions of the examiner notes. It checks that the count
// agrees with the rows.
func gridSplit(t *testing.T, app *App, req QueryRequest) (int, []string) {
	t.Helper()
	// Send the same ordering the frontend does.
	req.OrderBy = "datetime"
	resp, err := app.QueryEvents(req)
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	evidence := 0
	var notes []string
	for _, e := range resp.Events {
		if e.ID < 0 {
			notes = append(notes, e.Desc)
		} else {
			evidence++
		}
	}
	sort.Strings(notes)
	if int(resp.TotalCount) != evidence+len(notes) {
		t.Errorf("TotalCount = %d, rows = %d evidence + %d notes", resp.TotalCount, evidence, len(notes))
	}
	return evidence, notes
}

func TestNotesSourceConditionsSimpleMode(t *testing.T) {
	app := newNotesSourceApp(t)
	const (
		a = "note A"
		b = "note B"
	)
	cases := []struct {
		name         string
		req          QueryRequest
		wantEvidence int
		wantNotes    []string
	}{
		{"source = FILE excludes notes",
			QueryRequest{Filters: []FilterItem{{Field: "source", Operator: "=", Value: "FILE"}}}, 3, nil},
		{"source = EXAMINER returns only notes",
			QueryRequest{Filters: []FilterItem{{Field: "source", Operator: "=", Value: "EXAMINER"}}}, 0, []string{a, b}},
		{"source != EXAMINER excludes notes",
			QueryRequest{Filters: []FilterItem{{Field: "source", Operator: "!=", Value: "EXAMINER"}}}, 4, nil},
		{"source != REGISTRY keeps notes",
			QueryRequest{Filters: []FilterItem{{Field: "source", Operator: "!=", Value: "REGISTRY"}}}, 4, []string{a, b}},
		{"source LIKE FILE does not match EXAMINER",
			QueryRequest{Filters: []FilterItem{{Field: "source", Operator: "LIKE", Value: "FILE"}}}, 3, nil},
		{"source NOT LIKE FILE keeps notes",
			QueryRequest{Filters: []FilterItem{{Field: "source", Operator: "NOT LIKE", Value: "FILE"}}}, 1, []string{a, b}},
		{"OR: source = FILE OR color = RED keeps only note A",
			QueryRequest{Logic: "OR", Filters: []FilterItem{
				{Field: "source", Operator: "=", Value: "FILE"},
				{Field: "color", Operator: "=", Value: "RED"},
			}}, 3, []string{a}},
		{"OR: source = EXAMINER OR host = WS1 keeps all notes",
			QueryRequest{Logic: "OR", Filters: []FilterItem{
				{Field: "source", Operator: "=", Value: "EXAMINER"},
				{Field: "host", Operator: "=", Value: "WS1"},
			}}, 3, []string{a, b}},
		{"AND: source = EXAMINER AND color = RED keeps only note A",
			QueryRequest{Logic: "AND", Filters: []FilterItem{
				{Field: "source", Operator: "=", Value: "EXAMINER"},
				{Field: "color", Operator: "=", Value: "RED"},
			}}, 0, []string{a}},
		{"base source = FILE excludes notes",
			QueryRequest{BaseField: "source", BaseOp: "=", BaseValue: "FILE"}, 3, nil},
		{"base source = FILE: an OR filter cannot bring notes back",
			QueryRequest{BaseField: "source", BaseOp: "=", BaseValue: "FILE", Logic: "OR", Filters: []FilterItem{
				{Field: "source", Operator: "=", Value: "FILE"},
				{Field: "color", Operator: "=", Value: "RED"},
			}}, 3, nil},
		{"base source = EXAMINER returns only notes",
			QueryRequest{BaseField: "source", BaseOp: "=", BaseValue: "EXAMINER"}, 0, []string{a, b}},
		{"bookmark-only with source = EXAMINER keeps bookmarked notes",
			QueryRequest{BookmarkOnly: true, Filters: []FilterItem{{Field: "source", Operator: "=", Value: "EXAMINER"}}}, 0, []string{b}},
		{"compatible mapping: color = BLUE keeps note B",
			QueryRequest{Filters: []FilterItem{{Field: "color", Operator: "=", Value: "BLUE"}}}, 0, []string{b}},
		{"host filter excludes notes (note host is NULL)",
			QueryRequest{Filters: []FilterItem{{Field: "host", Operator: "=", Value: "WS1"}}}, 3, nil},
		{"OR: host = WS1 OR color = RED keeps note A",
			QueryRequest{Logic: "OR", Filters: []FilterItem{
				{Field: "host", Operator: "=", Value: "WS1"},
				{Field: "color", Operator: "=", Value: "RED"},
			}}, 3, []string{a}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evidence, notes := gridSplit(t, app, tc.req)
			if evidence != tc.wantEvidence {
				t.Errorf("evidence rows = %d, want %d", evidence, tc.wantEvidence)
			}
			if !reflect.DeepEqual(notes, tc.wantNotes) {
				t.Errorf("notes = %v, want %v", notes, tc.wantNotes)
			}
		})
	}
}

func TestNotesSourceExportMatchesGrid(t *testing.T) {
	app := newNotesSourceApp(t)

	// Base FILE: no notes, and the export bytes match the grid.
	base := QueryRequest{BaseField: "source", BaseOp: "=", BaseValue: "FILE"}
	if total := assertExportMatchesGrid(t, app, base); total != 3 {
		t.Errorf("grid total under base FILE = %d, want 3", total)
	}

	// OR with an always-true source term: every note is in the export, and the
	// bytes match the grid.
	orAll := QueryRequest{Logic: "OR", Filters: []FilterItem{
		{Field: "source", Operator: "=", Value: "EXAMINER"},
		{Field: "host", Operator: "=", Value: "WS1"},
	}}
	if total := assertExportMatchesGrid(t, app, orAll); total != 5 {
		t.Errorf("grid total for OR(source=EXAMINER, host=WS1) = %d, want 5 (3 WS1 rows + 2 notes)", total)
	}
}
