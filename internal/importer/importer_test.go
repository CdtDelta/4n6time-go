package importer

import (
	"errors"
	"testing"
)

// TestRegistryOrder pins the detection order. The original five entries keep
// their historic relative order from app.go; UnifiedLog JSONL sits before
// Plaso JSONL and UnifiedLog CSV before EZ Tools. Dynamic CSV is a catch-all
// and must be last.
func TestRegistryOrder(t *testing.T) {
	want := []string{
		FormatUnifiedLogJSONL, FormatJSONL, FormatTLN,
		FormatUnifiedLogCSV, FormatEZTools, FormatL2TCSV, FormatDynamicCSV,
	}

	if len(Registry) != len(want) {
		t.Fatalf("len(Registry) = %d, want %d", len(Registry), len(want))
	}
	for i, name := range want {
		if Registry[i].Name != name {
			t.Errorf("Registry[%d].Name = %q, want %q", i, Registry[i].Name, name)
		}
	}
	if last := Registry[len(Registry)-1].Name; last != FormatDynamicCSV {
		t.Errorf("last registry entry = %q, want %q", last, FormatDynamicCSV)
	}
}

// TestRegistryRecursiveFlag verifies only EZ Tools and the two UnifiedLog
// entries are eligible for recursive folder import. Plaso JSONL and the
// catch-all CSV formats must stay out.
func TestRegistryRecursiveFlag(t *testing.T) {
	recursive := map[string]bool{
		FormatEZTools:         true,
		FormatUnifiedLogCSV:   true,
		FormatUnifiedLogJSONL: true,
	}
	for _, f := range Registry {
		if f.Recursive != recursive[f.Name] {
			t.Errorf("%s: Recursive = %v, want %v", f.Name, f.Recursive, recursive[f.Name])
		}
	}
}

// TestRegistryFamilies verifies family assignments, and that every recursive
// entry has one so the summary can group it.
func TestRegistryFamilies(t *testing.T) {
	want := map[string]string{
		FormatEZTools:         FamilyEZTools,
		FormatUnifiedLogCSV:   FamilyUnifiedLog,
		FormatUnifiedLogJSONL: FamilyUnifiedLog,
	}
	for _, f := range Registry {
		if f.Family != want[f.Name] {
			t.Errorf("%s: Family = %q, want %q", f.Name, f.Family, want[f.Name])
		}
		if f.Recursive && f.Family == "" {
			t.Errorf("%s: recursive entry has no Family", f.Name)
		}
	}
}

// TestRegistryEntriesComplete guards against a registry entry missing a
// required function, which would panic at import time.
func TestRegistryEntriesComplete(t *testing.T) {
	for _, f := range Registry {
		if f.Detect == nil {
			t.Errorf("%s: Detect is nil", f.Name)
		}
		if f.Import == nil {
			t.Errorf("%s: Import is nil", f.Name)
		}
		for _, e := range f.Extensions {
			if len(e) < 2 || e[0] != '.' {
				t.Errorf("%s: extension %q must start with a dot", f.Name, e)
			}
		}
	}
}

// TestRecursiveExtensions verifies the recursive extension filter is derived
// from the registry: .csv from EZ Tools and UnifiedLog CSV, .jsonl from
// UnifiedLog JSONL. Plaso's .json must not appear.
func TestRecursiveExtensions(t *testing.T) {
	exts := recursiveExtensions(recursiveFormats())
	if len(exts) != 2 {
		t.Fatalf("recursive extensions = %v, want .csv and .jsonl", exts)
	}
	for _, e := range []string{".csv", ".jsonl"} {
		if _, ok := exts[e]; !ok {
			t.Errorf("recursive extensions = %v, missing %s", exts, e)
		}
	}
}

// TestImportFolderRecursiveGenericCSVSkipped verifies a generic CSV that
// Dynamic CSV would accept in single-file import is skipped, not imported,
// during a recursive walk.
func TestImportFolderRecursiveGenericCSVSkipped(t *testing.T) {
	root := t.TempDir()

	const genericCSV = "datetime,message,host\n2026-01-01 00:00:00,something happened,WS01\n"
	path := writeTmpCSV(t, root, "generic.csv", genericCSV)

	// Confirm the fixture really is a non-EZ CSV that single-file import
	// would pick up through a later registry entry.
	f, _, err := DetectFile(path)
	if err != nil {
		t.Fatalf("DetectFile: %v", err)
	}
	if f.Name == FormatEZTools {
		t.Fatalf("fixture detected as EZ Tools; test needs a non-EZ CSV")
	}

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if summary.TotalFilesProcessed != 0 {
		t.Errorf("TotalFilesProcessed = %d, want 0", summary.TotalFilesProcessed)
	}
	if store.insertedCount != 0 {
		t.Errorf("insertedCount = %d, want 0", store.insertedCount)
	}

	var found bool
	for _, sf := range summary.SkippedFiles {
		if sf.RelativePath == "generic.csv" && sf.Reason == SkipReasonUnrecognizedFormat {
			found = true
		}
	}
	if !found {
		t.Errorf("expected generic.csv in SkippedFiles with reason %q; got: %v",
			SkipReasonUnrecognizedFormat, summary.SkippedFiles)
	}
}

// TestDetectFileExclusiveExtensionFails verifies a .jsonl file that fails
// JSONL validation errors out instead of falling through to CSV formats.
func TestDetectFileExclusiveExtensionFails(t *testing.T) {
	dir := t.TempDir()
	path := writeTmpCSV(t, dir, "bad.jsonl", "datetime,message\n2026-01-01 00:00:00,x\n")

	f, _, err := DetectFile(path)
	if err == nil {
		t.Fatalf("DetectFile returned %q, want error", f.Name)
	}
	// With two .jsonl claimants, the error text must still be the one
	// Plaso JSONL produced on its own.
	const want = "invalid JSONL file: first line is not a JSON object"
	if err.Error() != want {
		t.Errorf("DetectFile error = %q, want %q", err.Error(), want)
	}
}

// TestDetectFileUnrecognized verifies an unparseable file returns
// ErrUnrecognizedFormat.
func TestDetectFileUnrecognized(t *testing.T) {
	dir := t.TempDir()
	path := writeTmpCSV(t, dir, "blob.bin", "\x00\x01\x02")

	if _, _, err := DetectFile(path); !errors.Is(err, ErrUnrecognizedFormat) {
		t.Errorf("DetectFile err = %v, want ErrUnrecognizedFormat", err)
	}
}
