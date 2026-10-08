package importer

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"strings"
	"testing"
)

// UnifiedLog fixtures. Boot and process UUIDs are fixed unless a test varies
// them.
const (
	ulBootUUID    = "BDC29971B7954B4C84BB0F66C67C7F58"
	ulProcessUUID = "C644A1F3927539A881F135389AFCCB11"
)

var ulCSVHeader = []string{
	"Timestamp", "Event Type", "Log Type", "Subsystem", "Thread ID", "PID", "EUID",
	"Library", "Library UUID", "Activity ID", "Parent Activity ID", "Category",
	"Process", "Process UUID", "Message", "Raw Message", "Boot UUID", "System Timezone Name",
}

// ulJSONLine returns one unifiedlog_iterator JSONL record.
func ulJSONLine(bootUUID, timestamp, message string) string {
	return `{"subsystem":"","thread_id":2093925,"pid":5856,"euid":501,"library":"/usr/bin/logger",` +
		`"library_uuid":"` + ulProcessUUID + `","activity_id":0,"parent_activity_id":0,` +
		`"category":"","event_type":"Log","log_type":"Default","process":"/usr/bin/logger",` +
		`"process_uuid":"` + ulProcessUUID + `","message":"` + message + `","raw_message":"%s",` +
		`"boot_uuid":"` + bootUUID + `","timezone_name":"UTC","timestamp":"` + timestamp + `",` +
		`"message_flags":["MainExe"],"evidence":"/case/x.logarchive/Persist/1.tracev3"}`
}

// ulJSONL returns a two-record JSONL file starting at firstTimestamp.
func ulJSONL(bootUUID, firstTimestamp string) string {
	return ulJSONLine(bootUUID, firstTimestamp, "first") + "\n" +
		ulJSONLine(bootUUID, "2026-10-07T15:00:00.000000000Z", "second") + "\n"
}

// ulCSV returns a two-record CSV file starting at firstTimestamp.
func ulCSV(t *testing.T, bootUUID, firstTimestamp string) string {
	t.Helper()
	row := func(ts, msg string) []string {
		return []string{
			ts, "Log", "Default", "", "2093925", "5856", "501", "/usr/bin/logger", ulProcessUUID,
			"0", "0", "", "/usr/bin/logger", ulProcessUUID, msg, "%s", bootUUID, "UTC",
		}
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, r := range [][]string{ulCSVHeader, row(firstTimestamp, "first"), row("2026-10-07T15:00:00.000Z", "second")} {
		if err := w.Write(r); err != nil {
			t.Fatalf("writing CSV fixture: %v", err)
		}
	}
	w.Flush()
	return buf.String()
}

const plasoJSONL = `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "timestamp_desc": "Last Written", "source_short": "FILE", "message": "test event", "parser": "mft"}` + "\n"

func findSkip(summary *ImportSummary, rel string) (SkippedFile, bool) {
	for _, sf := range summary.SkippedFiles {
		if sf.RelativePath == rel {
			return sf, true
		}
	}
	return SkippedFile{}, false
}

func duplicateReason(keptRel string) string {
	return "duplicate of " + keptRel + " (same UnifiedLog data)"
}

// --- Single-file detection ---

func TestDetectFileUnifiedLogAndNeighbors(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name, file, content, want string
	}{
		{"UnifiedLog JSONL is not Plaso", "ul.jsonl", ulJSONL(ulBootUUID, "2026-10-07T14:53:48.542682112Z"), FormatUnifiedLogJSONL},
		{"Plaso JSONL is not UnifiedLog", "plaso.jsonl", plasoJSONL, FormatJSONL},
		{"UnifiedLog CSV", "ul.csv", ulCSV(t, ulBootUUID, "2026-10-07T14:53:48.542Z"), FormatUnifiedLogCSV},
		{"EZ Tools CSV is not UnifiedLog", "lecmd.csv", minimalLECmdCSV, FormatEZTools},
		{"EvtxECmd CSV is not UnifiedLog", "evtx.csv",
			"RecordNumber,EventRecordId,TimeCreated,EventId,Level,Provider,Channel,ProcessId,ThreadId,Computer,UserId,MapDescription,UserName,RemoteHost,PayloadData1,PayloadData2,PayloadData3,PayloadData4,PayloadData5,PayloadData6,ExecutableInfo,HiddenRecord,SourceFile,Keywords,ExtraDataOffset,Payload\n" +
				"1,1,2026-01-01 00:00:00.0000000,4624,Info,Microsoft-Windows-Security-Auditing,Security,4,8,WS1,S-1-5-18,Logon,,,,,,,,,,False,C:\\Security.evtx,,0,\n",
			FormatEZTools},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTmpCSV(t, dir, tt.file, tt.content)
			f, _, err := DetectFile(path)
			if err != nil {
				t.Fatalf("DetectFile: %v", err)
			}
			if f.Name != tt.want {
				t.Errorf("DetectFile = %q, want %q", f.Name, tt.want)
			}
		})
	}
}

// TestDetectFileJSONObjectNeitherFormat verifies a .jsonl holding JSON that
// is neither UnifiedLog nor Plaso still reports Plaso's error text and does
// not fall through to the CSV formats.
func TestDetectFileJSONObjectNeitherFormat(t *testing.T) {
	path := writeTmpCSV(t, t.TempDir(), "other.jsonl", `{"datetime":"2026-01-01","host":"x"}`+"\n")
	_, _, err := DetectFile(path)
	if err == nil {
		t.Fatal("DetectFile succeeded, want error")
	}
	const want = "invalid JSONL file: does not appear to be Plaso JSONL (missing expected fields)"
	if err.Error() != want {
		t.Errorf("DetectFile error = %q, want %q", err.Error(), want)
	}
}

// TestDetectFileTLNExclusivityUnchanged verifies an invalid .tln still fails
// with the TLN error rather than trying other formats.
func TestDetectFileTLNExclusivityUnchanged(t *testing.T) {
	path := writeTmpCSV(t, t.TempDir(), "bad.tln", "datetime,message,host\n2026-01-01 00:00:00,x,y\n")
	_, _, err := DetectFile(path)
	if err == nil || !strings.HasPrefix(err.Error(), "invalid TLN file: ") {
		t.Errorf("DetectFile error = %v, want invalid TLN file error", err)
	}
}

// --- Recursive import ---

func TestImportFolderRecursiveMixedFamilies(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "unifiedlog.jsonl", ulJSONL(ulBootUUID, "2026-10-07T14:53:48.542682112Z"))
	writeTmpCSV(t, root, "lecmd.csv", minimalLECmdCSV)
	writeTmpCSV(t, root, "plaso.jsonl", plasoJSONL)
	writeTmpCSV(t, root, "unknown.csv", "Name,Value\nfoo,bar\n")

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if summary.TotalFilesProcessed != 2 {
		t.Errorf("TotalFilesProcessed = %d, want 2", summary.TotalFilesProcessed)
	}
	if stats := summary.PerTool[FormatUnifiedLogJSONL]; stats.FileCount != 1 || stats.EventCount != 2 {
		t.Errorf("UnifiedLog JSONL stats = %+v, want 1 file, 2 events", stats)
	}
	if stats := summary.PerTool["LECmd"]; stats.FileCount != 1 {
		t.Errorf("LECmd stats = %+v, want 1 file", stats)
	}

	for _, rel := range []string{"plaso.jsonl", "unknown.csv"} {
		sf, ok := findSkip(summary, rel)
		if !ok {
			t.Errorf("%s not in SkippedFiles: %v", rel, summary.SkippedFiles)
			continue
		}
		if sf.Reason == "" {
			t.Errorf("%s skipped without a reason", rel)
		}
	}

	wantFamilies := map[string]string{
		FormatUnifiedLogJSONL: FamilyUnifiedLog,
		"LECmd":               FamilyEZTools,
	}
	if len(summary.ToolFamilies) != len(wantFamilies) {
		t.Errorf("ToolFamilies = %v, want %v", summary.ToolFamilies, wantFamilies)
	}
	for label, family := range wantFamilies {
		if summary.ToolFamilies[label] != family {
			t.Errorf("ToolFamilies[%q] = %q, want %q", label, summary.ToolFamilies[label], family)
		}
	}
}

// --- Duplicate UnifiedLog outputs ---

func TestDedupeSameBaseNameKeepsJSONL(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "ul.csv", ulCSV(t, ulBootUUID, "2026-10-07T14:53:48.542Z"))
	writeTmpCSV(t, root, "ul.jsonl", ulJSONL(ulBootUUID, "2026-10-07T14:53:48.542682112Z"))

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if summary.TotalFilesProcessed != 1 || summary.PerTool[FormatUnifiedLogJSONL].FileCount != 1 {
		t.Errorf("want only ul.jsonl imported; PerTool = %v", summary.PerTool)
	}
	if _, ok := summary.PerTool[FormatUnifiedLogCSV]; ok {
		t.Error("ul.csv imported despite duplicate")
	}
	sf, ok := findSkip(summary, "ul.csv")
	if !ok || sf.Reason != duplicateReason("ul.jsonl") {
		t.Errorf("ul.csv skip = %+v (found %v), want reason %q", sf, ok, duplicateReason("ul.jsonl"))
	}
	if store.insertedCount != 2 {
		t.Errorf("insertedCount = %d, want 2 (one file's events)", store.insertedCount)
	}
}

func TestDedupeFingerprintAcrossDirectories(t *testing.T) {
	root := t.TempDir()
	// The CSV walks first; the JSONL must still be the one kept.
	writeTmpCSV(t, mkDir(t, root, "a_csv"), "export_one.csv", ulCSV(t, ulBootUUID, "2026-10-07T14:53:48.542Z"))
	writeTmpCSV(t, mkDir(t, root, "b_jsonl"), "events_two.jsonl", ulJSONL(ulBootUUID, "2026-10-07T14:53:48.542682112Z"))

	summary, err := ImportFolderRecursive(root, &mockStore{}, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if summary.PerTool[FormatUnifiedLogJSONL].FileCount != 1 {
		t.Errorf("JSONL not imported; PerTool = %v", summary.PerTool)
	}
	if _, ok := summary.PerTool[FormatUnifiedLogCSV]; ok {
		t.Error("CSV imported despite matching fingerprint")
	}
	csvRel := filepath.Join("a_csv", "export_one.csv")
	want := duplicateReason(filepath.Join("b_jsonl", "events_two.jsonl"))
	if sf, ok := findSkip(summary, csvRel); !ok || sf.Reason != want {
		t.Errorf("%s skip = %+v (found %v), want reason %q", csvRel, sf, ok, want)
	}
}

// TestDedupeSameBootDifferentTimestamps verifies a shared Boot UUID alone
// does not make two collections duplicates.
func TestDedupeSameBootDifferentTimestamps(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "morning.jsonl", ulJSONL(ulBootUUID, "2026-10-07T09:00:00.000000000Z"))
	writeTmpCSV(t, root, "evening.jsonl", ulJSONL(ulBootUUID, "2026-10-07T21:00:00.000000000Z"))

	summary, err := ImportFolderRecursive(root, &mockStore{}, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if summary.TotalFilesProcessed != 2 {
		t.Errorf("TotalFilesProcessed = %d, want 2; skipped: %v", summary.TotalFilesProcessed, summary.SkippedFiles)
	}
	if len(summary.SkippedFiles) != 0 {
		t.Errorf("SkippedFiles = %v, want none", summary.SkippedFiles)
	}
}

func TestDedupeIdenticalJSONLKeepsFirstInWalkOrder(t *testing.T) {
	root := t.TempDir()
	content := ulJSONL(ulBootUUID, "2026-10-07T14:53:48.542682112Z")
	writeTmpCSV(t, root, "a_copy.jsonl", content)
	writeTmpCSV(t, root, "b_copy.jsonl", content)

	summary, err := ImportFolderRecursive(root, &mockStore{}, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if summary.TotalFilesProcessed != 1 {
		t.Errorf("TotalFilesProcessed = %d, want 1", summary.TotalFilesProcessed)
	}
	if _, ok := findSkip(summary, "a_copy.jsonl"); ok {
		t.Error("a_copy.jsonl skipped; the first file in walk order must be kept")
	}
	if sf, ok := findSkip(summary, "b_copy.jsonl"); !ok || sf.Reason != duplicateReason("a_copy.jsonl") {
		t.Errorf("b_copy.jsonl skip = %+v (found %v), want reason %q", sf, ok, duplicateReason("a_copy.jsonl"))
	}
}
