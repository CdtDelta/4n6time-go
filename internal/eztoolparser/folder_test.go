package eztoolparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// minimalLECmdCSV is a valid single-row LECmd CSV for test fixtures.
// LECmd detection requires: SourceFile, TargetCreated, TargetModified, LocalPath, DriveType.
const minimalLECmdCSV = "SourceFile,TargetCreated,TargetModified,LocalPath,DriveType\n" +
	"test.lnk,2026-01-01 00:00:00,2026-01-02 00:00:00,C:\\path\\to\\file,Fixed\n"

// minimalMFTECmdBootCSV is a $Boot CSV with the distinguishing header columns and no timestamps.
const minimalMFTECmdBootCSV = "EntryPoint,Signature,BytesPerSector,SectorsPerCluster,MFTCluster\n" +
	"0,NTFS,512,8,786432\n"

// mockStore records insertions and stubs out all other Store methods.
type mockStore struct {
	insertedCount int
}

func (m *mockStore) InsertEvent(_ *model.Event) error { return nil }
func (m *mockStore) InsertEvents(events []*model.Event, _ func(int)) (int, error) {
	m.insertedCount += len(events)
	return len(events), nil
}
func (m *mockStore) QueryEvents(_ string, _ []interface{}, _ string, _, _ int) ([]*model.Event, error) {
	return nil, nil
}
func (m *mockStore) CountEvents(_ string, _ []interface{}) (int64, error) { return 0, nil }
func (m *mockStore) UpdateEvent(_ int64, _ map[string]interface{}) error  { return nil }
func (m *mockStore) ToggleBookmark(_ int64) (int64, error)                { return 0, nil }
func (m *mockStore) ExecuteQuery(_ string, _ []interface{}, _ *database.NotesFilter) ([]*model.Event, error) {
	return nil, nil
}
func (m *mockStore) ExecuteCountQuery(_ string, _ []interface{}, _ *database.NotesFilter) (int64, error) {
	return 0, nil
}
func (m *mockStore) GetDistinctValues(_ string) (map[string]int64, error)       { return nil, nil }
func (m *mockStore) GetDistinctValuesFiltered(_ string, _ string, _ []interface{}) (map[string]int64, error) {
	return nil, nil
}
func (m *mockStore) GetDistinctTags() ([]string, error)         { return nil, nil }
func (m *mockStore) GetMinMaxDate() (string, string, error)     { return "", "", nil }
func (m *mockStore) GetMinMaxDateFiltered(_ string, _ []interface{}) (string, string, error) {
	return "", "", nil
}
func (m *mockStore) GetTimelineHistogram(_ string, _ []interface{}) ([]database.TimelineBucket, error) {
	return nil, nil
}
func (m *mockStore) GetSavedQueries() ([]database.SavedQuery, error)          { return nil, nil }
func (m *mockStore) SaveQuery(_ string, _ string) error                        { return nil }
func (m *mockStore) DeleteQuery(_ string) error                                { return nil }
func (m *mockStore) InsertExaminerNote(_, _, _, _ string) (int64, error)       { return 0, nil }
func (m *mockStore) DeleteExaminerNote(_ int64) error                          { return nil }
func (m *mockStore) UpdateExaminerNoteColor(_ int64, _ string) error           { return nil }
func (m *mockStore) ToggleExaminerNoteBookmark(_ int64) (int64, error)         { return 0, nil }
func (m *mockStore) GetExaminerNotes() ([]*model.Event, error)                 { return nil, nil }
func (m *mockStore) BulkUpdateColor(_ []int64, _ string) error                 { return nil }
func (m *mockStore) BulkAddTag(_ []int64, _ string) error                      { return nil }
func (m *mockStore) BulkSetBookmark(_ []int64, _ int64) error                  { return nil }
func (m *mockStore) BulkUpdateExaminerNoteColor(_ []int64, _ string) error     { return nil }
func (m *mockStore) BulkSetExaminerNoteBookmark(_ []int64, _ int64) error      { return nil }
func (m *mockStore) SaveTabSession(_ string) error                             { return nil }
func (m *mockStore) LoadTabSession() (string, error)                           { return "", nil }
func (m *mockStore) UpdateMetadata() error                                     { return nil }
func (m *mockStore) RebuildIndexes(_ []string) error                           { return nil }
func (m *mockStore) Migrate() error                                            { return nil }
func (m *mockStore) Close() error                                              { return nil }
func (m *mockStore) Path() string                                              { return ":mock:" }

// writeTmpCSV writes content to a named file inside dir and returns the path.
func writeTmpCSV(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTmpCSV %s: %v", name, err)
	}
	return path
}

// mkDir creates a subdirectory and returns its path.
func mkDir(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkDir %s: %v", name, err)
	}
	return dir
}

// --- Tests ---

// TestImportFolderRecursiveDepthCap verifies that CSV files at depth 4 are
// not processed. The tree is: root/a/b/c/deep.csv (depth 4).
func TestImportFolderRecursiveDepthCap(t *testing.T) {
	root := t.TempDir()

	// depth 1 file - should be processed
	writeTmpCSV(t, root, "d1.csv", minimalLECmdCSV)

	// depth 4 file - must NOT be processed
	d3 := mkDir(t, mkDir(t, mkDir(t, root, "a"), "b"), "c")
	writeTmpCSV(t, d3, "deep.csv", minimalLECmdCSV)

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	// depth 4 file must not be counted
	if summary.TotalFilesProcessed != 1 {
		t.Errorf("TotalFilesProcessed = %d, want 1 (depth-1 file only)", summary.TotalFilesProcessed)
	}
	if summary.MaxDepthReached != 1 {
		t.Errorf("MaxDepthReached = %d, want 1", summary.MaxDepthReached)
	}
}

// TestImportFolderRecursiveSymlinkSkip verifies that a symlink to a CSV file
// is not followed or counted as an imported file.
func TestImportFolderRecursiveSymlinkSkip(t *testing.T) {
	root := t.TempDir()

	// Create the real CSV file.
	real := writeTmpCSV(t, root, "real.csv", minimalLECmdCSV)

	// Create a symlink to the same file; it should be skipped.
	link := filepath.Join(root, "link.csv")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	// Only real.csv should be processed; link.csv is a symlink and must be skipped.
	if summary.TotalFilesProcessed != 1 {
		t.Errorf("TotalFilesProcessed = %d, want 1 (real.csv only, link.csv is symlink)", summary.TotalFilesProcessed)
	}
}

// TestImportFolderRecursiveCSVFilter verifies that non-.csv files are silently
// ignored and do not appear in SkippedFiles.
func TestImportFolderRecursiveCSVFilter(t *testing.T) {
	root := t.TempDir()

	// A valid .csv that should be processed.
	writeTmpCSV(t, root, "valid.csv", minimalLECmdCSV)

	// A .txt file that should be silently ignored (not in SkippedFiles).
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("irrelevant"), 0644); err != nil {
		t.Fatalf("writing txt file: %v", err)
	}

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if summary.TotalFilesProcessed != 1 {
		t.Errorf("TotalFilesProcessed = %d, want 1", summary.TotalFilesProcessed)
	}

	for _, sf := range summary.SkippedFiles {
		if sf.RelativePath == "notes.txt" {
			t.Errorf("notes.txt appeared in SkippedFiles but should be silently ignored")
		}
	}
}

// TestImportFolderRecursiveMixedTree validates a tree containing:
//   - a recognized EZ Tool CSV (at depth 1)
//   - an unrecognized CSV (at depth 2)
//   - a non-CSV file (silently ignored)
//
// Verifies correct PerTool map, SkippedFiles, and TotalEvents.
func TestImportFolderRecursiveMixedTree(t *testing.T) {
	root := t.TempDir()

	// Depth 1: recognized LECmd CSV.
	writeTmpCSV(t, root, "lecmd.csv", minimalLECmdCSV)

	// Depth 1: unrecognized CSV (generic columns).
	writeTmpCSV(t, root, "unknown.csv", "Name,Value\nfoo,bar\n")

	// Depth 1: non-CSV (silently ignored).
	if err := os.WriteFile(filepath.Join(root, "log.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("writing txt: %v", err)
	}

	// Depth 2: another recognized CSV.
	sub := mkDir(t, root, "sublevel")
	writeTmpCSV(t, sub, "lecmd2.csv", minimalLECmdCSV)

	store := &mockStore{}
	var progressCalls int
	summary, err := ImportFolderRecursive(root, store, func(_ string, _ int) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	// Two recognized LECmd files processed.
	if summary.TotalFilesProcessed != 2 {
		t.Errorf("TotalFilesProcessed = %d, want 2", summary.TotalFilesProcessed)
	}

	// One unrecognized CSV skipped with correct reason.
	var foundUnrecognized bool
	for _, sf := range summary.SkippedFiles {
		if sf.RelativePath == "unknown.csv" && sf.Reason == SkipReasonUnrecognizedFormat {
			foundUnrecognized = true
		}
	}
	if !foundUnrecognized {
		t.Errorf("expected unknown.csv in SkippedFiles with reason %q; got: %v",
			SkipReasonUnrecognizedFormat, summary.SkippedFiles)
	}

	// PerTool map should have LECmd entry.
	if stats, ok := summary.PerTool[ToolLECmd]; !ok {
		t.Error("PerTool missing LECmd entry")
	} else if stats.FileCount != 2 {
		t.Errorf("LECmd FileCount = %d, want 2", stats.FileCount)
	}

	// Progress callback must have been called once per processed file.
	if progressCalls != 2 {
		t.Errorf("progress callback calls = %d, want 2", progressCalls)
	}
}

// TestImportFolderRecursiveNoTimestampSkip verifies that a recognized format
// with no timestamp columns (e.g. MFTECmd $Boot) is skipped with a clear
// reason rather than counted as a processed file or treated as unknown.
func TestImportFolderRecursiveNoTimestampSkip(t *testing.T) {
	root := t.TempDir()

	// $Boot: recognized but has no timestamp columns.
	writeTmpCSV(t, root, "MFTECmd_$Boot_Output.csv", minimalMFTECmdBootCSV)

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if summary.TotalFilesProcessed != 0 {
		t.Errorf("TotalFilesProcessed = %d, want 0 ($Boot should not count as processed)", summary.TotalFilesProcessed)
	}
	if store.insertedCount != 0 {
		t.Errorf("insertedCount = %d, want 0 (no events should be inserted)", store.insertedCount)
	}

	var found bool
	for _, sf := range summary.SkippedFiles {
		if sf.RelativePath == "MFTECmd_$Boot_Output.csv" &&
			strings.Contains(sf.Reason, "no timestamp columns") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected $Boot file in SkippedFiles with 'no timestamp columns' reason; got: %v",
			summary.SkippedFiles)
	}

	// Must not appear under PerTool.
	if _, ok := summary.PerTool[ToolMFTECmdBoot]; ok {
		t.Error("$Boot file must not appear in PerTool map")
	}
}

// TestImportFolderRecursiveSummaryCorrectness validates DirectoriesWalked,
// MaxDepthReached, and TotalFilesProcessed across a 3-level tree.
func TestImportFolderRecursiveSummaryCorrectness(t *testing.T) {
	root := t.TempDir()

	// depth 1 file
	writeTmpCSV(t, root, "d1.csv", minimalLECmdCSV)

	// depth 2 file
	sub1 := mkDir(t, root, "sub1")
	writeTmpCSV(t, sub1, "d2.csv", minimalLECmdCSV)

	// depth 3 file
	sub2 := mkDir(t, sub1, "sub2")
	writeTmpCSV(t, sub2, "d3.csv", minimalLECmdCSV)

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if summary.TotalFilesProcessed != 3 {
		t.Errorf("TotalFilesProcessed = %d, want 3", summary.TotalFilesProcessed)
	}
	if summary.MaxDepthReached != 3 {
		t.Errorf("MaxDepthReached = %d, want 3", summary.MaxDepthReached)
	}
	// sub1 and sub2 are subdirectories walked (root is not counted).
	if summary.DirectoriesWalked != 2 {
		t.Errorf("DirectoriesWalked = %d, want 2", summary.DirectoriesWalked)
	}
	if summary.TotalEvents == 0 {
		t.Error("TotalEvents = 0, expected > 0")
	}
}
