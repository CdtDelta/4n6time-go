package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/macaptparser"
)

// mac_apt fixtures. Headers match mac_apt v1.36.2 column lists exactly.
const (
	maFsHeader  = "LogID,EventFlagsHex,EventType,EventFlags,Filepath,File_ID,Log_Unknown,SourceModDate,Source"
	maFsRow     = "42,0x2,Create,Created,/Users/tom/a.txt,1234,,2026-03-04 05:06:07,/private/var/.fseventsd/1"
	maFsJSONRow = `{"LogID":42,"EventFlagsHex":"0x2","EventType":"Create","EventFlags":"Created",` +
		`"Filepath":"/Users/tom/a.txt","File_ID":1234,"Log_Unknown":null,` +
		`"SourceModDate":"2026-03-04 05:06:07","Source":"/private/var/.fseventsd/1"}`

	maASLHeader = "Timestamp,Nano,ID,Flags,Host,Sender,Facility,Level,PID,UID,GID,RefProc,RefPID,Message,Session,Key_Value,Source"
	maASLRow    = "2026-03-04 05:06:07+00:00,0,5,8,tom-mbp,loginwindow,auth,5,321,501,20,,0,hello,,,/x/asl"
)

// maFsCSV returns an FsEvents CSV with one row.
func maFsCSV() string { return maFsHeader + "\n" + maFsRow + "\n" }

// maFsJSONL returns an FsEvents JSONL with one row.
func maFsJSONL() string { return maFsJSONRow + "\n" }

// maASLFile returns an asl_ver2 CSV with one row.
func maASLFile() string { return maASLHeader + "\n" + maASLRow + "\n" }

// --- Detection through the registry ---

// TestDetectFileMacAptOutputs verifies each mac_apt file resolves to its
// mac_apt registry entry and label through DetectFile.
func TestDetectFileMacAptOutputs(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name, file, content, wantFormat, wantLabel string
	}{
		{"FsEvents CSV", "FsEvents.csv", maFsCSV(), FormatMacAptCSV, "mac_apt FsEvents"},
		{"FsEvents JSONL", "FsEvents.jsonl", maFsJSONL(), FormatMacAptJSONL, "mac_apt FsEvents"},
		{"asl_ver2 CSV", "asl_ver2.csv", maASLFile(), FormatMacAptCSV, "mac_apt asl_ver2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTmpCSV(t, dir, tt.file, tt.content)
			f, label, err := DetectFile(path)
			if err != nil {
				t.Fatalf("DetectFile: %v", err)
			}
			if f.Name != tt.wantFormat || label != tt.wantLabel {
				t.Errorf("DetectFile = (%q, %q), want (%q, %q)", f.Name, label, tt.wantFormat, tt.wantLabel)
			}
		})
	}
}

// TestMacAptNeighborsNotClaimed verifies mac_apt detection does not claim
// files from the neighboring formats, and that those formats still resolve
// to themselves through DetectFile.
func TestMacAptNeighborsNotClaimed(t *testing.T) {
	dir := t.TempDir()
	ulCSVPath := writeTmpCSV(t, dir, "ul.csv", ulCSV(t, ulBootUUID, "2026-10-07T14:00:00.000Z"))
	ulJSONLPath := writeTmpCSV(t, dir, "ul.jsonl", ulJSONL(ulBootUUID, "2026-10-07T14:00:00.000000000Z"))
	plasoPath := writeTmpCSV(t, dir, "plaso.jsonl", plasoJSONL)
	ezPath := writeTmpCSV(t, dir, "ez.csv",
		"RecordNumber,EventRecordId,TimeCreated,EventId,Level,Provider,Channel,ProcessId,ThreadId,Computer,UserId,MapDescription,UserName,RemoteHost,PayloadData1,PayloadData2,PayloadData3,PayloadData4,PayloadData5,PayloadData6,ExecutableInfo,HiddenRecord,SourceFile,Keywords,ExtraDataOffset,Payload\n")
	l2tPath := writeTmpCSV(t, dir, "l2t.csv",
		"date,time,timezone,MACB,source,sourcetype,type,user,host,short,desc,version,filename,inode,notes,format,extra\n")

	// CSV neighbors must fail mac_apt CSV detection.
	for name, path := range map[string]string{"UnifiedLog CSV": ulCSVPath, "EZ Tools CSV": ezPath, "L2T CSV": l2tPath} {
		if _, ok, err := macaptparser.DetectCSV(path); ok || err != nil {
			t.Errorf("%s: mac_apt DetectCSV = (%v, %v), want not detected", name, ok, err)
		}
		f, _, err := DetectFile(path)
		if err != nil {
			t.Errorf("%s: DetectFile: %v", name, err)
			continue
		}
		if strings.HasPrefix(f.Name, "mac_apt") {
			t.Errorf("%s: DetectFile resolved to %q", name, f.Name)
		}
	}

	// JSONL neighbors must fail mac_apt JSONL detection.
	for name, path := range map[string]string{"UnifiedLog JSONL": ulJSONLPath, "Plaso JSONL": plasoPath} {
		if _, ok, err := macaptparser.DetectJSONL(path); ok {
			t.Errorf("%s: mac_apt DetectJSONL accepted it (err %v)", name, err)
		}
		f, _, err := DetectFile(path)
		if err != nil {
			t.Errorf("%s: DetectFile: %v", name, err)
			continue
		}
		if strings.HasPrefix(f.Name, "mac_apt") {
			t.Errorf("%s: DetectFile resolved to %q", name, f.Name)
		}
	}

	// Plaso JSONL must still be the last .jsonl claimant, so a bad .jsonl
	// keeps its historic error text.
	bad := writeTmpCSV(t, dir, "bad.jsonl", "datetime,message\n2026-01-01 00:00:00,x\n")
	_, _, err := DetectFile(bad)
	const want = "invalid JSONL file: first line is not a JSON object"
	if err == nil || err.Error() != want {
		t.Errorf("bad .jsonl error = %v, want %q", err, want)
	}
}

// --- Dedupe through the recursive walk ---

// TestMacAptCSVAndJSONLImportOnce verifies a CSV and JSONL of one output in
// one directory import once, from the JSONL, with the CSV listed as a
// duplicate.
func TestMacAptCSVAndJSONLImportOnce(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "FsEvents.csv", maFsCSV())
	writeTmpCSV(t, root, "FsEvents.jsonl", maFsJSONL())

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, "", nil, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	if summary.TotalFilesProcessed != 1 {
		t.Errorf("TotalFilesProcessed = %d, want 1", summary.TotalFilesProcessed)
	}
	if store.insertedCount != 1 {
		t.Errorf("insertedCount = %d, want 1 (JSONL only)", store.insertedCount)
	}
	sf, ok := findSkip(summary, "FsEvents.csv")
	if !ok {
		t.Fatalf("FsEvents.csv not in SkippedFiles: %v", summary.SkippedFiles)
	}
	if want := "duplicate of FsEvents.jsonl (same mac_apt data)"; sf.Reason != want {
		t.Errorf("FsEvents.csv reason = %q, want %q", sf.Reason, want)
	}
	if fam := summary.ToolFamilies["mac_apt FsEvents"]; fam != FamilyMacApt {
		t.Errorf("ToolFamilies[mac_apt FsEvents] = %q, want %q", fam, FamilyMacApt)
	}
}

// TestMacAptSameNameDifferentDirectories verifies files sharing a base name
// in different directories are both imported.
func TestMacAptSameNameDifferentDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTmpCSV(t, filepath.Join(root, "a"), "FsEvents.csv", maFsCSV())
	writeTmpCSV(t, filepath.Join(root, "b"), "FsEvents.jsonl", maFsJSONL())

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, "", nil, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if summary.TotalFilesProcessed != 2 {
		t.Errorf("TotalFilesProcessed = %d, want 2", summary.TotalFilesProcessed)
	}
	if len(summary.SkippedFiles) != 0 {
		t.Errorf("SkippedFiles = %v, want none", summary.SkippedFiles)
	}
}

// TestMacAptSameNameDifferentOutputs verifies a CSV and JSONL with the same
// base name but different detected outputs are not grouped.
func TestMacAptSameNameDifferentOutputs(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "out.csv", maASLFile())
	writeTmpCSV(t, root, "out.jsonl", maFsJSONL())

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, "", nil, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if summary.TotalFilesProcessed != 2 {
		t.Errorf("TotalFilesProcessed = %d, want 2", summary.TotalFilesProcessed)
	}
	if len(summary.SkippedFiles) != 0 {
		t.Errorf("SkippedFiles = %v, want none", summary.SkippedFiles)
	}
}

// TestMacAptDedupeLeavesUnifiedLogAlone verifies the mac_apt pass does not
// touch a UnifiedLog CSV/JSONL pair, which keeps its own reason text.
func TestMacAptDedupeLeavesUnifiedLogAlone(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "ul.csv", ulCSV(t, ulBootUUID, "2026-10-07T14:00:00.000Z"))
	writeTmpCSV(t, root, "ul.jsonl", ulJSONL(ulBootUUID, "2026-10-07T14:00:00.000000000Z"))

	summary, err := ImportFolderRecursive(root, &mockStore{}, "", nil, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	sf, ok := findSkip(summary, "ul.csv")
	if !ok {
		t.Fatalf("ul.csv not skipped: %v", summary.SkippedFiles)
	}
	if sf.Reason != duplicateReason("ul.jsonl") {
		t.Errorf("ul.csv reason = %q, want UnifiedLog duplicate text", sf.Reason)
	}
}

const maBiomeHeader = "Biome Type,Filename timestamp,Offset,State,Data,Timestamp1,Timestamp2,User,Source"

// maBiomeBadTS is a Biome row whose Timestamp1 is garbage and Timestamp2 is
// valid: one event and one unparseable value.
const maBiomeBadTS = "App.InFocus,,0,,x,not-a-time,2026-03-04 05:06:09,tom,/s\n"

// TestRecursiveSummaryCarriesUnparseable verifies the unparseable count lands
// on the tool that produced it, and on the file, while other tools stay zero.
func TestRecursiveSummaryCarriesUnparseable(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "FsEvents.csv", maFsCSV())
	writeTmpCSV(t, root, "Biome.csv", maBiomeHeader+"\n"+maBiomeBadTS)

	summary, err := ImportFolderRecursive(root, &mockStore{}, "", nil, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}

	biome := summary.PerTool["mac_apt Biome"]
	if biome.EventCount != 1 || biome.UnparseableCount != 1 {
		t.Errorf("mac_apt Biome stats = %+v, want 1 event and 1 unparseable", biome)
	}
	if fs := summary.PerTool["mac_apt FsEvents"]; fs.UnparseableCount != 0 {
		t.Errorf("mac_apt FsEvents UnparseableCount = %d, want 0", fs.UnparseableCount)
	}
	want := []UnparseableFile{{RelativePath: "Biome.csv", Count: 1}}
	if len(summary.UnparseableFiles) != 1 || summary.UnparseableFiles[0] != want[0] {
		t.Errorf("UnparseableFiles = %+v, want %+v", summary.UnparseableFiles, want)
	}
}

// TestRecursiveSummaryCleanHasNoUnparseable verifies a clean import leaves
// the summary's unparseable fields empty.
func TestRecursiveSummaryCleanHasNoUnparseable(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "FsEvents.csv", maFsCSV())

	summary, err := ImportFolderRecursive(root, &mockStore{}, "", nil, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if len(summary.UnparseableFiles) != 0 || summary.PerTool["mac_apt FsEvents"].UnparseableCount != 0 {
		t.Errorf("clean import reported unparseable: %+v", summary)
	}
}

// TestImportFileReportsUnparseable verifies single-file import writes the log
// line and puts the count in the final status message. A clean file writes
// neither.
func TestImportFileReportsUnparseable(t *testing.T) {
	dir := t.TempDir()
	biome := writeTmpCSV(t, dir, "Biome.csv", maBiomeHeader+"\n"+maBiomeBadTS)
	clean := writeTmpCSV(t, dir, "FsEvents.csv", maFsCSV())

	var logs []string
	logf := func(level, msg string) { logs = append(logs, level+" "+msg) }

	format, label, err := DetectFile(biome)
	if err != nil {
		t.Fatalf("DetectFile(biome): %v", err)
	}
	var statuses []string
	p := &Progress{Status: func(m string, _ int) { statuses = append(statuses, m) }}
	if _, err := ImportFile(biome, format, label, "", &mockStore{}, p, logf); err != nil {
		t.Fatalf("ImportFile(biome): %v", err)
	}
	if p.Unparseable != 1 {
		t.Errorf("Progress.Unparseable = %d, want 1", p.Unparseable)
	}
	const wantLog = "INFO Biome.csv: 1 unparseable timestamp value(s) skipped"
	if !containsString(logs, wantLog) {
		t.Errorf("logs = %v, want %q", logs, wantLog)
	}
	if len(statuses) == 0 || !strings.Contains(statuses[len(statuses)-1], "1 unparseable timestamp value(s) skipped") {
		t.Errorf("statuses = %v, want final status with the count", statuses)
	}

	logs = nil
	format, label, err = DetectFile(clean)
	if err != nil {
		t.Fatalf("DetectFile(clean): %v", err)
	}
	p = &Progress{}
	if _, err := ImportFile(clean, format, label, "", &mockStore{}, p, logf); err != nil {
		t.Fatalf("ImportFile(clean): %v", err)
	}
	for _, l := range logs {
		if strings.Contains(l, "unparseable") {
			t.Errorf("clean import logged %q", l)
		}
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
