package csvparser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

func writeTempCSV(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing temp CSV: %v", err)
	}
	return path
}

// Minimal valid L2T CSV content for testing.
const validL2TCSV = `date,time,timezone,MACB,source,sourcetype,type,user,host,short,desc,version,filename,inode,notes,format,extra
01/15/2025,10:30:00,UTC,MACB,FILE,OS:NTFS:MFT,Last Written,admin,WORKSTATION1,short desc,full description,2,/Users/admin/test.txt,12345,,mft,extra info
02/20/2025,14:00:00,UTC,..C.,REG,Registry Key,Content Modification,SYSTEM,SERVER01,reg short,Registry key modified,2,HKLM\Software\Test,0,,winreg,
`

func TestValidateHeader(t *testing.T) {
	path := writeTempCSV(t, "valid.csv", validL2TCSV)
	err := ValidateHeader(path)
	if err != nil {
		t.Errorf("expected valid header, got error: %v", err)
	}
}

func TestValidateHeaderBadHeader(t *testing.T) {
	content := "wrong,header,format\n1,2,3\n"
	path := writeTempCSV(t, "bad.csv", content)
	err := ValidateHeader(path)
	if err == nil {
		t.Error("expected error for bad header, got nil")
	}
}

func TestValidateHeaderTooShort(t *testing.T) {
	content := "date,time,timezone\n1,2,3\n"
	path := writeTempCSV(t, "short.csv", content)
	err := ValidateHeader(path)
	if err == nil {
		t.Error("expected error for short header, got nil")
	}
}

func TestValidateHeaderMissingFile(t *testing.T) {
	err := ValidateHeader("/nonexistent/path.csv")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestReadEvents(t *testing.T) {
	path := writeTempCSV(t, "events.csv", validL2TCSV)

	result, err := ReadEvents(path, "", "", 0, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("expected 2 events, got %d", result.Count)
	}

	e := result.Events[0]
	if e.Datetime != "2025-01-15 10:30:00" {
		t.Errorf("expected datetime '2025-01-15 10:30:00', got '%s'", e.Datetime)
	}
	if e.Timezone != "UTC" {
		t.Errorf("expected timezone 'UTC', got '%s'", e.Timezone)
	}
	if e.MACB != "MACB" {
		t.Errorf("expected MACB 'MACB', got '%s'", e.MACB)
	}
	if e.Source != "FILE" {
		t.Errorf("expected source 'FILE', got '%s'", e.Source)
	}
	if e.SourceType != "OS:NTFS:MFT" {
		t.Errorf("expected sourcetype 'OS:NTFS:MFT', got '%s'", e.SourceType)
	}
	if e.Host != "WORKSTATION1" {
		t.Errorf("expected host 'WORKSTATION1', got '%s'", e.Host)
	}
	if e.Filename != "/Users/admin/test.txt" {
		t.Errorf("expected filename '/Users/admin/test.txt', got '%s'", e.Filename)
	}
	if e.Offset != -1 {
		t.Errorf("expected offset -1, got %d", e.Offset)
	}
	if e.Tag != "" {
		t.Errorf("expected empty tag, got '%s'", e.Tag)
	}
}

func TestReadEventsSecondRow(t *testing.T) {
	path := writeTempCSV(t, "events.csv", validL2TCSV)

	result, err := ReadEvents(path, "", "", 0, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	e := result.Events[1]
	if e.Source != "REG" {
		t.Errorf("expected source 'REG', got '%s'", e.Source)
	}
	if e.Host != "SERVER01" {
		t.Errorf("expected host 'SERVER01', got '%s'", e.Host)
	}
	if e.Datetime != "2025-02-20 14:00:00" {
		t.Errorf("expected datetime '2025-02-20 14:00:00', got '%s'", e.Datetime)
	}
}

func TestReadEventsWithLimit(t *testing.T) {
	path := writeTempCSV(t, "events.csv", validL2TCSV)

	result, err := ReadEvents(path, "", "", 1, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("expected 1 event with limit, got %d", result.Count)
	}
}

func TestReadEventsWithDateFilter(t *testing.T) {
	path := writeTempCSV(t, "events.csv", validL2TCSV)

	// Only events after Jan 20 and before Mar 1
	result, err := ReadEvents(path, "2025-01-20", "2025-03-01", 0, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("expected 1 event in date range, got %d", result.Count)
	}
	if result.Excluded != 1 {
		t.Errorf("expected 1 excluded event, got %d", result.Excluded)
	}
	if result.Events[0].Source != "REG" {
		t.Errorf("expected REG event, got '%s'", result.Events[0].Source)
	}
}

func TestReadEventsProgress(t *testing.T) {
	// Build a CSV with 25,000 rows to trigger progress callback
	var content string
	content = "date,time,timezone,MACB,source,sourcetype,type,user,host,short,desc,version,filename,inode,notes,format,extra\n"
	for i := 0; i < 25000; i++ {
		content += "01/15/2025,10:30:00,UTC,MACB,FILE,NTFS,Last Written,admin,WS1,short,desc,2,file.txt,0,,mft,\n"
	}

	path := writeTempCSV(t, "big.csv", content)

	var calls int
	result, err := ReadEvents(path, "", "", 0, func(count int) {
		calls++
	})
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	if result.Count != 25000 {
		t.Errorf("expected 25000 events, got %d", result.Count)
	}
	if calls != 2 { // called at 10000 and 20000
		t.Errorf("expected 2 progress calls, got %d", calls)
	}
}

func TestReadEventsNullBytes(t *testing.T) {
	content := "date,time,timezone,MACB,source,sourcetype,type,user,host,short,desc,version,filename,inode,notes,format,extra\n" +
		"01/15/2025,10:30:00,UTC,MACB,FILE,NTFS,Last Written,adm\x00in,WS1,short,de\x00sc,2,file.txt,0,,mft,\n"

	path := writeTempCSV(t, "nulls.csv", content)

	result, err := ReadEvents(path, "", "", 0, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	if result.Count != 1 {
		t.Fatalf("expected 1 event, got %d", result.Count)
	}

	// Null bytes should be stripped
	if result.Events[0].User != "admin" {
		t.Errorf("expected 'admin' (null stripped), got '%s'", result.Events[0].User)
	}
}

func TestReadEventsInvalidCSV(t *testing.T) {
	content := "not,a,valid,header\n"
	path := writeTempCSV(t, "invalid.csv", content)

	_, err := ReadEvents(path, "", "", 0, nil)
	if err == nil {
		t.Error("expected error for invalid CSV, got nil")
	}
}

func TestReadEventsShortRow(t *testing.T) {
	// Row with fewer columns than expected should still work (safeIndex)
	content := "date,time,timezone,MACB,source,sourcetype,type,user,host,short,desc,version,filename,inode,notes,format,extra\n" +
		"01/15/2025,10:30:00,UTC,MACB,FILE\n"

	path := writeTempCSV(t, "short_row.csv", content)

	result, err := ReadEvents(path, "", "", 0, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	if result.Count != 1 {
		t.Fatalf("expected 1 event, got %d", result.Count)
	}
	if result.Events[0].Source != "FILE" {
		t.Errorf("expected source 'FILE', got '%s'", result.Events[0].Source)
	}
	// Missing fields should be empty
	if result.Events[0].Host != "" {
		t.Errorf("expected empty host, got '%s'", result.Events[0].Host)
	}
}

func TestWriteEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.csv")

	events := []*model.Event{
		{
			Datetime: "2025-01-15 10:30:00", Timezone: "UTC", MACB: "MACB",
			Source: "FILE", SourceType: "NTFS", Type: "Last Written",
			User: "admin", Host: "WS1", Desc: "test file",
			Filename: "/test.txt", Inode: "123", Notes: "",
			Format: "mft", Extra: "", ReportNotes: "", InReport: "",
			Tag: "malware", Color: "RED", Offset: 0,
			StoreNumber: -1, StoreIndex: -1, VSSStoreNumber: -1,
		},
	}

	err := WriteEvents(path, events)
	if err != nil {
		t.Fatalf("WriteEvents failed: %v", err)
	}

	// Read it back and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}

	content := string(data)
	if !contains(content, "datetime,timezone,MACB") {
		t.Error("expected export header")
	}
	if !contains(content, "2025-01-15 10:30:00") {
		t.Error("expected datetime in export")
	}
	if !contains(content, "malware") {
		t.Error("expected tag in export")
	}
	if !contains(content, "RED") {
		t.Error("expected color in export")
	}
}

func TestRoundTrip(t *testing.T) {
	// Read L2T CSV, write as export, verify data survives
	srcPath := writeTempCSV(t, "source.csv", validL2TCSV)
	result, err := ReadEvents(srcPath, "", "", 0, nil)
	if err != nil {
		t.Fatalf("ReadEvents failed: %v", err)
	}

	// Tag one event
	result.Events[0].Tag = "interesting"
	result.Events[0].Color = "GREEN"

	exportPath := filepath.Join(t.TempDir(), "export.csv")
	err = WriteEvents(exportPath, result.Events)
	if err != nil {
		t.Fatalf("WriteEvents failed: %v", err)
	}

	// Verify the file exists and has content
	info, err := os.Stat(exportPath)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if info.Size() == 0 {
		t.Error("export file is empty")
	}
}

func TestReadColorCoding(t *testing.T) {
	content := "type,colorcode\nFILE,RED\nREG,BLUE\nEVT,\n"
	path := writeTempCSV(t, "colors.csv", content)

	cc, err := ReadColorCoding(path)
	if err != nil {
		t.Fatalf("ReadColorCoding failed: %v", err)
	}

	if cc.Field != "type" {
		t.Errorf("expected field 'type', got '%s'", cc.Field)
	}
	if cc.Mapping["FILE"] != "RED" {
		t.Errorf("expected FILE=RED, got '%s'", cc.Mapping["FILE"])
	}
	if cc.Mapping["REG"] != "BLUE" {
		t.Errorf("expected REG=BLUE, got '%s'", cc.Mapping["REG"])
	}
	if cc.Mapping["EVT"] != "WHITE" {
		t.Errorf("expected EVT=WHITE (default), got '%s'", cc.Mapping["EVT"])
	}
}

func TestReadColorCodingHost(t *testing.T) {
	content := "host,colorcode\nWS1,GREEN\nSERVER01,YELLOW\n"
	path := writeTempCSV(t, "host_colors.csv", content)

	cc, err := ReadColorCoding(path)
	if err != nil {
		t.Fatalf("ReadColorCoding failed: %v", err)
	}

	if cc.Field != "host" {
		t.Errorf("expected field 'host', got '%s'", cc.Field)
	}
}

func TestReadColorCodingBadHeader(t *testing.T) {
	content := "wrong,header\nFILE,RED\n"
	path := writeTempCSV(t, "bad_colors.csv", content)

	_, err := ReadColorCoding(path)
	if err == nil {
		t.Error("expected error for bad header, got nil")
	}
}

func TestWriteColorCoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "colors_out.csv")

	mapping := map[string]string{
		"FILE": "RED",
		"REG":  "BLUE",
	}

	err := WriteColorCoding(path, "type", mapping)
	if err != nil {
		t.Fatalf("WriteColorCoding failed: %v", err)
	}

	// Read it back
	cc, err := ReadColorCoding(path)
	if err != nil {
		t.Fatalf("ReadColorCoding failed: %v", err)
	}

	if cc.Mapping["FILE"] != "RED" {
		t.Errorf("expected FILE=RED, got '%s'", cc.Mapping["FILE"])
	}
}

func TestReadSavedQueries(t *testing.T) {
	content := "Name,SQL,Description,EID,OS,IP\n" +
		"Find malware,tag LIKE '%malware%',Finds tagged events,,,\n" +
		"Recent events,datetime > '2025-01-01',Last year,,Windows,\n"

	path := writeTempCSV(t, "queries.csv", content)

	queries, err := ReadSavedQueries(path)
	if err != nil {
		t.Fatalf("ReadSavedQueries failed: %v", err)
	}

	if len(queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(queries))
	}

	if queries[0].Name != "Find malware" {
		t.Errorf("expected name 'Find malware', got '%s'", queries[0].Name)
	}
	if queries[0].SQL != "tag LIKE '%malware%'" {
		t.Errorf("expected SQL clause, got '%s'", queries[0].SQL)
	}
	if queries[1].OS != "Windows" {
		t.Errorf("expected OS 'Windows', got '%s'", queries[1].OS)
	}
}

func TestReadSavedQueriesBadHeader(t *testing.T) {
	content := "wrong,format\ntest,test\n"
	path := writeTempCSV(t, "bad_queries.csv", content)

	_, err := ReadSavedQueries(path)
	if err == nil {
		t.Error("expected error for bad header, got nil")
	}
}

func TestWriteSavedQueries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queries_out.csv")

	queries := []SavedQueryEntry{
		{Name: "Test query", SQL: "source = 'FILE'", Description: "test"},
	}

	err := WriteSavedQueries(path, queries)
	if err != nil {
		t.Fatalf("WriteSavedQueries failed: %v", err)
	}

	// Read back
	result, err := ReadSavedQueries(path)
	if err != nil {
		t.Fatalf("ReadSavedQueries failed: %v", err)
	}

	if len(result) != 1 || result[0].Name != "Test query" {
		t.Errorf("round trip failed: got %v", result)
	}
}

// --- Date/Time reformatting tests ---

func TestReformatDate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"01/15/2025", "2025-01-15"},
		{"12/31/2024", "2024-12-31"},
		{"2025-01-15", "2025-01-15"}, // already correct format
		{"garbage", "0000-00-00"},
		{"", "0000-00-00"},
	}

	for _, tc := range tests {
		got := reformatDate(tc.input)
		if got != tc.expected {
			t.Errorf("reformatDate(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestReformatTime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"10:30:00", "10:30:00"},
		{"23:59:59", "23:59:59"},
		{"garbage", "00:00:00"},
		{"", "00:00:00"},
	}

	for _, tc := range tests {
		got := reformatTime(tc.input)
		if got != tc.expected {
			t.Errorf("reformatTime(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
