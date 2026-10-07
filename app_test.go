package main

import (
	"path/filepath"
	"strings"
	"testing"

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
