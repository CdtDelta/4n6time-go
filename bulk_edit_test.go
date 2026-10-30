package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// beCountUpdates records the UPDATE statements bulk edits run, by table.
func beCountUpdates(t *testing.T) map[string]int {
	t.Helper()
	counts := map[string]int{}
	database.BulkUpdateHook = func(q string) {
		for _, table := range []string{"log2timeline", "examiner_notes"} {
			if strings.HasPrefix(q, "UPDATE "+table+" ") {
				counts[table]++
			}
		}
	}
	t.Cleanup(func() { database.BulkUpdateHook = nil })
	return counts
}

// beIDs returns the evidence IDs in insert order.
func beIDs(t *testing.T, store database.Store) []int64 {
	t.Helper()
	events, err := store.QueryEvents("", nil, "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// beStored reads one row's tag and color straight from the table.
func beStored(t *testing.T, store ifStore, backend, table string, id int64) (string, string) {
	t.Helper()
	idCol, ph := "id", "$1"
	if backend == "sqlite" {
		ph = "?"
		if table == "log2timeline" {
			idCol = "rowid"
		}
	}
	var tag, color *string
	if err := store.Conn().QueryRow("SELECT tag, color FROM "+table+" WHERE "+idCol+" = "+ph, id).Scan(&tag, &color); err != nil {
		t.Fatalf("reading %s %d: %v", table, id, err)
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return deref(tag), deref(color)
}

func beByID(rows []database.BulkEditRow) map[int64]database.BulkEditRow {
	m := map[int64]database.BulkEditRow{}
	for _, r := range rows {
		m[r.ID] = r
	}
	return m
}

// TestBulkEditReturnsRows checks that bulk color and tag return the updated
// rows, that color is one statement per table, that the tag semantics are
// unchanged (dedup with trimming and case folding), and that examiner notes
// take a color but never a bulk tag.
func TestBulkEditReturnsRows(t *testing.T) {
	for _, b := range ifBackends() {
		t.Run(b.name, func(t *testing.T) {
			store := b.newStore(t)
			events := []*model.Event{
				{Datetime: "2025-01-10 10:00:00", Source: "FILE", Tag: "a, B", Color: "RED"},
				{Datetime: "2025-01-10 10:00:01", Source: "FILE"},
				{Datetime: "2025-01-10 10:00:02", Source: "FILE", Tag: "a"},
				{Datetime: "2025-01-10 10:00:03", Source: "FILE", Tag: "keep", Color: "BLUE"},
			}
			if _, err := store.InsertEvents(events, nil); err != nil {
				t.Fatal(err)
			}
			ids := beIDs(t, store)
			// InsertExaminerNote returns the note's grid ID, which is already
			// negative; noteID is its positive examiner_notes.id.
			noteGridID, err := store.InsertExaminerNote("2025-01-10 11:00:00", "note", "ntag", "")
			if err != nil {
				t.Fatal(err)
			}
			noteID := -noteGridID
			app := &App{store: store, driver: b.name}
			counts := beCountUpdates(t)

			sel := []int64{ids[0], ids[1], ids[2], noteGridID}
			rows, err := app.BulkUpdateColor(sel, "GREEN")
			if err != nil {
				t.Fatalf("BulkUpdateColor: %v", err)
			}
			got := beByID(rows)
			if len(got) != 4 {
				t.Fatalf("color returned %d rows, want 4: %+v", len(got), rows)
			}
			wantTags := map[int64]string{ids[0]: "a, B", ids[1]: "", ids[2]: "a", noteGridID: "ntag"}
			for id, tag := range wantTags {
				if r := got[id]; r.Color != "GREEN" || r.Tag != tag {
					t.Errorf("color row %d = %+v, want tag %q color GREEN", id, r, tag)
				}
			}
			if counts["log2timeline"] != 1 || counts["examiner_notes"] != 1 {
				t.Errorf("color UPDATE statements = %v, want one per table", counts)
			}
			if _, color := beStored(t, store, b.name, "log2timeline", ids[3]); color != "BLUE" {
				t.Errorf("unselected row color = %q, want BLUE", color)
			}
			if _, color := beStored(t, store, b.name, "examiner_notes", noteID); color != "GREEN" {
				t.Errorf("note color = %q, want GREEN", color)
			}

			rows, err = app.BulkAddTag(sel, "b")
			if err != nil {
				t.Fatalf("BulkAddTag: %v", err)
			}
			got = beByID(rows)
			if _, ok := got[noteGridID]; ok || len(got) != 3 {
				t.Errorf("tag returned %+v, want the three events and no note", rows)
			}
			wantTags = map[int64]string{ids[0]: "a, B", ids[1]: "b", ids[2]: "a,b"}
			for id, tag := range wantTags {
				if r := got[id]; r.Tag != tag || r.Color != "GREEN" {
					t.Errorf("tag row %d = %+v, want tag %q color GREEN", id, r, tag)
				}
				if stored, _ := beStored(t, store, b.name, "log2timeline", id); stored != tag {
					t.Errorf("stored tag %d = %q, want %q", id, stored, tag)
				}
			}
			if tag, _ := beStored(t, store, b.name, "examiner_notes", noteID); tag != "ntag" {
				t.Errorf("note tag = %q, want ntag unchanged", tag)
			}
		})
	}
}

// TestBulkColorSQLiteChunks lowers the SQLite chunk size and checks that a
// selection past it is split into several statements and fully updated.
func TestBulkColorSQLiteChunks(t *testing.T) {
	defer database.SetSQLiteBulkChunk(2)()
	store := newSQLiteForPush(t, "chunks.db")
	if _, err := store.InsertEvents(ccEvents(5, "WS1"), nil); err != nil {
		t.Fatal(err)
	}
	ids := beIDs(t, store)
	app := &App{store: store, driver: "sqlite"}
	counts := beCountUpdates(t)

	rows, err := app.BulkUpdateColor(ids, "RED")
	if err != nil {
		t.Fatalf("BulkUpdateColor: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("returned %d rows, want 5", len(rows))
	}
	if counts["log2timeline"] != 3 {
		t.Errorf("UPDATE statements = %d, want 3 (chunks of 2, 2, 1)", counts["log2timeline"])
	}
	for _, id := range ids {
		if _, color := beStored(t, store, "sqlite", "log2timeline", id); color != "RED" {
			t.Errorf("row %d color = %q, want RED", id, color)
		}
	}
}

// TestKeywordSearchEditableFields pins what the frontend's bulk edit reload
// rule assumes: keyword search reads tag and not color.
func TestKeywordSearchEditableFields(t *testing.T) {
	has := map[string]bool{}
	for _, f := range keywordSearchFields {
		has[f] = true
	}
	if !has["tag"] || has["color"] {
		t.Errorf("keywordSearchFields has tag=%v color=%v; update KEYWORD_SEARCH_EDITABLE_FIELDS in App.jsx", has["tag"], has["color"])
	}
}
