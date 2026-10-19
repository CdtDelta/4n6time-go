package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertExportMatchesGrid runs req through the grid (QueryEvents) and through
// the simple-mode export, then compares the two as bytes. Both requests order
// by datetime, as the advanced parity test does, so the row order is the same.
// It returns the grid's total count.
func assertExportMatchesGrid(t *testing.T, app *App, req QueryRequest) int64 {
	t.Helper()
	req.OrderBy = "datetime"

	grid, err := app.QueryEvents(req)
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	want := writeEventsCSV(t, app, grid.Events)

	path := filepath.Join(t.TempDir(), "export.csv")
	if _, err := app.exportCSVTo(path, req, func(string) {}); err != nil {
		t.Fatalf("exportCSVTo: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("export does not match grid\n--- export ---\n%s\n--- grid ---\n%s", got, want)
	}
	return grid.TotalCount
}

func TestSimpleExportBaseQueryANDedOutsideOr(t *testing.T) {
	app := newExportTestApp(t)
	// Base FILE, then host=WS2 OR source=REG. The base must AND with the OR
	// group, leaving only the FILE event from WS2 (evil.dll loaded). Notes are
	// excluded because host is a field notes lack.
	req := QueryRequest{
		BaseField: "source", BaseOp: "=", BaseValue: "FILE",
		Filters: []FilterItem{
			{Field: "host", Operator: "=", Value: "WS2"},
			{Field: "source", Operator: "=", Value: "REG"},
		},
		Logic: "OR",
	}
	if total := assertExportMatchesGrid(t, app, req); total != 1 {
		t.Errorf("grid total = %d, want 1 (base AND (host OR source))", total)
	}
}

func TestSimpleExportBaseExcludesNotesLikeGrid(t *testing.T) {
	app := newExportTestApp(t)
	// host is not an examiner-note field, so the base excludes notes. The grid
	// has 4 WS1 events and no note, and the export must match that.
	req := QueryRequest{BaseField: "host", BaseOp: "=", BaseValue: "WS1"}
	if total := assertExportMatchesGrid(t, app, req); total != 4 {
		t.Errorf("grid total = %d, want 4 WS1 events", total)
	}

	path := filepath.Join(t.TempDir(), "notes.csv")
	req.OrderBy = "datetime"
	if _, err := app.exportCSVTo(path, req, func(string) {}); err != nil {
		t.Fatalf("exportCSVTo: %v", err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "analyst note") {
		t.Error("export includes the examiner note under a host base query")
	}
}

func TestSimpleExportNoBaseUnchangedParity(t *testing.T) {
	app := newExportTestApp(t)
	// Empty base (the All Events tab): the export matches the grid, notes included.
	if total := assertExportMatchesGrid(t, app, QueryRequest{}); total != 7 {
		t.Errorf("grid total with no base = %d, want 7 (6 events + note)", total)
	}
}

func TestSimpleExportImportBaseReturnsOnlyThatImport(t *testing.T) {
	fx := newImportFilterFixture(t)
	req := QueryRequest{BaseField: "import_id", BaseOp: "=", BaseValue: itoa(fx.idA)}
	if total := assertExportMatchesGrid(t, fx.app, req); total != 3 {
		t.Errorf("grid total under import base = %d, want 3", total)
	}

	path := filepath.Join(t.TempDir(), "import-base.csv")
	req.OrderBy = "datetime"
	if _, err := fx.app.exportCSVTo(path, req, func(string) {}); err != nil {
		t.Fatalf("exportCSVTo: %v", err)
	}
	data, _ := os.ReadFile(path)
	if n := strings.Count(string(data), ",a.jsonl"); n != 3 {
		t.Errorf("export has %d rows from a.jsonl, want 3", n)
	}

	bad := QueryRequest{BaseField: "import_id", BaseOp: "LIKE", BaseValue: itoa(fx.idA)}
	if _, err := fx.app.exportCSVTo(filepath.Join(t.TempDir(), "bad.csv"), bad, func(string) {}); err == nil {
		t.Error("export accepted a LIKE base on import_id")
	}
}
