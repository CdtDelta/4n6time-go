package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
)

// importLECmdCSV is a valid single-row LECmd CSV, a recognized EZ Tools format.
const importLECmdCSV = "SourceFile,TargetCreated,TargetModified,LocalPath,DriveType\n" +
	"test.lnk,2026-01-01 00:00:00,2026-01-02 00:00:00,C:\\path\\to\\file,Fixed\n"

// importTLN is a valid one-event TLN file.
const importTLN = "Time|Source|Host|User|Description\n" +
	"1700000000|FILE|HOST1|alice|first event\n"

// newImportTestApp returns an App with an open SQLite store and no Wails
// context, so import methods skip the file dialogs and runtime events.
func newImportTestApp(t *testing.T) *App {
	t.Helper()
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "import.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return &App{store: store, driver: "sqlite"}
}

func writeImportFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportCSVRecordsDetectedLabelAndTrimmedLabel(t *testing.T) {
	app := newImportTestApp(t)
	dir := t.TempDir()
	ez := writeImportFile(t, filepath.Join(dir, "lecmd.csv"), importLECmdCSV)
	tln := writeImportFile(t, filepath.Join(dir, "timeline.tln"), importTLN)

	for _, path := range []string{ez, tln} {
		info, err := app.ImportCSV(path, "  Host1 ")
		if err != nil {
			t.Fatalf("ImportCSV(%s): %v", filepath.Base(path), err)
		}
		if info == nil {
			t.Fatalf("ImportCSV(%s) returned no DBInfo", filepath.Base(path))
		}
	}

	records, err := app.store.GetImports()
	if err != nil {
		t.Fatalf("GetImports: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d import records, want 2: %+v", len(records), records)
	}
	want := map[string]string{
		"lecmd.csv":    "LECmd", // per-tool label, as recursive import records it
		"timeline.tln": "TLN",   // entry name, which is also what Detect returns
	}
	for _, rec := range records {
		if rec.Format != want[rec.Filename] {
			t.Errorf("%s format = %q, want %q", rec.Filename, rec.Format, want[rec.Filename])
		}
		if rec.Label != "Host1" {
			t.Errorf("%s label = %q, want trimmed Host1", rec.Filename, rec.Label)
		}
		if rec.Status != "complete" {
			t.Errorf("%s status = %q, want complete", rec.Filename, rec.Status)
		}
	}
}

func TestImportCSVEmptyPathCancels(t *testing.T) {
	app := newImportTestApp(t)
	info, err := app.ImportCSV("", "ignored")
	if err != nil || info != nil {
		t.Fatalf("ImportCSV(\"\") = %v, %v; want nil, nil", info, err)
	}
	if records, _ := app.store.GetImports(); len(records) != 0 {
		t.Errorf("cancelled import created %d records", len(records))
	}
}

func TestImportFolderRecursiveRecordsLabelPerFile(t *testing.T) {
	app := newImportTestApp(t)
	root := t.TempDir()
	writeImportFile(t, filepath.Join(root, "a.csv"), importLECmdCSV)
	writeImportFile(t, filepath.Join(root, "sub", "b.csv"), importLECmdCSV)
	writeImportFile(t, filepath.Join(root, "junk.csv"), "foo,bar\n1,2\n")

	summary, err := app.ImportFolderRecursive(root, "  Host1 ")
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if summary == nil || summary.TotalFilesProcessed != 2 {
		t.Fatalf("summary = %+v, want 2 files processed", summary)
	}

	records, err := app.store.GetImports()
	if err != nil {
		t.Fatalf("GetImports: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d import records, want 2: %+v", len(records), records)
	}
	names := map[string]bool{}
	for _, rec := range records {
		names[rec.Filename] = true
		if rec.Label != "Host1" || rec.Format != "LECmd" {
			t.Errorf("%s label/format = %q/%q, want Host1/LECmd", rec.Filename, rec.Label, rec.Format)
		}
	}
	if !names["a.csv"] || !names["sub/b.csv"] {
		t.Errorf("record filenames = %v, want a.csv and sub/b.csv", names)
	}
}

func TestImportFolderRecursiveEmptyPathCancels(t *testing.T) {
	app := newImportTestApp(t)
	summary, err := app.ImportFolderRecursive("", "ignored")
	if err != nil || summary != nil {
		t.Fatalf("ImportFolderRecursive(\"\") = %v, %v; want nil, nil", summary, err)
	}
}
