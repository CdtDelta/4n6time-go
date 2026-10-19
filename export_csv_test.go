package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

// wantExportHeader is the export column list. The first 23 names are the
// pre-existing export columns in their original order; import_file is last.
var wantExportHeader = []string{
	"datetime", "timezone", "MACB", "source", "sourcetype", "type",
	"user", "host", "desc", "filename", "inode", "notes", "format",
	"extra", "reportnotes", "inreport", "tag", "color",
	"offset", "store_number", "store_index", "vss_store_number", "bookmark",
	"import_file",
}

func readExportCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("reading exported CSV: %v", err)
	}
	return rows
}

func TestWriteEventsCSVAppendsImportFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.csv")
	events := []*model.Event{
		{Datetime: "2025-01-01 00:00:00", Source: "FILE", Desc: "attributed", Bookmark: 1, ImportID: 2},
		{Datetime: "2025-01-02 00:00:00", Source: "FILE", Desc: "unattributed", ImportID: 0},
		{Datetime: "2025-01-03 00:00:00", Source: "FILE", Desc: "dangling", ImportID: 9},
		{Datetime: "2025-01-04 00:00:00", Source: "FILE", Desc: "comma, and \"quote\"", ImportID: 2},
	}
	importFiles := map[int64]string{2: "ul.jsonl, with comma.jsonl"}

	if err := writeExportCSV(path, events, importFiles); err != nil {
		t.Fatalf("writeExportCSV: %v", err)
	}
	rows := readExportCSV(t, path)

	if len(rows) != 1+len(events) {
		t.Fatalf("got %d rows, want header plus %d events", len(rows), len(events))
	}
	if !reflect.DeepEqual(rows[0], wantExportHeader) {
		t.Errorf("header = %v\nwant %v", rows[0], wantExportHeader)
	}

	last := len(wantExportHeader) - 1
	if got := rows[1][last]; got != "ul.jsonl, with comma.jsonl" {
		t.Errorf("import_file for ImportID 2 = %q", got)
	}
	if got := rows[2][last]; got != "" {
		t.Errorf("import_file for ImportID 0 = %q, want empty", got)
	}
	if got := rows[3][last]; got != "" {
		t.Errorf("import_file for ImportID missing from map = %q, want empty", got)
	}
	if rows[4][8] != "comma, and \"quote\"" {
		t.Errorf("desc with comma and quote did not round trip: %q", rows[4][8])
	}

	// Existing columns keep their values; the raw import ID is not one of them.
	if rows[1][0] != "2025-01-01 00:00:00" || rows[1][3] != "FILE" || rows[1][22] != "1" {
		t.Errorf("existing columns changed: datetime=%q source=%q bookmark=%q",
			rows[1][0], rows[1][3], rows[1][22])
	}
	for _, r := range rows[1:] {
		if len(r) != len(wantExportHeader) {
			t.Fatalf("row has %d columns, want %d", len(r), len(wantExportHeader))
		}
	}
}
