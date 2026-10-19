package importer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// captureStore keeps every event handed to InsertEvent and InsertEvents so a
// test can inspect ImportID after stamping.
type captureStore struct {
	*mockStore
	events []*model.Event
}

func (c *captureStore) InsertEvent(e *model.Event) error {
	c.events = append(c.events, e)
	return nil
}

func (c *captureStore) InsertEvents(events []*model.Event, _ func(int)) (int, error) {
	c.events = append(c.events, events...)
	return len(events), nil
}

func TestStampingStoreSetsImportID(t *testing.T) {
	cs := &captureStore{mockStore: &mockStore{}}
	s := &stampingStore{Store: cs, importID: 7}

	single := &model.Event{Desc: "single"}
	if err := s.InsertEvent(single); err != nil {
		t.Fatalf("InsertEvent: %v", err)
	}
	batch := []*model.Event{{Desc: "a"}, {Desc: "b"}}
	if n, err := s.InsertEvents(batch, nil); err != nil || n != 2 {
		t.Fatalf("InsertEvents = %d, %v; want 2, nil", n, err)
	}

	if len(cs.events) != 3 {
		t.Fatalf("store received %d events, want 3", len(cs.events))
	}
	for _, e := range cs.events {
		if e.ImportID != 7 {
			t.Errorf("event %q ImportID = %d, want 7", e.Desc, e.ImportID)
		}
	}
}

func TestStampingStoreNilEventDoesNotPanic(t *testing.T) {
	cs := &captureStore{mockStore: &mockStore{}}
	s := &stampingStore{Store: cs, importID: 3}
	if err := s.InsertEvent(nil); err != nil {
		t.Fatalf("InsertEvent(nil): %v", err)
	}
	if _, err := s.InsertEvents([]*model.Event{nil}, nil); err != nil {
		t.Fatalf("InsertEvents([nil]): %v", err)
	}
}

// openTestStore returns a fresh SQLite store in a temp directory.
func openTestStore(t *testing.T) *database.SQLiteStore {
	t.Helper()
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "t.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// eventsByImport counts events per ImportID across the whole store.
func eventsByImport(t *testing.T, store *database.SQLiteStore) map[int64]int {
	t.Helper()
	events, err := store.QueryEvents("", nil, "", 0, 0)
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	counts := make(map[int64]int)
	for _, e := range events {
		counts[e.ImportID]++
	}
	return counts
}

// collectLogs returns a Logger that appends each message to the slice.
func collectLogs(out *[]string) Logger {
	return func(level, msg string) {
		*out = append(*out, level+" "+msg)
	}
}

// tlnFixture is a two-event TLN file with the standard header.
const tlnFixture = "Time|Source|Host|User|Description\n" +
	"1700000000|FILE|HOST1|alice|first event\n" +
	"1700000100|FILE|HOST1|bob|second event\n"

func TestImportFileTracked(t *testing.T) {
	store := openTestStore(t)
	path := filepath.Join(t.TempDir(), "timeline.tln")
	if err := os.WriteFile(path, []byte(tlnFixture), 0600); err != nil {
		t.Fatal(err)
	}
	format, detected, err := DetectFile(path)
	if err != nil {
		t.Fatalf("DetectFile: %v", err)
	}

	var logs []string
	res, err := ImportFile(path, format, detected, "  HOST1  ", store, nil, collectLogs(&logs))
	if err != nil {
		t.Fatalf("ImportFile: %v", err)
	}
	if res.ID == 0 || res.Count != 2 {
		t.Fatalf("result = %+v, want ID set and Count 2", res)
	}

	rec, err := store.GetImport(res.ID)
	if err != nil || rec == nil {
		t.Fatalf("GetImport(%d) = %v, %v", res.ID, rec, err)
	}
	if rec.Filename != "timeline.tln" {
		t.Errorf("Filename = %q, want timeline.tln", rec.Filename)
	}
	if rec.Label != "HOST1" {
		t.Errorf("Label = %q, want trimmed HOST1", rec.Label)
	}
	if rec.Format != FormatTLN {
		t.Errorf("Format = %q, want %q", rec.Format, FormatTLN)
	}
	if rec.Status != model.ImportStatusComplete {
		t.Errorf("Status = %q, want complete", rec.Status)
	}
	if rec.EventCount != 2 {
		t.Errorf("EventCount = %d, want 2", rec.EventCount)
	}

	counts := eventsByImport(t, store)
	if counts[res.ID] != 2 || len(counts) != 1 {
		t.Errorf("events by import = %v, want 2 events all on import %d", counts, res.ID)
	}

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "INFO Import record") || !strings.Contains(joined, "created") ||
		!strings.Contains(joined, "finalized") || !strings.Contains(joined, "status=complete") {
		t.Errorf("logs missing creation or finalization lines:\n%s", joined)
	}
}

func TestImportFileFailedMarksRecordFailed(t *testing.T) {
	store := openTestStore(t)
	boom := errors.New("reading broken: boom")
	format := &Format{
		Name: "Broken",
		Import: func(path string, s database.Store, p *Progress) (int, error) {
			return 0, boom
		},
	}

	var logs []string
	res, err := ImportFile("broken.csv", format, format.Name, "", store, nil, collectLogs(&logs))
	if !errors.Is(err, boom) {
		t.Fatalf("ImportFile error = %v, want the original import error", err)
	}
	if res.ID == 0 {
		t.Fatal("failed import has no record ID")
	}

	rec, err := store.GetImport(res.ID)
	if err != nil || rec == nil {
		t.Fatalf("GetImport = %v, %v", rec, err)
	}
	if rec.Status != model.ImportStatusFailed {
		t.Errorf("Status = %q, want failed", rec.Status)
	}
	if rec.EventCount != 0 {
		t.Errorf("EventCount = %d, want 0", rec.EventCount)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "status=failed") {
		t.Errorf("logs missing failed finalization:\n%s", strings.Join(logs, "\n"))
	}
}

// lecmdFixture writes a LECmd CSV (a recognized EZ Tools format) into dir.
func lecmdFixture(t *testing.T, dir, name string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(minimalLECmdCSV), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestImportFolderRecursiveTracked(t *testing.T) {
	store := openTestStore(t)
	root := t.TempDir()
	lecmdFixture(t, root, "a.csv")
	lecmdFixture(t, root, "sub/b.csv")
	writeTmpCSV(t, root, "junk.csv", "foo,bar\n1,2\n")

	var logs []string
	summary, err := ImportFolderRecursive(root, store, "  Host1 ", collectLogs(&logs), nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if _, ok := findSkip(summary, "junk.csv"); !ok {
		t.Error("junk.csv was not skipped")
	}

	records, err := store.GetImports()
	if err != nil {
		t.Fatalf("GetImports: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d import records, want 2 (one per imported file): %+v", len(records), records)
	}

	byName := make(map[string]model.ImportRecord)
	for _, r := range records {
		byName[r.Filename] = r
	}
	for _, name := range []string{"a.csv", "sub/b.csv"} {
		rec, ok := byName[name]
		if !ok {
			t.Errorf("no import record for %q; have %v", name, byName)
			continue
		}
		if rec.Label != "Host1" {
			t.Errorf("%s label = %q, want Host1", name, rec.Label)
		}
		if rec.Format != "LECmd" {
			t.Errorf("%s format = %q, want LECmd", name, rec.Format)
		}
		if rec.Status != model.ImportStatusComplete {
			t.Errorf("%s status = %q, want complete", name, rec.Status)
		}
		if rec.EventCount == 0 {
			t.Errorf("%s event count = 0, want the imported count", name)
		}
	}
	if _, ok := byName["junk.csv"]; ok {
		t.Error("skipped junk.csv has an import record")
	}

	counts := eventsByImport(t, store)
	for _, rec := range records {
		if int64(counts[rec.ID]) != rec.EventCount {
			t.Errorf("%s: %d events stamped, record says %d", rec.Filename, counts[rec.ID], rec.EventCount)
		}
	}
	if counts[0] != 0 {
		t.Errorf("%d events have no import ID, want none", counts[0])
	}
}

func TestImportFolderRecursiveDuplicateUnifiedLogOneRecord(t *testing.T) {
	store := openTestStore(t)
	root := t.TempDir()
	writeTmpCSV(t, root, "ul.csv", ulCSV(t, ulBootUUID, "2026-10-07T14:53:48.542Z"))
	writeTmpCSV(t, root, "ul.jsonl", ulJSONL(ulBootUUID, "2026-10-07T14:53:48.542682112Z"))

	summary, err := ImportFolderRecursive(root, store, "", nil, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if _, ok := findSkip(summary, "ul.csv"); !ok {
		t.Fatal("ul.csv was not marked as a duplicate")
	}

	records, err := store.GetImports()
	if err != nil {
		t.Fatalf("GetImports: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d import records, want 1: %+v", len(records), records)
	}
	if records[0].Filename != "ul.jsonl" || records[0].Format != FormatUnifiedLogJSONL {
		t.Errorf("record = %+v, want ul.jsonl as UnifiedLog JSONL", records[0])
	}
}

// exportedHeader and exportedHeaderWithImport are the export writer's header
// before and after import_file was appended. Detection must treat them alike.
const (
	exportedHeader           = "datetime,timezone,MACB,source,sourcetype,type,user,host,desc,filename,inode,notes,format,extra,reportnotes,inreport,tag,color,offset,store_number,store_index,vss_store_number,bookmark"
	exportedHeaderWithImport = exportedHeader + ",import_file"
)

func TestExportedHeaderDetectionUnchanged(t *testing.T) {
	dir := t.TempDir()
	row := "2025-01-01 00:00:00,UTC,M,FILE,OS:NTFS,Written,admin,HOST,desc,/a,1,,mft,,,,,,0,-1,-1,-1,0\n"

	var got [2]string
	for i, header := range []string{exportedHeader, exportedHeaderWithImport} {
		path := filepath.Join(dir, fmt.Sprintf("export%d.csv", i))
		if err := os.WriteFile(path, []byte(header+"\n"+row), 0600); err != nil {
			t.Fatal(err)
		}
		format, _, err := DetectFile(path)
		if err != nil {
			t.Fatalf("DetectFile(%d): %v", i, err)
		}
		got[i] = format.Name
	}
	if got[0] != got[1] {
		t.Errorf("detection changed when import_file was appended: before %q, after %q", got[0], got[1])
	}
}
