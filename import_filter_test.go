package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// importFilterFixture holds the import IDs of the seeded data set:
//
//	import A (a.jsonl): 3 FILE events
//	import B (b.csv):   1 FILE and 2 REG events
//	no import:          1 FILE event (import_id NULL)
//	examiner note:      1 note (never counted under an import filter)
//
// FILE totals 5 events, REG totals 2.
type importFilterFixture struct {
	app *App
	idA int64
	idB int64
}

func newImportFilterFixture(t *testing.T) importFilterFixture {
	t.Helper()
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "filter.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	idA, err := store.CreateImport(model.ImportRecord{Filename: "a.jsonl", Label: "WKSTN04", Format: "Plaso JSONL"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinalizeImport(idA, 3, model.ImportStatusComplete, ""); err != nil {
		t.Fatal(err)
	}
	idB, err := store.CreateImport(model.ImportRecord{Filename: "b.csv", Format: "LECmd"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinalizeImport(idB, 3, model.ImportStatusComplete, ""); err != nil {
		t.Fatal(err)
	}

	ev := func(source string, importID int64, n int) []*model.Event {
		var out []*model.Event
		for i := 0; i < n; i++ {
			out = append(out, &model.Event{
				Source:   source,
				Datetime: fmt.Sprintf("2025-02-0%d 10:00:00", i+1),
				Desc:     source + " event",
				ImportID: importID,
			})
		}
		return out
	}
	var events []*model.Event
	events = append(events, ev("FILE", idA, 3)...)
	events = append(events, ev("FILE", idB, 1)...)
	events = append(events, ev("REG", idB, 2)...)
	events = append(events, ev("FILE", 0, 1)...)
	if _, err := store.InsertEvents(events, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	if _, err := store.InsertExaminerNote("2025-02-05 10:00:00", "examiner note", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote: %v", err)
	}
	return importFilterFixture{
		app: &App{store: store, driver: "sqlite"},
		idA: idA,
		idB: idB,
	}
}

// importHistogramTotal sums the histogram buckets for req.
func importHistogramTotal(t *testing.T, app *App, req QueryRequest) int64 {
	t.Helper()
	buckets, err := app.GetTimelineHistogram(req)
	if err != nil {
		t.Fatalf("GetTimelineHistogram: %v", err)
	}
	var total int64
	for _, b := range buckets {
		total += b.Count
	}
	return total
}

func TestImportIDValueRules(t *testing.T) {
	if n, err := importIDValue("=", " 42 "); err != nil || n != 42 {
		t.Errorf("importIDValue(=, ' 42 ') = %d, %v; want 42", n, err)
	}
	for _, tc := range []struct{ op, value string }{
		{"=", "abc"},
		{"=", ""},
		{"=", "1.5"},
		{"!=", "5"},
		{"LIKE", "5"},
	} {
		if _, err := importIDValue(tc.op, tc.value); err == nil {
			t.Errorf("importIDValue(%q, %q) accepted; want error", tc.op, tc.value)
		}
	}
}

func TestGetFilteredDistinctValuesImportScope(t *testing.T) {
	fx := newImportFilterFixture(t)

	// Under a FILE base, both imports have FILE events; the NULL import is dropped.
	got, err := fx.app.GetFilteredDistinctValues("import_id", "source", "=", "FILE")
	if err != nil {
		t.Fatalf("GetFilteredDistinctValues: %v", err)
	}
	wantFile := map[string]int64{itoa(fx.idA): 3, itoa(fx.idB): 1}
	if !sameCounts(got, wantFile) {
		t.Errorf("FILE scope = %v, want %v", got, wantFile)
	}

	// Under a REG base only import B has events.
	got, err = fx.app.GetFilteredDistinctValues("import_id", "source", "=", "REG")
	if err != nil {
		t.Fatalf("GetFilteredDistinctValues(REG): %v", err)
	}
	if !sameCounts(got, map[string]int64{itoa(fx.idB): 2}) {
		t.Errorf("REG scope = %v, want only import B", got)
	}

	// Unscoped distinct values also drop the NULL import.
	got, err = fx.app.GetDistinctValues("import_id")
	if err != nil {
		t.Fatalf("GetDistinctValues: %v", err)
	}
	if !sameCounts(got, map[string]int64{itoa(fx.idA): 3, itoa(fx.idB): 3}) {
		t.Errorf("unscoped imports = %v, want A:3 B:3 with no NULL entry", got)
	}
}

func TestImportFilterGridCountAndNotes(t *testing.T) {
	fx := newImportFilterFixture(t)
	importA := FilterItem{Field: "import_id", Operator: "=", Value: itoa(fx.idA)}
	fileSource := FilterItem{Field: "source", Operator: "=", Value: "FILE"}

	cases := []struct {
		name      string
		req       QueryRequest
		wantTotal int64
	}{
		{"import alone", QueryRequest{Filters: []FilterItem{importA}}, 3},
		{"import AND source", QueryRequest{Filters: []FilterItem{importA, fileSource}, Logic: "AND"}, 3},
		{"import OR source", QueryRequest{Filters: []FilterItem{importA, fileSource}, Logic: "OR"}, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := fx.app.QueryEvents(tc.req)
			if err != nil {
				t.Fatalf("QueryEvents: %v", err)
			}
			if resp.TotalCount != tc.wantTotal {
				t.Errorf("grid total = %d, want %d", resp.TotalCount, tc.wantTotal)
			}
			if got := importHistogramTotal(t, fx.app, tc.req); got != tc.wantTotal {
				t.Errorf("histogram total = %d, want %d (must match grid)", got, tc.wantTotal)
			}
			// Examiner notes are excluded whenever the import filter is present.
			for _, e := range resp.Events {
				if e.ID < 0 {
					t.Errorf("examiner note %d returned under an import filter", e.ID)
				}
			}
		})
	}
}

func TestImportFilterExcludesNotesEvenAlongsideCompatibleFilters(t *testing.T) {
	fx := newImportFilterFixture(t)
	// A datetime filter alone would keep notes; the import filter must still drop them.
	req := QueryRequest{Filters: []FilterItem{
		{Field: "datetime", Operator: ">=", Value: "2025-01-01 00:00:00"},
		{Field: "import_id", Operator: "=", Value: itoa(fx.idA)},
	}}
	resp, err := fx.app.QueryEvents(req)
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if resp.TotalCount != 3 {
		t.Errorf("total = %d, want 3 (no note)", resp.TotalCount)
	}
	for _, e := range resp.Events {
		if e.ID < 0 {
			t.Errorf("examiner note %d returned", e.ID)
		}
	}
}

func TestImportFilterBaseQueryRules(t *testing.T) {
	fx := newImportFilterFixture(t)

	resp, err := fx.app.QueryEvents(QueryRequest{BaseField: "import_id", BaseOp: "=", BaseValue: itoa(fx.idA)})
	if err != nil {
		t.Fatalf("base import_id = accepted: %v", err)
	}
	if resp.TotalCount != 3 {
		t.Errorf("base import total = %d, want 3", resp.TotalCount)
	}
	if got := importHistogramTotal(t, fx.app, QueryRequest{BaseField: "import_id", BaseOp: "=", BaseValue: itoa(fx.idA)}); got != 3 {
		t.Errorf("histogram under base = %d, want 3", got)
	}

	rejected := []QueryRequest{
		{BaseField: "import_id", BaseOp: "LIKE", BaseValue: itoa(fx.idA)},
		{BaseField: "import_id", BaseOp: "!=", BaseValue: itoa(fx.idA)},
		{BaseField: "import_id", BaseOp: "=", BaseValue: "abc"},
		{Filters: []FilterItem{{Field: "import_id", Operator: "LIKE", Value: "1"}}},
		{Filters: []FilterItem{{Field: "import_id", Operator: "=", Value: "x"}}},
	}
	for i, req := range rejected {
		if _, err := fx.app.QueryEvents(req); err == nil {
			t.Errorf("case %d: QueryEvents accepted an invalid import_id value", i)
		}
	}
	if _, err := fx.app.GetFilteredDistinctValues("source", "import_id", "LIKE", "1"); err == nil {
		t.Error("GetFilteredDistinctValues accepted a LIKE base on import_id")
	}
	if _, err := fx.app.GetFilteredDistinctValues("source", "import_id", "=", "nope"); err == nil {
		t.Error("GetFilteredDistinctValues accepted a non-numeric import base")
	}
}

func TestImportFilterSimpleExportRows(t *testing.T) {
	fx := newImportFilterFixture(t)
	path := filepath.Join(t.TempDir(), "import.csv")
	req := QueryRequest{Filters: []FilterItem{{Field: "import_id", Operator: "=", Value: itoa(fx.idA)}}}

	if _, err := fx.app.exportCSVTo(path, req, func(string) {}); err != nil {
		t.Fatalf("exportCSVTo: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}
	if len(rows) != 1+3 {
		t.Fatalf("export has %d rows, want header plus 3", len(rows))
	}
	last := len(rows[0]) - 1
	if rows[0][last] != "import_file" {
		t.Fatalf("last header = %q, want import_file", rows[0][last])
	}
	for _, r := range rows[1:] {
		if r[last] != "a.jsonl" {
			t.Errorf("import_file = %q, want a.jsonl", r[last])
		}
	}

	bad := QueryRequest{Filters: []FilterItem{{Field: "import_id", Operator: "=", Value: "abc"}}}
	if _, err := fx.app.exportCSVTo(filepath.Join(t.TempDir(), "bad.csv"), bad, func(string) {}); err == nil {
		t.Error("export accepted a non-numeric import id")
	}
}

func TestAdvancedBaseClauseImportID(t *testing.T) {
	got, err := advancedClauseWithBase("source = 'FILE'", "import_id", "=", "3")
	if err != nil {
		t.Fatalf("advancedClauseWithBase: %v", err)
	}
	if want := "import_id = 3 AND (source = 'FILE')"; got != want {
		t.Errorf("clause = %q, want %q", got, want)
	}
	if _, err := advancedClauseWithBase("x", "import_id", "=", "three"); err == nil {
		t.Error("non-numeric import base accepted in advanced clause")
	}
	if _, err := advancedClauseWithBase("x", "import_id", "LIKE", "3"); err == nil {
		t.Error("LIKE accepted for import base in advanced clause")
	}
}

// itoa formats an import id for a filter value.
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

// sameCounts reports whether two distinct-value maps hold the same keys and counts.
func sameCounts(got, want map[string]int64) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}
