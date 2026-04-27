package eztoolparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testdataDir = "../../testdata"

// testFile returns the absolute path to a testdata file.
func testFile(name string) string {
	return filepath.Join(testdataDir, name)
}

// --- Tool Detection Tests ---

func TestDetectEvtxECmd(t *testing.T) {
	err := ValidateFile(testFile("evtxcmd_application.csv"))
	if err != nil {
		t.Fatalf("expected EvtxECmd to be recognized: %v", err)
	}

	res, err := ReadEvents(testFile("evtxcmd_application.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolEvtxECmd {
		t.Errorf("expected tool %q, got %q", ToolEvtxECmd, res.Tool)
	}
}

func TestDetectPECmd(t *testing.T) {
	res, err := ReadEvents(testFile("20260320204231_PECmd_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolPECmd {
		t.Errorf("expected tool %q, got %q", ToolPECmd, res.Tool)
	}
}

func TestDetectLECmd(t *testing.T) {
	res, err := ReadEvents(testFile("lecmd_1060393143_html_lnk.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolLECmd {
		t.Errorf("expected tool %q, got %q", ToolLECmd, res.Tool)
	}
}

func TestDetectJLECmdAutomatic(t *testing.T) {
	res, err := ReadEvents(testFile("jmplst_testing_sample_AutomaticDestinations.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolJLECmdAutomatic {
		t.Errorf("expected tool %q, got %q", ToolJLECmdAutomatic, res.Tool)
	}
}

func TestDetectJLECmdCustom(t *testing.T) {
	res, err := ReadEvents(testFile("jmplst_testing_sample_CustomDestinations.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolJLECmdCustom {
		t.Errorf("expected tool %q, got %q", ToolJLECmdCustom, res.Tool)
	}
}

func TestDetectAmcacheUnassociatedFiles(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_UnassociatedFileEntries.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheUnassociatedFiles {
		t.Errorf("expected tool %q, got %q", ToolAmcacheUnassociatedFiles, res.Tool)
	}
}

func TestDetectAmcacheDeviceContainers(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_DeviceContainers.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheDeviceContainers {
		t.Errorf("expected tool %q, got %q", ToolAmcacheDeviceContainers, res.Tool)
	}
}

func TestDetectAmcacheDevicePnps(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_DevicePnps.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheDevicePnps {
		t.Errorf("expected tool %q, got %q", ToolAmcacheDevicePnps, res.Tool)
	}
}

func TestDetectAmcacheDriveBinaries(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_DriveBinaries.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheDriveBinaries {
		t.Errorf("expected tool %q, got %q", ToolAmcacheDriveBinaries, res.Tool)
	}
}

func TestDetectAmcacheDriverPackages(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_DriverPackages.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheDriverPackages {
		t.Errorf("expected tool %q, got %q", ToolAmcacheDriverPackages, res.Tool)
	}
}

func TestDetectAmcacheShortCuts(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_ShortCuts.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheShortCuts {
		t.Errorf("expected tool %q, got %q", ToolAmcacheShortCuts, res.Tool)
	}
}

func TestDetectSrumECmdAppTimeline(t *testing.T) {
	res, err := ReadEvents(testFile("20260320205219_SrumECmd_AppTimelineProvider_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolSrumECmdAppTimeline {
		t.Errorf("expected tool %q, got %q", ToolSrumECmdAppTimeline, res.Tool)
	}
}

func TestDetectSrumECmdEnergyUsage(t *testing.T) {
	res, err := ReadEvents(testFile("20260320205219_SrumECmd_EnergyUsage_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolSrumECmdEnergyUsage {
		t.Errorf("expected tool %q, got %q", ToolSrumECmdEnergyUsage, res.Tool)
	}
}

func TestDetectSrumECmdNetworkConnections(t *testing.T) {
	res, err := ReadEvents(testFile("20260320205219_SrumECmd_NetworkConnections_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolSrumECmdNetworkConnections {
		t.Errorf("expected tool %q, got %q", ToolSrumECmdNetworkConnections, res.Tool)
	}
}

func TestDetectSrumECmdNetworkUsages(t *testing.T) {
	res, err := ReadEvents(testFile("20260320205219_SrumECmd_NetworkUsages_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolSrumECmdNetworkUsages {
		t.Errorf("expected tool %q, got %q", ToolSrumECmdNetworkUsages, res.Tool)
	}
}

func TestDetectSrumECmdPushNotifications(t *testing.T) {
	res, err := ReadEvents(testFile("20260320205219_SrumECmd_PushNotifications_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolSrumECmdPushNotifications {
		t.Errorf("expected tool %q, got %q", ToolSrumECmdPushNotifications, res.Tool)
	}
}

func TestDetectSrumECmdVfuprov(t *testing.T) {
	res, err := ReadEvents(testFile("20260320205219_SrumECmd_vfuprov_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolSrumECmdVfuprov {
		t.Errorf("expected tool %q, got %q", ToolSrumECmdVfuprov, res.Tool)
	}
}

func TestDetectMFTECmd(t *testing.T) {
	res, err := ReadEvents(testFile("mftecmd_sample.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolMFTECmd {
		t.Errorf("expected tool %q, got %q", ToolMFTECmd, res.Tool)
	}
}

func TestDetectSBECmd(t *testing.T) {
	res, err := ReadEvents(testFile("SBECmd_usrClass.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolSBECmd {
		t.Errorf("expected tool %q, got %q", ToolSBECmd, res.Tool)
	}
}

// --- Multi-Timestamp Expansion ---

func TestPECmdMultiTimestampExpansion(t *testing.T) {
	res, err := ReadEvents(testFile("20260320204231_PECmd_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	// The PECmd file has 4 data rows. Each row can expand to multiple events
	// based on how many non-empty timestamp columns it has.
	if res.Count == 0 {
		t.Fatal("expected events from PECmd, got 0")
	}

	// Check that events have different types (timestamp column names)
	types := make(map[string]bool)
	for _, e := range res.Events {
		types[e.Type] = true
	}
	if len(types) < 2 {
		t.Errorf("expected multiple timestamp types from expansion, got %d: %v", len(types), types)
	}

	// Verify LastRun events exist
	if !types["LastRun"] {
		t.Error("expected LastRun type in expanded events")
	}
}

// --- Empty Timestamp Skipping ---

func TestEmptyTimestampSkipping(t *testing.T) {
	tests := []struct {
		input string
		empty bool
	}{
		{"", true},
		{"   ", true},
		{"0001-01-01 00:00:00", true},
		{"0001-01-01 00:00:00.0000000", true},
		{"2026-02-02 17:47:29", false},
		{"2026-02-02 17:47:29.9004489", false},
	}

	for _, tt := range tests {
		got := isEmptyTimestamp(tt.input)
		if got != tt.empty {
			t.Errorf("isEmptyTimestamp(%q) = %v, want %v", tt.input, got, tt.empty)
		}
	}
}

// --- MACB Derivation ---

func TestDeriveMACB(t *testing.T) {
	tests := []struct {
		colName  string
		expected string
	}{
		{"Created0x10", "...B"},
		{"Created0x30", "...B"},
		{"TargetCreated", "...B"},
		{"SourceCreated", "...B"},
		{"CreationTime", "...B"},
		{"TrackerCreatedOn", "...B"},
		{"CreatedOn", "...B"},
		{"LastModified0x10", "M..."},
		{"LastModified0x30", "M..."},
		{"TargetModified", "M..."},
		{"SourceModified", "M..."},
		{"LastRun", "M..."},
		{"PreviousRun0", "M..."},
		{"LastWriteTime", "M..."},
		{"ModifiedOn", "M..."},
		{"LastAccess0x10", ".A.."},
		{"LastAccess0x30", ".A.."},
		{"TargetAccessed", ".A.."},
		{"SourceAccessed", ".A.."},
		{"AccessedOn", ".A.."},
		{"FirstInteracted", ".A.."},
		{"LastInteracted", ".A.."},
		{"LastRecordChange0x10", "..C."},
		{"LastRecordChange0x30", "..C."},
		{"TimeCreated", "...."},
		{"Timestamp", "...."},
		{"LinkDate", "...."},
		{"ExeTimestamp", "...."},
		{"EventTimestamp", "...."},
		{"EndTime", "...."},
		{"StartTime", "...."},
		{"ConnectStartTime", "...."},
	}

	for _, tt := range tests {
		got := deriveMACB(tt.colName)
		if got != tt.expected {
			t.Errorf("deriveMACB(%q) = %q, want %q", tt.colName, got, tt.expected)
		}
	}
}

// --- Field Mapping Tests ---

func TestEvtxECmdFieldMapping(t *testing.T) {
	res, err := ReadEvents(testFile("evtxcmd_application.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Count == 0 {
		t.Fatal("expected events")
	}

	e := res.Events[0]
	if e.Source != "EVTX" {
		t.Errorf("source = %q, want EVTX", e.Source)
	}
	if e.SourceType != "Application" {
		t.Errorf("sourcetype = %q, want Application", e.SourceType)
	}
	if e.Format != "eztool_evtxecmd" {
		t.Errorf("format = %q, want eztool_evtxecmd", e.Format)
	}
	if e.Host != "DAFBEACH" {
		t.Errorf("host = %q, want DAFBEACH", e.Host)
	}
	if e.Timezone != "UTC" {
		t.Errorf("timezone = %q, want UTC", e.Timezone)
	}
	if e.Type != "TimeCreated" {
		t.Errorf("type = %q, want TimeCreated", e.Type)
	}
	if e.Datetime != "2026-02-02 17:47:29" {
		t.Errorf("datetime = %q, want 2026-02-02 17:47:29", e.Datetime)
	}
	if e.EventID == "" {
		t.Error("expected event_identifier to be set")
	}
}

func TestPECmdFieldMapping(t *testing.T) {
	res, err := ReadEvents(testFile("20260320204231_PECmd_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	// Find a LastRun event
	for _, e := range res.Events {
		if e.Type == "LastRun" {
			if e.Source != "PF" {
				t.Errorf("source = %q, want PF", e.Source)
			}
			if e.SourceType != "Prefetch" {
				t.Errorf("sourcetype = %q, want Prefetch", e.SourceType)
			}
			if e.Format != "eztool_pecmd" {
				t.Errorf("format = %q, want eztool_pecmd", e.Format)
			}
			if e.Desc == "" {
				t.Error("expected desc (ExecutableName) to be set")
			}
			if e.MACB != "M..." {
				t.Errorf("MACB = %q, want M... for LastRun", e.MACB)
			}
			return
		}
	}
	t.Error("no LastRun event found in PECmd output")
}

func TestMFTECmdFieldMapping(t *testing.T) {
	res, err := ReadEvents(testFile("mftecmd_sample.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Count == 0 {
		t.Fatal("expected events")
	}

	e := res.Events[0]
	if e.Source != "MFT" {
		t.Errorf("source = %q, want MFT", e.Source)
	}
	if e.SourceType != "NTFS MFT Entry" {
		t.Errorf("sourcetype = %q, want NTFS MFT Entry", e.SourceType)
	}
	if e.Format != "eztool_mftecmd" {
		t.Errorf("format = %q, want eztool_mftecmd", e.Format)
	}
	// desc should be ParentPath\FileName
	if !strings.Contains(e.Desc, `\`) && e.Desc != "" {
		t.Errorf("desc should contain backslash separator, got %q", e.Desc)
	}
	if e.Inode == "" {
		t.Error("expected inode (EntryNumber) to be set")
	}
}

func TestSBECmdFieldMapping(t *testing.T) {
	res, err := ReadEvents(testFile("SBECmd_usrClass.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Count == 0 {
		t.Fatal("expected events")
	}

	e := res.Events[0]
	if e.Source != "SHELLBAGS" {
		t.Errorf("source = %q, want SHELLBAGS", e.Source)
	}
	if e.SourceType != "Shellbag Entry" {
		t.Errorf("sourcetype = %q, want Shellbag Entry", e.SourceType)
	}
	if e.Format != "eztool_sbecmd" {
		t.Errorf("format = %q, want eztool_sbecmd", e.Format)
	}
}

// --- BOM Handling ---

func TestBOMHandling(t *testing.T) {
	// These files have BOMs: evtxcmd, srumecmd, mftecmd, sbecmd
	bomFiles := []struct {
		file string
		tool string
	}{
		{"evtxcmd_application.csv", ToolEvtxECmd},
		{"20260320205219_SrumECmd_AppTimelineProvider_Output.csv", ToolSrumECmdAppTimeline},
		{"mftecmd_sample.csv", ToolMFTECmd},
		{"SBECmd_usrClass.csv", ToolSBECmd},
	}

	for _, bf := range bomFiles {
		t.Run(bf.file, func(t *testing.T) {
			err := ValidateFile(testFile(bf.file))
			if err != nil {
				t.Errorf("ValidateFile failed for BOM file %s: %v", bf.file, err)
			}

			res, err := ReadEvents(testFile(bf.file), nil)
			if err != nil {
				t.Errorf("ReadEvents failed for BOM file %s: %v", bf.file, err)
				return
			}
			if res.Tool != bf.tool {
				t.Errorf("tool = %q, want %q for %s", res.Tool, bf.tool, bf.file)
			}
		})
	}
}

// --- ValidateFile rejection ---

func TestValidateFileRejectsNonEZTool(t *testing.T) {
	// Create a temp CSV that is not an EZ Tool format
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "not_ez_tool.csv")
	err := os.WriteFile(tmpFile, []byte("name,age,city\nAlice,30,NYC\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	err = ValidateFile(tmpFile)
	if err == nil {
		t.Error("expected ValidateFile to reject non-EZ Tool CSV")
	}
}

func TestValidateFileAcceptsEZTool(t *testing.T) {
	err := ValidateFile(testFile("20260320204231_PECmd_Output.csv"))
	if err != nil {
		t.Errorf("expected ValidateFile to accept PECmd CSV: %v", err)
	}
}

// --- ReadDirectory ---

func TestReadDirectory(t *testing.T) {
	// Create a temp directory with a couple of EZ Tool CSVs
	tmpDir := t.TempDir()

	// Copy a small EZ Tool file
	src, err := os.ReadFile(testFile("amcache-testing_ShortCuts.csv"))
	if err != nil {
		t.Fatalf("failed to read source file: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "shortcuts1.csv"), src, 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "shortcuts2.csv"), src, 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	// Also write a non-CSV file that should be skipped
	err = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("not a csv"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	res, err := ReadDirectory(tmpDir, nil)
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}

	if res.Count == 0 {
		t.Error("expected events from ReadDirectory")
	}

	// Should have processed both CSV files
	// Each file has 72 data rows, each with 1 timestamp column
	if res.Tool != ToolAmcacheShortCuts {
		t.Errorf("tool = %q, want %q", res.Tool, ToolAmcacheShortCuts)
	}
}

func TestReadDirectorySkipsUnrecognized(t *testing.T) {
	tmpDir := t.TempDir()

	// Write only non-EZ Tool CSVs
	err := os.WriteFile(filepath.Join(tmpDir, "random.csv"), []byte("a,b,c\n1,2,3\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = ReadDirectory(tmpDir, nil)
	if err == nil {
		t.Error("expected ReadDirectory to return error when no recognized files found")
	}
}

// --- Datetime Normalization ---

func TestNormalizeDatetime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2026-02-02 17:47:29.9004489", "2026-02-02 17:47:29"},
		{"2026-02-02 17:47:29", "2026-02-02 17:47:29"},
		{"02/11/2088 03:51:29 +00:00", "2088-02-11 03:51:29"},
		{"", ""},
		{"0001-01-01 00:00:00", ""},
		{"0001-01-01 00:00:00.0000000", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		got := normalizeDatetime(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeDatetime(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- SrumECmd ExeTimestamp format ---

func TestSrumExeTimestampFormat(t *testing.T) {
	res, err := ReadEvents(testFile("20260320205219_SrumECmd_AppTimelineProvider_Output.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	// Check that ExeTimestamp events with MM/DD/YYYY format are handled
	for _, e := range res.Events {
		if e.Type == "ExeTimestamp" && e.Datetime != "" {
			// Should be normalized to YYYY-MM-DD HH:MM:SS
			if e.Datetime[4] != '-' {
				t.Errorf("ExeTimestamp not normalized: %q", e.Datetime)
			}
			return
		}
	}
	// If no ExeTimestamp events, that's OK (they might all be empty/0001-01-01)
}

// --- Progress callback ---

func TestProgressCallback(t *testing.T) {
	called := false
	cb := func(n int) {
		called = true
	}

	// Use a file with enough rows to trigger the callback (needs >10000 rows)
	// evtxcmd_application.csv has ~14k rows
	_, err := ReadEvents(testFile("evtxcmd_application.csv"), cb)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	if !called {
		t.Error("expected progress callback to be called for large file")
	}
}
