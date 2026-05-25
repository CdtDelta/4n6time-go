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

func TestDetectMFTECmdMFT(t *testing.T) {
	res, err := ReadEvents(testFile("mftecmd_sample.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolMFTECmdMFT {
		t.Errorf("expected tool %q, got %q", ToolMFTECmdMFT, res.Tool)
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

func TestDetectMFTECmdBoot(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "MFTECmd_$Boot_Output.csv")
	content := "EntryPoint,Signature,BytesPerSector,SectorsPerCluster,MFTCluster,VolumeSize,VolumeSerialNumber\n" +
		"0,NTFS,512,8,786432,976773120,4A5E3C2D1B6F\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolMFTECmdBoot {
		t.Errorf("expected tool %q, got %q", ToolMFTECmdBoot, tool)
	}
}

func TestDetectMFTECmdSDS(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "MFTECmd_$SDS_Output.csv")
	content := "Hash,Offset,Length,OwnerSid,GroupSid,SaclAceCount,DaclAceCount\n" +
		"3F2A1B4C,0,168,S-1-5-32-544,S-1-5-18,0,2\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolMFTECmdSDS {
		t.Errorf("expected tool %q, got %q", ToolMFTECmdSDS, tool)
	}
}

func TestNoTimestampFormatsContainsBoot(t *testing.T) {
	if _, ok := NoTimestampFormats[ToolMFTECmdBoot]; !ok {
		t.Errorf("NoTimestampFormats missing %q", ToolMFTECmdBoot)
	}
}

func TestNoTimestampFormatsContainsSDS(t *testing.T) {
	if _, ok := NoTimestampFormats[ToolMFTECmdSDS]; !ok {
		t.Errorf("NoTimestampFormats missing %q", ToolMFTECmdSDS)
	}
}

func TestMFTECmdBootReadEventsZeroEvents(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "MFTECmd_$Boot_Output.csv")
	content := "EntryPoint,Signature,BytesPerSector,SectorsPerCluster,MFTCluster\n" +
		"0,NTFS,512,8,786432\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolMFTECmdBoot {
		t.Errorf("expected tool %q, got %q", ToolMFTECmdBoot, res.Tool)
	}
	if len(res.Events) != 0 {
		t.Errorf("expected 0 events for $Boot, got %d", len(res.Events))
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
	if e.Source != "FILESYSTEM" {
		t.Errorf("source = %q, want FILESYSTEM", e.Source)
	}
	if e.SourceType != "MFT" {
		t.Errorf("sourcetype = %q, want MFT", e.SourceType)
	}
	if e.Format != "eztool_mftecmd" {
		t.Errorf("format = %q, want eztool_mftecmd", e.Format)
	}
	// desc should be the FileName column value (e.g. "$MFT")
	if e.Desc == "" {
		t.Error("expected non-empty desc (FileName)")
	}
	// Filename should be the ParentPath column value
	if e.Filename == "" {
		t.Error("expected non-empty filename (ParentPath)")
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
		{"mftecmd_sample.csv", ToolMFTECmdMFT},
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

// --- Amcache subtype round-trip tests ---

func TestAmcacheDeviceContainersRoundTrip(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_DeviceContainers.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheDeviceContainers {
		t.Errorf("tool = %q, want %q", res.Tool, ToolAmcacheDeviceContainers)
	}
	if res.Count == 0 {
		t.Fatal("expected events, got 0")
	}
	e := res.Events[0]
	if e.Source != "AMCACHE" {
		t.Errorf("source = %q, want AMCACHE", e.Source)
	}
	if e.SourceType != "Amcache DeviceContainers" {
		t.Errorf("sourcetype = %q, want Amcache DeviceContainers", e.SourceType)
	}
	if e.Format != "eztool_amcacheparser" {
		t.Errorf("format = %q, want eztool_amcacheparser", e.Format)
	}
	// All events must come from KeyLastWriteTimestamp (only timestamp column).
	for _, ev := range res.Events {
		if ev.Type != "KeyLastWriteTimestamp" {
			t.Errorf("unexpected event type %q (only KeyLastWriteTimestamp expected)", ev.Type)
		}
	}
	// Desc must be non-empty (FriendlyName or KeyName fallback).
	if e.Desc == "" {
		t.Error("expected non-empty desc")
	}
}

func TestAmcacheDevicePnpsRoundTrip(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_DevicePnps.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheDevicePnps {
		t.Errorf("tool = %q, want %q", res.Tool, ToolAmcacheDevicePnps)
	}
	if res.Count == 0 {
		t.Fatal("expected events, got 0")
	}
	e := res.Events[0]
	if e.Source != "AMCACHE" {
		t.Errorf("source = %q, want AMCACHE", e.Source)
	}
	if e.SourceType != "Amcache DevicePnps" {
		t.Errorf("sourcetype = %q, want Amcache DevicePnps", e.SourceType)
	}
	if e.Format != "eztool_amcacheparser" {
		t.Errorf("format = %q, want eztool_amcacheparser", e.Format)
	}
	// Desc must be non-empty (Description or KeyName fallback).
	if e.Desc == "" {
		t.Error("expected non-empty desc")
	}
	// Notes should be non-empty for rows where Class/Manufacturer/DriverName/DriverVerVersion have values.
	if e.Notes == "" {
		t.Error("expected non-empty notes (Class, Manufacturer, DriverName, DriverVerVersion)")
	}
}

func TestAmcacheDriveBinariesRoundTrip(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_DriveBinaries.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheDriveBinaries {
		t.Errorf("tool = %q, want %q", res.Tool, ToolAmcacheDriveBinaries)
	}
	if res.Count == 0 {
		t.Fatal("expected events, got 0")
	}
	e := res.Events[0]
	if e.Source != "AMCACHE" {
		t.Errorf("source = %q, want AMCACHE", e.Source)
	}
	if e.SourceType != "Amcache DriveBinaries" {
		t.Errorf("sourcetype = %q, want Amcache DriveBinaries", e.SourceType)
	}
	if e.Format != "eztool_amcacheparser" {
		t.Errorf("format = %q, want eztool_amcacheparser", e.Format)
	}
	// DriveBinaries has three timestamp columns; rows with all three populated
	// should produce three events. Verify at least two distinct types are present.
	types := make(map[string]bool)
	for _, ev := range res.Events {
		types[ev.Type] = true
	}
	for _, want := range []string{"KeyLastWriteTimestamp", "DriverTimeStamp", "DriverLastWriteTime"} {
		if !types[want] {
			t.Errorf("expected event type %q to be present", want)
		}
	}
}

func TestAmcacheDriverPackagesRoundTrip(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_DriverPackages.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheDriverPackages {
		t.Errorf("tool = %q, want %q", res.Tool, ToolAmcacheDriverPackages)
	}
	if res.Count == 0 {
		t.Fatal("expected events, got 0")
	}
	e := res.Events[0]
	if e.Source != "AMCACHE" {
		t.Errorf("source = %q, want AMCACHE", e.Source)
	}
	if e.SourceType != "Amcache DriverPackages" {
		t.Errorf("sourcetype = %q, want Amcache DriverPackages", e.SourceType)
	}
	if e.Format != "eztool_amcacheparser" {
		t.Errorf("format = %q, want eztool_amcacheparser", e.Format)
	}
	// DriverPackages has two timestamp columns; verify both types appear.
	types := make(map[string]bool)
	for _, ev := range res.Events {
		types[ev.Type] = true
	}
	for _, want := range []string{"KeyLastWriteTimestamp", "Date"} {
		if !types[want] {
			t.Errorf("expected event type %q to be present", want)
		}
	}
	// Desc must be non-empty (Inf or KeyName fallback).
	if e.Desc == "" {
		t.Error("expected non-empty desc")
	}
}

func TestAmcacheShortCutsRoundTrip(t *testing.T) {
	res, err := ReadEvents(testFile("amcache-testing_ShortCuts.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheShortCuts {
		t.Errorf("tool = %q, want %q", res.Tool, ToolAmcacheShortCuts)
	}
	if res.Count == 0 {
		t.Fatal("expected events, got 0")
	}
	e := res.Events[0]
	if e.Source != "AMCACHE" {
		t.Errorf("source = %q, want AMCACHE", e.Source)
	}
	if e.SourceType != "Amcache ShortCuts" {
		t.Errorf("sourcetype = %q, want Amcache ShortCuts", e.SourceType)
	}
	if e.Format != "eztool_amcacheparser" {
		t.Errorf("format = %q, want eztool_amcacheparser", e.Format)
	}
	// All events from KeyLastWriteTimestamp only.
	for _, ev := range res.Events {
		if ev.Type != "KeyLastWriteTimestamp" {
			t.Errorf("unexpected event type %q", ev.Type)
		}
	}
	if e.Desc == "" {
		t.Error("expected non-empty desc (LnkName)")
	}
	if e.Notes == "" {
		t.Error("expected non-empty notes (KeyName)")
	}
}

// --- RBCmd tests ---

func TestDetectRBCmd(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "RBCmd_Output.csv")
	content := "SourceName,FileType,FileName,FileSize,DeletedOn\n" +
		"$IABCD1234.docx,$I,ImportantDoc.docx,45678,2022-12-25 23:06:03\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolRBCmd {
		t.Errorf("expected tool %q, got %q", ToolRBCmd, tool)
	}
}

func TestRBCmdRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "RBCmd_Output.csv")
	content := "SourceName,FileType,FileName,FileSize,DeletedOn\n" +
		"$IABCD1234.docx,$I,ImportantDoc.docx,45678,2022-12-25 23:06:03\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolRBCmd {
		t.Errorf("tool = %q, want %q", res.Tool, ToolRBCmd)
	}
	if res.Count != 1 {
		t.Fatalf("expected 1 event, got %d", res.Count)
	}
	e := res.Events[0]
	if e.Source != "FILESYSTEM" {
		t.Errorf("source = %q, want FILESYSTEM", e.Source)
	}
	if e.SourceType != "$Recycle.Bin" {
		t.Errorf("sourcetype = %q, want $Recycle.Bin", e.SourceType)
	}
	if e.Format != "eztool_rbcmd" {
		t.Errorf("format = %q, want eztool_rbcmd", e.Format)
	}
	if e.Desc != "ImportantDoc.docx" {
		t.Errorf("desc = %q, want ImportantDoc.docx", e.Desc)
	}
	if e.Type != "DeletedOn" {
		t.Errorf("type = %q, want DeletedOn", e.Type)
	}
	if e.Datetime != "2022-12-25 23:06:03" {
		t.Errorf("datetime = %q, want 2022-12-25 23:06:03", e.Datetime)
	}
	if e.MACB != "M..." {
		t.Errorf("MACB = %q, want M...", e.MACB)
	}
}

func TestRBCmdSkipsEmptyDeletedOn(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "RBCmd_Output.csv")
	content := "SourceName,FileType,FileName,FileSize,DeletedOn\n" +
		"$IABCD1234.docx,$I,ImportantDoc.docx,45678,2022-12-25 23:06:03\n" +
		"$IXXXX9999.png,$I,Photo.png,102400,\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	// Only the first row has a DeletedOn value; the second must be skipped.
	if res.Count != 1 {
		t.Errorf("expected 1 event (empty DeletedOn row skipped), got %d", res.Count)
	}
}

// --- AppCompatCacheParser tests ---

func TestDetectAppCompatCacheParser(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "AppCompatCacheParser_Output.csv")
	content := "SourceFile,ControlSet,CacheEntryPosition,Path,LastModifiedTimeUTC,Executed,Duplicate\n" +
		"shimcache.bin,ControlSet001,0,C:\\Windows\\System32\\svchost.exe,2026-04-02 00:41:02,Yes,False\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolAppCompatCacheParser {
		t.Errorf("expected tool %q, got %q", ToolAppCompatCacheParser, tool)
	}
}

func TestAppCompatCacheParserRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "AppCompatCacheParser_Output.csv")
	content := "SourceFile,ControlSet,CacheEntryPosition,Path,LastModifiedTimeUTC,Executed,Duplicate\n" +
		"shimcache.bin,ControlSet001,0,C:\\Windows\\System32\\svchost.exe,2026-04-02 00:41:02,Yes,False\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAppCompatCacheParser {
		t.Errorf("tool = %q, want %q", res.Tool, ToolAppCompatCacheParser)
	}
	if res.Count != 1 {
		t.Fatalf("expected 1 event, got %d", res.Count)
	}
	e := res.Events[0]
	if e.Source != "REGISTRY" {
		t.Errorf("source = %q, want REGISTRY", e.Source)
	}
	if e.SourceType != "AppCompatCache" {
		t.Errorf("sourcetype = %q, want AppCompatCache", e.SourceType)
	}
	if e.Format != "eztool_appcompatcacheparser" {
		t.Errorf("format = %q, want eztool_appcompatcacheparser", e.Format)
	}
	if e.Desc != `C:\Windows\System32\svchost.exe` {
		t.Errorf("desc = %q, want C:\\Windows\\System32\\svchost.exe", e.Desc)
	}
	if e.Type != "LastModifiedTimeUTC" {
		t.Errorf("type = %q, want LastModifiedTimeUTC", e.Type)
	}
	if e.Datetime != "2026-04-02 00:41:02" {
		t.Errorf("datetime = %q, want 2026-04-02 00:41:02", e.Datetime)
	}
	if !strings.Contains(e.Notes, "ControlSet001") {
		t.Errorf("notes = %q, expected ControlSet001", e.Notes)
	}
}

func TestAppCompatCacheParserSkipsNAAndEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "AppCompatCacheParser_Output.csv")
	content := "SourceFile,ControlSet,CacheEntryPosition,Path,LastModifiedTimeUTC,Executed,Duplicate\n" +
		"shimcache.bin,ControlSet001,0,C:\\Windows\\System32\\svchost.exe,2026-04-02 00:41:02,Yes,False\n" +
		"shimcache.bin,ControlSet001,1,C:\\test.exe,NA,Yes,False\n" +
		"shimcache.bin,ControlSet001,2,C:\\another.exe,,Yes,False\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	// Only the first row has a parseable timestamp; NA and empty rows must be skipped.
	if res.Count != 1 {
		t.Errorf("expected 1 event (NA and empty rows skipped), got %d", res.Count)
	}
}

// --- SrumECmd AppResourceUseInfo tests ---

func TestDetectSrumECmdAppResourceUseInfo(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "SrumECmd_AppResourceUseInfo_Output.csv")
	content := "Id,Timestamp,ExeInfo,SidType,UserName,Sid,ExeInfoDescription," +
		"BackgroundBytesRead,BackgroundBytesWritten,ForegroundBytesRead,ForegroundBytesWritten,FaceTime\n" +
		"1,2026-04-26 18:42:46.8517087,C:\\test.exe,UserSid,TestUser,S-1-5-21-123,Test App," +
		"1024,512,2048,1536,3000\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolSrumECmdAppResourceUseInfo {
		t.Errorf("expected tool %q, got %q", ToolSrumECmdAppResourceUseInfo, tool)
	}
}

func TestSrumECmdAppResourceUseInfoRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "SrumECmd_AppResourceUseInfo_Output.csv")
	content := "Id,Timestamp,ExeInfo,SidType,UserName,Sid,ExeInfoDescription," +
		"BackgroundBytesRead,BackgroundBytesWritten,ForegroundBytesRead,ForegroundBytesWritten,FaceTime\n" +
		"1,2026-04-26 18:42:46.8517087,C:\\test.exe,UserSid,TestUser,S-1-5-21-123,Test App," +
		"1024,512,2048,1536,3000\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolSrumECmdAppResourceUseInfo {
		t.Errorf("tool = %q, want %q", res.Tool, ToolSrumECmdAppResourceUseInfo)
	}
	if res.Count == 0 {
		t.Fatal("expected events, got 0")
	}
	e := res.Events[0]
	if e.Source != "SRUM" {
		t.Errorf("source = %q, want SRUM", e.Source)
	}
	if e.SourceType != "SRUM App Resource Use" {
		t.Errorf("sourcetype = %q, want SRUM App Resource Use", e.SourceType)
	}
	if e.Format != "eztool_srumecmd" {
		t.Errorf("format = %q, want eztool_srumecmd", e.Format)
	}
	if e.Desc != `C:\test.exe` {
		t.Errorf("desc = %q, want C:\\test.exe", e.Desc)
	}
	if e.Type != "Timestamp" {
		t.Errorf("type = %q, want Timestamp", e.Type)
	}
	// Extra must contain AppResourceUseInfo-specific fields.
	if !strings.Contains(e.Extra, "BackgroundBytesRead") {
		t.Errorf("extra = %q, expected BackgroundBytesRead", e.Extra)
	}
}

// --- AmcacheParser AssociatedFileEntries tests ---

func TestDetectAmcacheAssociatedFileEntries(t *testing.T) {
	tmpDir := t.TempDir()
	// Filename contains "associatedfile" (but not "unassociated") — must detect as Associated.
	path := filepath.Join(tmpDir, "amcache-testing_AssociatedFileEntries.csv")
	content := "ApplicationName,ProgramId,SHA1,FileKeyLastWriteTimestamp,FullPath,LinkDate\n" +
		"TestApp,{abc},da39a3ee5e,2026-01-15 10:00:00,C:\\test.exe,2026-01-01 00:00:00\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolAmcacheAssociatedFileEntries {
		t.Errorf("expected tool %q, got %q", ToolAmcacheAssociatedFileEntries, tool)
	}
}

func TestDetectAmcacheUnassociatedFileEntriesStillWorks(t *testing.T) {
	// The existing testdata filename "amcache-testing_UnassociatedFileEntries.csv" contains
	// "unassociated", so it must still resolve to ToolAmcacheUnassociatedFiles even though
	// "unassociatedfileentries" contains "associatedfile" as a substring.
	res, err := ReadEvents(testFile("amcache-testing_UnassociatedFileEntries.csv"), nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheUnassociatedFiles {
		t.Errorf("expected tool %q, got %q", ToolAmcacheUnassociatedFiles, res.Tool)
	}
}

// --- AmcacheParser ProgramEntries tests ---

func TestDetectAmcacheProgramEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "amcache-testing_ProgramEntries.csv")
	content := "Name,ProgramId,Publisher,Version,Language,InstallDate," +
		"InstallDateArpLastModified,InstallDateMsi,InstallDateFromLinkFile,KeyLastWriteTimestamp\n" +
		"TestApp,{abc123},Acme,1.0,1033,2026-01-10 00:00:00," +
		"2026-01-11 00:00:00,2026-01-12 00:00:00,2026-01-09 00:00:00,2026-01-15 00:00:00\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolAmcacheProgramEntries {
		t.Errorf("expected tool %q, got %q", ToolAmcacheProgramEntries, tool)
	}
}

func TestAmcacheProgramEntriesRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "amcache-testing_ProgramEntries.csv")
	content := "Name,ProgramId,Publisher,Version,Language,InstallDate," +
		"InstallDateArpLastModified,InstallDateMsi,InstallDateFromLinkFile,KeyLastWriteTimestamp\n" +
		"TestApp,{abc123},Acme,1.0,1033,2026-01-10 00:00:00," +
		"2026-01-11 00:00:00,2026-01-12 00:00:00,2026-01-09 00:00:00,2026-01-15 00:00:00\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolAmcacheProgramEntries {
		t.Errorf("tool = %q, want %q", res.Tool, ToolAmcacheProgramEntries)
	}
	if res.Count == 0 {
		t.Fatal("expected events, got 0")
	}
	// The single row has 5 non-empty timestamp columns — expect 5 events.
	if res.Count != 5 {
		t.Errorf("expected 5 events (one per timestamp column), got %d", res.Count)
	}
	e := res.Events[0]
	if e.Source != "AMCACHE" {
		t.Errorf("source = %q, want AMCACHE", e.Source)
	}
	if e.SourceType != "Amcache ProgramEntries" {
		t.Errorf("sourcetype = %q, want Amcache ProgramEntries", e.SourceType)
	}
	if e.Format != "eztool_amcacheparser" {
		t.Errorf("format = %q, want eztool_amcacheparser", e.Format)
	}
	if e.Desc != "TestApp" {
		t.Errorf("desc = %q, want TestApp", e.Desc)
	}
	// Verify all 5 timestamp types appear.
	types := make(map[string]bool)
	for _, ev := range res.Events {
		types[ev.Type] = true
	}
	for _, want := range []string{
		"KeyLastWriteTimestamp", "InstallDate",
		"InstallDateArpLastModified", "InstallDateMsi", "InstallDateFromLinkFile",
	} {
		if !types[want] {
			t.Errorf("expected event type %q to be present", want)
		}
	}
}

// --- MFTECmd $J tests ---

func TestDetectMFTECmdJ(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "MFTECmd_$J_Output.csv")
	content := "EntryNumber,SequenceNumber,Name,ParentEntryNumber,ParentSequenceNumber," +
		"ParentPath,UpdateTimestamp,UpdateReasons,UpdateSequenceNumber\n" +
		"39,1,$MFT,5,5,.,2026-03-01 10:00:00,DataExtend,123456789\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolMFTECmdJ {
		t.Errorf("expected tool %q, got %q", ToolMFTECmdJ, tool)
	}
}

func TestMFTECmdJRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "MFTECmd_$J_Output.csv")
	content := "EntryNumber,SequenceNumber,Name,ParentEntryNumber,ParentSequenceNumber," +
		"ParentPath,UpdateTimestamp,UpdateReasons,UpdateSequenceNumber\n" +
		"39,1,important.docx,5,5,\\Users\\test,2026-03-01 10:00:00,DataExtend,123456789\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolMFTECmdJ {
		t.Errorf("tool = %q, want %q", res.Tool, ToolMFTECmdJ)
	}
	if res.Count != 1 {
		t.Fatalf("expected 1 event, got %d", res.Count)
	}
	e := res.Events[0]
	if e.Source != "FILESYSTEM" {
		t.Errorf("source = %q, want FILESYSTEM", e.Source)
	}
	if e.SourceType != "MFT Journal" {
		t.Errorf("sourcetype = %q, want MFT Journal", e.SourceType)
	}
	if e.Format != "eztool_mftecmd" {
		t.Errorf("format = %q, want eztool_mftecmd", e.Format)
	}
	if e.Desc != "important.docx" {
		t.Errorf("desc = %q, want important.docx", e.Desc)
	}
	if e.Type != "UpdateTimestamp" {
		t.Errorf("type = %q, want UpdateTimestamp", e.Type)
	}
	if e.Datetime != "2026-03-01 10:00:00" {
		t.Errorf("datetime = %q, want 2026-03-01 10:00:00", e.Datetime)
	}
	if !strings.Contains(e.Extra, "UpdateReasons") {
		t.Errorf("extra = %q, expected UpdateReasons", e.Extra)
	}
}

// --- Source normalization ---

func TestSourceNormalizationUppercase(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "RBCmd_Output.csv")
	content := "SourceName,FileType,FileName,FileSize,DeletedOn\n" +
		"$IABCD1234.docx,$I,ImportantDoc.docx,45678,2022-12-25 23:06:03\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Count == 0 {
		t.Fatal("expected events")
	}
	for _, e := range res.Events {
		if e.Source != strings.ToUpper(e.Source) {
			t.Errorf("Source %q is not uppercase", e.Source)
		}
	}
}

// --- WxTCmd tests ---

func TestDetectWxTCmdActivity(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "WxTCmd_Activity.csv")
	content := "ActivityTypeOrg,ActivityType,Executable,DisplayText,StartTime,EndTime," +
		"LastModifiedTime,LastModifiedOnClient,OriginalLastModifiedOnClient," +
		"DevicePlatform,TimeZone,Duration,IsLocalOnly,CreatedInCloud,ETag,ExpirationTime\n" +
		"Microsoft.Windows.Photos,InFocus,C:\\Windows\\Photos.exe,Photos," +
		"2026-01-10 09:00:00,2026-01-10 09:15:00," +
		"2026-01-10 09:15:00,2026-01-10 09:15:00,2026-01-10 09:00:00," +
		"Windows.Desktop,UTC,900,0,0,abc123def,2026-03-10 09:00:00\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolWxTCmdActivity {
		t.Errorf("expected tool %q, got %q", ToolWxTCmdActivity, tool)
	}
}

func TestWxTCmdActivityRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "WxTCmd_Activity.csv")
	content := "ActivityTypeOrg,ActivityType,Executable,DisplayText,StartTime,EndTime," +
		"LastModifiedTime,LastModifiedOnClient,OriginalLastModifiedOnClient," +
		"DevicePlatform,TimeZone,Duration,IsLocalOnly,CreatedInCloud,ETag,ExpirationTime\n" +
		"Microsoft.Windows.Photos,InFocus,C:\\Windows\\Photos.exe,Photos," +
		"2026-01-10 09:00:00,2026-01-10 09:15:00," +
		",,," +
		"Windows.Desktop,UTC,900,0,0,abc123def,2026-03-10 09:00:00\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Tool != ToolWxTCmdActivity {
		t.Errorf("tool = %q, want %q", res.Tool, ToolWxTCmdActivity)
	}
	if res.Count < 2 {
		t.Fatalf("expected at least 2 events (StartTime + EndTime), got %d", res.Count)
	}
	e := res.Events[0]
	if e.Source != "WINDOWSTIMELINE" {
		t.Errorf("source = %q, want WINDOWSTIMELINE", e.Source)
	}
	if e.SourceType != "Windows 10 Timeline" {
		t.Errorf("sourcetype = %q, want Windows 10 Timeline", e.SourceType)
	}
	if e.Desc != "Photos" {
		t.Errorf("desc = %q, want Photos", e.Desc)
	}
	types := make(map[string]bool)
	for _, ev := range res.Events {
		types[ev.Type] = true
	}
	for _, want := range []string{"StartTime", "EndTime"} {
		if !types[want] {
			t.Errorf("expected event type %q to be present", want)
		}
	}
	// Verify MACB values
	for _, ev := range res.Events {
		switch ev.Type {
		case "StartTime", "EndTime":
			if ev.MACB != "M..." {
				t.Errorf("MACB for %q = %q, want M...", ev.Type, ev.MACB)
			}
		case "LastModifiedTime", "LastModifiedOnClient", "OriginalLastModifiedOnClient":
			if ev.MACB != ".AB." {
				t.Errorf("MACB for %q = %q, want .AB.", ev.Type, ev.MACB)
			}
		}
	}
}

func TestWxTCmdActivityEmptyTimestampsSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "WxTCmd_Activity.csv")
	content := "ActivityTypeOrg,ActivityType,Executable,DisplayText,StartTime,EndTime," +
		"LastModifiedTime,LastModifiedOnClient,OriginalLastModifiedOnClient," +
		"DevicePlatform,TimeZone,Duration,IsLocalOnly,CreatedInCloud,ETag,ExpirationTime\n" +
		"Microsoft.Windows.Photos,InFocus,C:\\Windows\\Photos.exe,Photos," +
		"2026-01-10 09:00:00,,,,,Windows.Desktop,UTC,900,0,0,abc123def,2026-03-10 09:00:00\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	res, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}
	if res.Count != 1 {
		t.Errorf("expected exactly 1 event (only StartTime populated), got %d", res.Count)
	}
	if res.Events[0].Type != "StartTime" {
		t.Errorf("type = %q, want StartTime", res.Events[0].Type)
	}
}

func TestDetectWxTCmdPackageIDs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "WxTCmd_PackageIDs.csv")
	content := "PackageId,Platform,AdditionalInformation,Expires\n" +
		"com.microsoft.photos,Windows.Desktop,,2026-06-01 00:00:00\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	tool, err := DetectTool(path)
	if err != nil {
		t.Fatalf("DetectTool failed: %v", err)
	}
	if tool != ToolWxTCmdPackageIDs {
		t.Errorf("expected tool %q, got %q", ToolWxTCmdPackageIDs, tool)
	}
}

func TestWxTCmdPackageIDsInNoTimestampFormats(t *testing.T) {
	if _, ok := NoTimestampFormats[ToolWxTCmdPackageIDs]; !ok {
		t.Errorf("NoTimestampFormats missing %q", ToolWxTCmdPackageIDs)
	}
}

func TestImportFolderRecursiveWxTCmdPackageIDsSkipped(t *testing.T) {
	root := t.TempDir()
	content := "PackageId,Platform,AdditionalInformation,Expires\n" +
		"com.microsoft.photos,Windows.Desktop,,2026-06-01 00:00:00\n"
	if err := os.WriteFile(filepath.Join(root, "WxTCmd_PackageIDs_Output.csv"),
		[]byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if summary.TotalFilesProcessed != 0 {
		t.Errorf("TotalFilesProcessed = %d, want 0 (PackageIDs should not count)", summary.TotalFilesProcessed)
	}
	if store.insertedCount != 0 {
		t.Errorf("insertedCount = %d, want 0", store.insertedCount)
	}
	var found bool
	for _, sf := range summary.SkippedFiles {
		if sf.RelativePath == "WxTCmd_PackageIDs_Output.csv" &&
			strings.Contains(sf.Reason, "no timestamp columns") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected PackageIDs file in SkippedFiles with 'no timestamp columns'; got: %v",
			summary.SkippedFiles)
	}
}
