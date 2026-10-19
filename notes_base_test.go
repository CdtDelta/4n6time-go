package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// newNotesBaseApp seeds evidence and three examiner notes with descriptions,
// tags, and colors, so each compatible base field can be tested.
//
//	evidence: E1 FILE WS1 "malware.exe", E2 FILE WS1 "benign.txt", E3 REG WS1 "other"
//	note N1: "malware dropper note", tag t1, RED
//	note N2: "benign note",          tag t2, BLUE
//	note N3: "malware beacon",       tag t1, BLUE
func newNotesBaseApp(t *testing.T) *App {
	t.Helper()
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "notesbase.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	evidence := []*model.Event{
		{Datetime: "2025-01-01 10:00:00", Source: "FILE", Host: "WS1", Desc: "malware.exe"},
		{Datetime: "2025-01-02 10:00:00", Source: "FILE", Host: "WS1", Desc: "benign.txt"},
		{Datetime: "2025-01-03 10:00:00", Source: "REG", Host: "WS1", Desc: "other"},
	}
	if _, err := store.InsertEvents(evidence, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	notes := [][4]string{
		{"2025-01-04 10:00:00", "malware dropper note", "t1", "RED"},
		{"2025-01-05 10:00:00", "benign note", "t2", "BLUE"},
		{"2025-01-06 10:00:00", "malware beacon", "t1", "BLUE"},
	}
	for _, n := range notes {
		if _, err := store.InsertExaminerNote(n[0], n[1], n[2], n[3]); err != nil {
			t.Fatalf("InsertExaminerNote: %v", err)
		}
	}
	return &App{store: store, driver: "sqlite"}
}

func TestBaseOnCompatibleFieldsAppliesToNotes(t *testing.T) {
	app := newNotesBaseApp(t)
	const (
		dropper = "malware dropper note"
		beacon  = "malware beacon"
		benign  = "benign note"
	)
	cases := []struct {
		name         string
		req          QueryRequest
		wantEvidence int
		wantNotes    []string
	}{
		{"base desc LIKE malware: matching notes and evidence",
			QueryRequest{BaseField: "desc", BaseOp: "LIKE", BaseValue: "malware"},
			1, []string{beacon, dropper}},
		{"base desc = exact text: only that note",
			QueryRequest{BaseField: "desc", BaseOp: "=", BaseValue: "benign note"},
			0, []string{benign}},
		{"base desc LIKE malware with OR group: base still restricts notes",
			QueryRequest{BaseField: "desc", BaseOp: "LIKE", BaseValue: "malware", Logic: "OR", Filters: []FilterItem{
				{Field: "source", Operator: "=", Value: "FILE"},
				{Field: "color", Operator: "=", Value: "BLUE"},
			}},
			1, []string{beacon}},
		{"base desc LIKE malware with a satisfied OR group: base still restricts notes",
			QueryRequest{BaseField: "desc", BaseOp: "LIKE", BaseValue: "malware", Logic: "OR", Filters: []FilterItem{
				{Field: "source", Operator: "=", Value: "EXAMINER"},
				{Field: "color", Operator: "=", Value: "RED"},
			}},
			0, []string{beacon, dropper}},
		{"base tag = t1: notes with that tag",
			QueryRequest{BaseField: "tag", BaseOp: "=", BaseValue: "t1"},
			0, []string{beacon, dropper}},
		{"base color = BLUE: notes with that color",
			QueryRequest{BaseField: "color", BaseOp: "=", BaseValue: "BLUE"},
			0, []string{benign, beacon}},
		{"base host still excludes notes",
			QueryRequest{BaseField: "host", BaseOp: "=", BaseValue: "WS1"},
			3, nil},
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

func TestSimpleExportDescBaseMatchesGrid(t *testing.T) {
	app := newNotesBaseApp(t)
	req := QueryRequest{BaseField: "desc", BaseOp: "LIKE", BaseValue: "malware"}
	if total := assertExportMatchesGrid(t, app, req); total != 3 {
		t.Errorf("grid total under desc base = %d, want 3 (1 evidence + 2 notes)", total)
	}
}

// The notes-side base condition must number its placeholders after the bookmark
// term and before the filter group, with arguments in the same order, so that
// PostgreSQL $N values line up with the args.
func TestNotesBaseConditionPostgresPlaceholders(t *testing.T) {
	pg := &database.PostgresDialect{}
	req := QueryRequest{
		BaseField: "desc", BaseOp: "LIKE", BaseValue: "malware",
		Logic: "OR",
		Filters: []FilterItem{
			{Field: "color", Operator: "=", Value: "BLUE"},
		},
	}
	// startIdx 4 means the main query used three placeholders.
	exclude, nf := simpleNotesPlan(req, pg, 4)
	if exclude || nf == nil {
		t.Fatalf("simpleNotesPlan = exclude %v, filter %+v; want a notes filter", exclude, nf)
	}
	wantSQL := "description LIKE $4 AND (color = $5)"
	if nf.SQL != wantSQL {
		t.Errorf("SQL = %q, want %q", nf.SQL, wantSQL)
	}
	if len(nf.Args) != 2 || nf.Args[0] != "%malware%" || nf.Args[1] != "BLUE" {
		t.Errorf("Args = %#v, want [%%malware%% BLUE]", nf.Args)
	}
}
