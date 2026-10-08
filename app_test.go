package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/csvparser"
	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// newTestSQLiteApp returns an App backed by a fresh SQLite database holding
// one event. No Wails context is set, so only methods that do not touch the
// runtime may be called.
func newTestSQLiteApp(t *testing.T) (*App, database.Store) {
	t.Helper()
	store, err := database.CreateStore("sqlite", filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatalf("creating SQLite store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.InsertEvent(&model.Event{
		Datetime: "2026-01-01 00:00:00",
		Source:   "FILE",
		Desc:     "test event",
	}); err != nil {
		t.Fatalf("inserting event: %v", err)
	}
	return &App{store: store, driver: "sqlite"}, store
}

// assertTableIntact confirms log2timeline still exists and still holds the
// seeded event.
func assertTableIntact(t *testing.T, store database.Store) {
	t.Helper()
	count, err := store.CountEvents("", nil)
	if err != nil {
		t.Fatalf("log2timeline no longer queryable: %v", err)
	}
	if count != 1 {
		t.Errorf("log2timeline row count = %d, want 1", count)
	}
}

func TestAdvancedSearchRejectsSemicolonOnSQLite(t *testing.T) {
	app, store := newTestSQLiteApp(t)

	attacks := []string{
		"1=1; DROP TABLE log2timeline",
		"1=1 --'\n; DROP TABLE log2timeline; --",
		"1=1 /* ' */ ; DROP TABLE log2timeline",
	}
	for _, clause := range attacks {
		resp, err := app.AdvancedSearch(clause, 1, 100)
		if err == nil {
			t.Errorf("AdvancedSearch(%q) succeeded with %+v, want validation error", clause, resp)
			continue
		}
		if !strings.Contains(err.Error(), "not permitted in advanced search clauses") {
			t.Errorf("AdvancedSearch(%q) error = %q, want validation error", clause, err.Error())
		}
		assertTableIntact(t, store)
	}

	// A legitimate clause still runs against the intact table.
	resp, err := app.AdvancedSearch("source = 'FILE'", 1, 100)
	if err != nil {
		t.Fatalf("AdvancedSearch(valid clause): %v", err)
	}
	if resp.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", resp.TotalCount)
	}
}

func TestTimelineHistogramRejectsSemicolonOnSQLite(t *testing.T) {
	app, store := newTestSQLiteApp(t)

	_, err := app.GetTimelineHistogram(QueryRequest{
		SearchText: "1=1; DROP TABLE log2timeline",
		SearchMode: "advanced",
	})
	if err == nil {
		t.Fatal("GetTimelineHistogram accepted a semicolon in advanced mode, want validation error")
	}
	if !strings.Contains(err.Error(), "semicolons are not permitted") {
		t.Errorf("GetTimelineHistogram error = %q, want semicolon validation error", err.Error())
	}
	assertTableIntact(t, store)
}

// newExportTestApp returns an App backed by a SQLite database seeded with
// events on two hosts plus one examiner note. Datetimes are distinct so the
// datetime ordering is deterministic across pages and the single export query.
func newExportTestApp(t *testing.T) *App {
	t.Helper()
	store, err := database.CreateStore("sqlite", filepath.Join(t.TempDir(), "export.db"), nil)
	if err != nil {
		t.Fatalf("creating SQLite store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	seed := []*model.Event{
		{Datetime: "2026-01-01 00:00:01", Source: "FILE", Host: "WS1", Desc: "evil.exe created"},
		{Datetime: "2026-01-01 00:00:02", Source: "REG", Host: "WS1", Desc: "evil run key"},
		{Datetime: "2026-01-01 00:00:03", Source: "FILE", Host: "WS1", Desc: "benign.txt created"},
		{Datetime: "2026-01-01 00:00:04", Source: "FILE", Host: "WS2", Desc: "evil.dll loaded"},
		{Datetime: "2026-01-01 00:00:05", Source: "EVT", Host: "WS2", Desc: "logon; type 3"},
		{Datetime: "2026-01-01 00:00:07", Source: "FILE", Host: "WS1", Desc: "evil.ps1 written"},
	}
	for _, e := range seed {
		if err := store.InsertEvent(e); err != nil {
			t.Fatalf("inserting event: %v", err)
		}
	}
	if _, err := store.InsertExaminerNote("2026-01-01 00:00:06", "analyst note on evil", "", ""); err != nil {
		t.Fatalf("inserting examiner note: %v", err)
	}
	return &App{store: store, driver: "sqlite"}
}

// writeEventsCSV renders events with the same writer ExportCSV uses, so the
// result can be compared byte for byte with an export file.
func writeEventsCSV(t *testing.T, events []*model.Event) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "expected.csv")
	if err := csvparser.WriteEvents(path, events); err != nil {
		t.Fatalf("writing expected CSV: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading expected CSV: %v", err)
	}
	return data
}

func hasExaminerRow(events []*model.Event) bool {
	for _, e := range events {
		if e.Source == "EXAMINER" {
			return true
		}
	}
	return false
}

func TestExportCSVAdvancedMatchesGrid(t *testing.T) {
	tests := []struct {
		name         string
		clause       string
		baseField    string
		baseOp       string
		baseValue    string
		wantExaminer bool
	}{
		{
			name:         "no base query, notes included",
			clause:       "desc LIKE '%evil%'",
			wantExaminer: true,
		},
		{
			name:         "base query, notes included",
			clause:       "desc LIKE '%evil%'",
			baseField:    "host",
			baseOp:       "=",
			baseValue:    "WS1",
			wantExaminer: true,
		},
		{
			name:         "source filter, notes excluded",
			clause:       "source = 'FILE' AND desc LIKE '%evil%'",
			wantExaminer: false,
		},
		{
			name:         "base query plus source filter, notes excluded",
			clause:       "source = 'FILE' OR desc LIKE '%run%'",
			baseField:    "host",
			baseOp:       "LIKE",
			baseValue:    "WS1",
			wantExaminer: false,
		},
		{
			name:         "explicit examiner source, notes included",
			clause:       "source = 'EXAMINER' OR source = 'REG'",
			wantExaminer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newExportTestApp(t)

			// The grid's clause, built the way App.jsx builds it before
			// calling AdvancedSearch.
			gridClause := tt.clause
			if tt.baseField != "" {
				safeVal := strings.ReplaceAll(tt.baseValue, "'", "''")
				gridClause = tt.baseField + " " + tt.baseOp + " '" + safeVal + "' AND (" + tt.clause + ")"
			}

			// Walk every page with a small page size so paging is exercised.
			const pageSize = 2
			var gridEvents []*model.Event
			for page := 1; ; page++ {
				resp, err := app.AdvancedSearch(gridClause, page, pageSize)
				if err != nil {
					t.Fatalf("AdvancedSearch page %d: %v", page, err)
				}
				gridEvents = append(gridEvents, resp.Events...)
				if int64(page*pageSize) >= resp.TotalCount {
					if int64(len(gridEvents)) != resp.TotalCount {
						t.Fatalf("collected %d grid rows, TotalCount %d", len(gridEvents), resp.TotalCount)
					}
					break
				}
			}
			if len(gridEvents) == 0 {
				t.Fatal("grid returned no rows; fixture does not exercise the export")
			}
			if got := hasExaminerRow(gridEvents); got != tt.wantExaminer {
				t.Errorf("grid examiner note present = %v, want %v", got, tt.wantExaminer)
			}

			exportPath := filepath.Join(t.TempDir(), "export.csv")
			msg, err := app.exportCSVTo(exportPath, QueryRequest{
				OrderBy:    "datetime",
				SearchText: tt.clause,
				SearchMode: "advanced",
				BaseField:  tt.baseField,
				BaseOp:     tt.baseOp,
				BaseValue:  tt.baseValue,
			}, func(string) {})
			if err != nil {
				t.Fatalf("exportCSVTo: %v", err)
			}
			if !strings.Contains(msg, "Exported") {
				t.Errorf("export message = %q", msg)
			}

			got, err := os.ReadFile(exportPath)
			if err != nil {
				t.Fatalf("reading export: %v", err)
			}
			want := writeEventsCSV(t, gridEvents)
			if !bytes.Equal(got, want) {
				t.Errorf("export does not match grid rows\n--- export ---\n%s\n--- grid ---\n%s", got, want)
			}
		})
	}
}

func TestExportCSVAdvancedRejectsSemicolon(t *testing.T) {
	app := newExportTestApp(t)
	exportPath := filepath.Join(t.TempDir(), "export.csv")

	_, err := app.exportCSVTo(exportPath, QueryRequest{
		SearchText: "1=1; DROP TABLE log2timeline",
		SearchMode: "advanced",
	}, func(string) {})
	if err == nil {
		t.Fatal("exportCSVTo accepted a semicolon in advanced mode, want validation error")
	}
	if !strings.Contains(err.Error(), "semicolons are not permitted") {
		t.Errorf("exportCSVTo error = %q, want semicolon validation error", err.Error())
	}
	if _, statErr := os.Stat(exportPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("export file exists after rejected query (stat err: %v)", statErr)
	}
}

// TestExportCSVSimpleUnchanged guards the keyword export path: it must keep
// matching the grid's QueryEvents rows, keep treating search text as a
// literal keyword (so a semicolon is fine), and ignore the base query as it
// always has.
func TestExportCSVSimpleUnchanged(t *testing.T) {
	tests := []struct {
		name       string
		searchText string
		searchMode string
		wantRows   int
	}{
		// Keyword search does not filter examiner notes (buildNotesFilter
		// ignores SearchText), so the one note appears in every result below.
		// 4 events match "evil", plus the note.
		{"keyword, simple mode", "evil", "simple", 5},
		// Requests from before the frontend sent searchMode.
		{"keyword, mode unset", "evil", "", 5},
		// 1 event matches, plus the note.
		{"keyword containing semicolon", "logon; type", "simple", 2},
		// Advanced mode with no clause falls back to the simple path, like
		// the grid: all 6 events plus the note.
		{"advanced mode, empty clause", "", "advanced", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newExportTestApp(t)
			req := QueryRequest{
				Logic:      "AND",
				OrderBy:    "datetime",
				Page:       1,
				PageSize:   1000,
				SearchText: tt.searchText,
				SearchMode: tt.searchMode,
			}

			grid, err := app.QueryEvents(req)
			if err != nil {
				t.Fatalf("QueryEvents: %v", err)
			}

			exportPath := filepath.Join(t.TempDir(), "export.csv")
			if _, err := app.exportCSVTo(exportPath, req, func(string) {}); err != nil {
				t.Fatalf("exportCSVTo: %v", err)
			}
			got, err := os.ReadFile(exportPath)
			if err != nil {
				t.Fatalf("reading export: %v", err)
			}

			if !bytes.Equal(got, writeEventsCSV(t, grid.Events)) {
				t.Errorf("simple export does not match QueryEvents rows\n%s", got)
			}
			// Header line plus one line per row.
			if lines := strings.Count(string(got), "\n"); lines != tt.wantRows+1 {
				t.Errorf("export has %d lines, want %d (header + %d rows)", lines, tt.wantRows+1, tt.wantRows)
			}
			if !strings.HasPrefix(string(got), "datetime,timezone,MACB,source,") {
				t.Errorf("export header changed: %q", strings.SplitN(string(got), "\n", 2)[0])
			}
		})
	}
}

// newHistogramTestApp returns an App backed by a SQLite database seeded with
// events across two hosts, some bookmarked. It holds no examiner notes: the
// store's histogram query never counts notes in any mode, while the grid's
// UNION can include them, so notes would make the totals differ for a reason
// unrelated to filter handling.
func newHistogramTestApp(t *testing.T) *App {
	t.Helper()
	store, err := database.CreateStore("sqlite", filepath.Join(t.TempDir(), "histogram.db"), nil)
	if err != nil {
		t.Fatalf("creating SQLite store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	seed := []*model.Event{
		{Datetime: "2026-01-01 00:00:01", Source: "FILE", Host: "WS1", Desc: "evil.exe created", Bookmark: 1},
		{Datetime: "2026-01-01 00:00:02", Source: "REG", Host: "WS1", Desc: "evil run key"},
		{Datetime: "2026-01-01 00:00:03", Source: "FILE", Host: "WS1", Desc: "benign.txt created"},
		{Datetime: "2026-01-02 00:00:04", Source: "FILE", Host: "WS2", Desc: "evil.dll loaded", Bookmark: 1},
		{Datetime: "2026-01-02 00:00:05", Source: "EVT", Host: "WS2", Desc: "logon type 3"},
		{Datetime: "2026-01-03 00:00:07", Source: "FILE", Host: "WS1", Desc: "evil.ps1 written"},
	}
	for _, e := range seed {
		if err := store.InsertEvent(e); err != nil {
			t.Fatalf("inserting event: %v", err)
		}
	}
	return &App{store: store, driver: "sqlite"}
}

// histogramTotal sums every bucket the histogram returns for req.
func histogramTotal(t *testing.T, app *App, req QueryRequest) int64 {
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

// TestAdvancedHistogramIgnoresFilterPanelAndBookmark verifies the advanced
// histogram applies only the clause, matching AdvancedSearch, even when the
// request also carries bookmark-only, a filter panel filter, and a date range.
func TestAdvancedHistogramIgnoresFilterPanelAndBookmark(t *testing.T) {
	app := newHistogramTestApp(t)
	const clause = "desc LIKE '%evil%'"

	grid, err := app.AdvancedSearch(clause, 1, 100)
	if err != nil {
		t.Fatalf("AdvancedSearch: %v", err)
	}
	if grid.TotalCount != 4 {
		t.Fatalf("AdvancedSearch TotalCount = %d, want 4 (fixture check)", grid.TotalCount)
	}

	got := histogramTotal(t, app, QueryRequest{
		Logic:        "AND",
		SearchText:   clause,
		SearchMode:   "advanced",
		BookmarkOnly: true,
		Filters: []FilterItem{
			{Field: "host", Operator: "=", Value: "WS2"},
			{Field: "datetime", Operator: ">=", Value: "2026-01-02"},
			{Field: "datetime", Operator: "<=", Value: "2026-01-02"},
		},
	})
	// With the filters and bookmark-only applied the total would be 1
	// (evil.dll on WS2, bookmarked, 2026-01-02).
	if got != grid.TotalCount {
		t.Errorf("advanced histogram total = %d, want %d (AdvancedSearch total)", got, grid.TotalCount)
	}
}

// TestAdvancedHistogramAppliesBaseQuery verifies a pivot tab's base query
// still narrows the advanced histogram, matching the grid's prefixed clause.
func TestAdvancedHistogramAppliesBaseQuery(t *testing.T) {
	app := newHistogramTestApp(t)
	const clause = "desc LIKE '%evil%'"

	// The grid's clause, prefixed the way App.jsx prefixes it for a pivot tab.
	grid, err := app.AdvancedSearch("host = 'WS1' AND ("+clause+")", 1, 100)
	if err != nil {
		t.Fatalf("AdvancedSearch: %v", err)
	}
	if grid.TotalCount != 3 {
		t.Fatalf("AdvancedSearch TotalCount = %d, want 3 (fixture check)", grid.TotalCount)
	}

	got := histogramTotal(t, app, QueryRequest{
		Logic:        "AND",
		SearchText:   clause,
		SearchMode:   "advanced",
		BookmarkOnly: true,
		BaseField:    "host",
		BaseOp:       "=",
		BaseValue:    "WS1",
	})
	if got != grid.TotalCount {
		t.Errorf("advanced histogram total with base query = %d, want %d", got, grid.TotalCount)
	}
}

// TestSimpleHistogramBookmarkOnlyStillFilters guards simple mode: the
// bookmark-only toggle and filter panel filters still narrow the histogram.
func TestSimpleHistogramBookmarkOnlyStillFilters(t *testing.T) {
	app := newHistogramTestApp(t)

	if got := histogramTotal(t, app, QueryRequest{Logic: "AND", SearchMode: "simple", BookmarkOnly: true}); got != 2 {
		t.Errorf("simple histogram bookmark-only total = %d, want 2", got)
	}
	if got := histogramTotal(t, app, QueryRequest{
		Logic:        "AND",
		SearchMode:   "simple",
		BookmarkOnly: true,
		Filters:      []FilterItem{{Field: "host", Operator: "=", Value: "WS2"}},
	}); got != 1 {
		t.Errorf("simple histogram bookmark-only + host filter total = %d, want 1", got)
	}
	// Advanced mode with an empty clause falls back to the simple path, as
	// the grid does, so bookmark-only still applies.
	if got := histogramTotal(t, app, QueryRequest{Logic: "AND", SearchMode: "advanced", BookmarkOnly: true}); got != 2 {
		t.Errorf("advanced mode with empty clause, bookmark-only total = %d, want 2", got)
	}
}
