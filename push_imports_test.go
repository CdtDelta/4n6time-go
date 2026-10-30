package main

import (
	"path/filepath"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

func newSQLiteForPush(t *testing.T, name string) *database.SQLiteStore {
	t.Helper()
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), name), nil)
	if err != nil {
		t.Fatalf("CreateSQLite(%s): %v", name, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestRemapImportIDs(t *testing.T) {
	events := []*model.Event{
		{Desc: "none", ImportID: 0},
		{Desc: "mapped", ImportID: 2},
		{Desc: "dangling", ImportID: 9},
	}
	cleared := remapImportIDs(events, map[int64]int64{2: 5})

	if cleared != 1 {
		t.Errorf("cleared = %d, want 1", cleared)
	}
	want := []int64{0, 5, 0}
	for i, e := range events {
		if e.ImportID != want[i] {
			t.Errorf("event %q ImportID = %d, want %d", e.Desc, e.ImportID, want[i])
		}
	}
}

func TestRemapImportIDsEmpty(t *testing.T) {
	if cleared := remapImportIDs(nil, map[int64]int64{1: 1}); cleared != 0 {
		t.Errorf("cleared = %d for no events, want 0", cleared)
	}
}

func TestCopyImportRecordsPreservesFields(t *testing.T) {
	src := newSQLiteForPush(t, "src.db")
	dst := newSQLiteForPush(t, "dst.db")

	complete, err := src.CreateImport(model.ImportRecord{
		Filename: "plaso.jsonl", Label: "WS1", Format: "Plaso JSONL", ImportedAt: "2026-01-02 03:04:05",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.FinalizeImport(complete, 42, model.ImportStatusComplete, ""); err != nil {
		t.Fatal(err)
	}
	failed, err := src.CreateImport(model.ImportRecord{Filename: "bad.csv", Format: "EZ Tools CSV"})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.FinalizeImport(failed, 0, model.ImportStatusFailed, ""); err != nil {
		t.Fatal(err)
	}
	pending, err := src.CreateImport(model.ImportRecord{Filename: "stuck.csv"})
	if err != nil {
		t.Fatal(err)
	}

	idMap, err := copyImportRecords(src, dst, nil)
	if err != nil {
		t.Fatalf("copyImportRecords: %v", err)
	}
	if len(idMap) != 3 {
		t.Fatalf("idMap has %d entries, want 3: %v", len(idMap), idMap)
	}

	srcRecs, err := src.GetImports()
	if err != nil {
		t.Fatal(err)
	}
	dstRecs, err := dst.GetImports()
	if err != nil {
		t.Fatal(err)
	}
	if len(dstRecs) != 3 {
		t.Fatalf("destination has %d records, want 3", len(dstRecs))
	}

	for _, s := range srcRecs {
		newID, ok := idMap[s.ID]
		if !ok {
			t.Fatalf("no mapping for source import %d (%s)", s.ID, s.Filename)
		}
		d, err := dst.GetImport(newID)
		if err != nil || d == nil {
			t.Fatalf("GetImport(%d) = %v, %v", newID, d, err)
		}
		if d.Filename != s.Filename || d.Label != s.Label || d.Format != s.Format {
			t.Errorf("%s: filename/label/format = %q/%q/%q, want %q/%q/%q",
				s.Filename, d.Filename, d.Label, d.Format, s.Filename, s.Label, s.Format)
		}
		if d.ImportedAt != s.ImportedAt {
			t.Errorf("%s: imported_at = %q, want %q", s.Filename, d.ImportedAt, s.ImportedAt)
		}
		if d.EventCount != s.EventCount {
			t.Errorf("%s: event_count = %d, want %d", s.Filename, d.EventCount, s.EventCount)
		}
		if d.Status != s.Status {
			t.Errorf("%s: status = %q, want %q", s.Filename, d.Status, s.Status)
		}
	}

	// The in_progress record stays in_progress rather than being finalized.
	if d, _ := dst.GetImport(idMap[pending]); d == nil || d.Status != model.ImportStatusInProgress {
		t.Errorf("pending import status on destination = %+v, want in_progress", d)
	}
}

func TestCopyImportRecordsThenRemapEvents(t *testing.T) {
	src := newSQLiteForPush(t, "src.db")
	dst := newSQLiteForPush(t, "dst.db")

	srcID, err := src.CreateImport(model.ImportRecord{Filename: "a.jsonl"})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.FinalizeImport(srcID, 1, model.ImportStatusComplete, ""); err != nil {
		t.Fatal(err)
	}
	// Burn an ID on the destination so the mapping is not the identity.
	if _, err := dst.CreateImport(model.ImportRecord{Filename: "existing.csv"}); err != nil {
		t.Fatal(err)
	}

	idMap, err := copyImportRecords(src, dst, nil)
	if err != nil {
		t.Fatalf("copyImportRecords: %v", err)
	}
	events := []*model.Event{{Desc: "e", ImportID: srcID}}
	if cleared := remapImportIDs(events, idMap); cleared != 0 {
		t.Fatalf("cleared = %d, want 0", cleared)
	}
	if events[0].ImportID != idMap[srcID] || events[0].ImportID == srcID {
		t.Errorf("ImportID = %d, want destination ID %d (source was %d)", events[0].ImportID, idMap[srcID], srcID)
	}
	if rec, _ := dst.GetImport(events[0].ImportID); rec == nil || rec.Filename != "a.jsonl" {
		t.Errorf("remapped ImportID does not point at a.jsonl: %+v", rec)
	}
}
