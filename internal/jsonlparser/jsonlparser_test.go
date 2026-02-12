package jsonlparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}

// --- Validation Tests ---

func TestValidateFile_Valid(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "timestamp_desc": "Last Written", "source_short": "FILE", "source_long": "NTFS MFT", "message": "test event", "parser": "mft"}
`
	path := writeTempFile(t, "valid.jsonl", content)
	err := ValidateFile(path)
	if err != nil {
		t.Errorf("expected valid file, got error: %v", err)
	}
}

func TestValidateFile_Empty(t *testing.T) {
	path := writeTempFile(t, "empty.jsonl", "")
	err := ValidateFile(path)
	if err == nil {
		t.Error("expected error for empty file, got nil")
	}
}

func TestValidateFile_NotJSON(t *testing.T) {
	path := writeTempFile(t, "notjson.jsonl", "this is not json\n")
	err := ValidateFile(path)
	if err == nil {
		t.Error("expected error for non-JSON file, got nil")
	}
}

func TestValidateFile_NoTimestamp(t *testing.T) {
	content := `{"message": "no timestamp here", "source_short": "FILE"}
`
	path := writeTempFile(t, "notime.jsonl", content)
	err := ValidateFile(path)
	if err == nil {
		t.Error("expected error for missing timestamp, got nil")
	}
}

func TestValidateFile_NoMessageOrSource(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00"}
`
	path := writeTempFile(t, "nomsg.jsonl", content)
	err := ValidateFile(path)
	if err == nil {
		t.Error("expected error for missing message/source_short, got nil")
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	err := ValidateFile("/nonexistent/path.jsonl")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestValidateFile_CSVNotJSON(t *testing.T) {
	content := "date,time,timezone,MACB\n01/15/2025,10:30:00,UTC,MACB\n"
	path := writeTempFile(t, "actually_csv.jsonl", content)
	err := ValidateFile(path)
	if err == nil {
		t.Error("expected error for CSV content in JSONL file, got nil")
	}
}

// --- ReadEvents Tests ---

func TestReadEvents_SingleEvent(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "timestamp_desc": "Content Modification Time", "source_short": "FILE", "source_long": "NTFS MFT", "message": "test file event", "parser": "mft", "filename": "/Users/admin/test.txt", "hostname": "WORKSTATION1", "username": "admin"}
`
	path := writeTempFile(t, "single.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("expected 1 event, got %d", result.Count)
	}

	e := result.Events[0]
	if e.Datetime != "2024-01-15T10:30:00+00:00" {
		t.Errorf("datetime = %q, want %q", e.Datetime, "2024-01-15T10:30:00+00:00")
	}
	if e.Source != "FILE" {
		t.Errorf("source = %q, want %q", e.Source, "FILE")
	}
	if e.SourceType != "NTFS MFT" {
		t.Errorf("sourcetype = %q, want %q", e.SourceType, "NTFS MFT")
	}
	if e.Type != "Content Modification Time" {
		t.Errorf("type = %q, want %q", e.Type, "Content Modification Time")
	}
	if e.Desc != "test file event" {
		t.Errorf("desc = %q, want %q", e.Desc, "test file event")
	}
	if e.Format != "mft" {
		t.Errorf("format = %q, want %q", e.Format, "mft")
	}
	if e.Filename != "/Users/admin/test.txt" {
		t.Errorf("filename = %q, want %q", e.Filename, "/Users/admin/test.txt")
	}
	if e.Host != "WORKSTATION1" {
		t.Errorf("host = %q, want %q", e.Host, "WORKSTATION1")
	}
	if e.User != "admin" {
		t.Errorf("user = %q, want %q", e.User, "admin")
	}
}

func TestReadEvents_MultipleEvents(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "timestamp_desc": "Last Written", "source_short": "FILE", "message": "event one", "parser": "mft"}
{"timestamp": 1705398600000000, "datetime": "2024-01-16T10:30:00+00:00", "timestamp_desc": "Last Accessed", "source_short": "FILE", "message": "event two", "parser": "mft"}
{"timestamp": 1705485000000000, "datetime": "2024-01-17T10:30:00+00:00", "timestamp_desc": "Creation Time", "source_short": "REG", "message": "event three", "parser": "winreg"}
`
	path := writeTempFile(t, "multi.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 3 {
		t.Errorf("expected 3 events, got %d", result.Count)
	}
}

func TestReadEvents_SkipsInvalidLines(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "good event", "parser": "mft"}
this is not json at all
{"timestamp": 1705398600000000, "datetime": "2024-01-16T10:30:00+00:00", "source_short": "FILE", "message": "another good event", "parser": "mft"}
`
	path := writeTempFile(t, "mixed.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 2 {
		t.Errorf("expected 2 events, got %d", result.Count)
	}
	if result.Excluded != 1 {
		t.Errorf("expected 1 excluded, got %d", result.Excluded)
	}
}

func TestReadEvents_SkipsBlankLines(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft"}

{"timestamp": 1705398600000000, "datetime": "2024-01-16T10:30:00+00:00", "source_short": "FILE", "message": "event2", "parser": "mft"}
`
	path := writeTempFile(t, "blanks.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 2 {
		t.Errorf("expected 2 events, got %d", result.Count)
	}
}

func TestReadEvents_EmptyFile(t *testing.T) {
	path := writeTempFile(t, "empty.jsonl", "")
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("expected 0 events, got %d", result.Count)
	}
}

// --- Timestamp Conversion Tests ---

func TestReadEvents_TimestampFallback(t *testing.T) {
	// No datetime field, should convert from microsecond timestamp
	// 1705312200000000 microseconds = 2024-01-15 09:50:00 UTC
	content := `{"timestamp": 1705312200000000, "source_short": "FILE", "message": "ts only", "parser": "mft"}
`
	path := writeTempFile(t, "tsonly.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("expected 1 event, got %d", result.Count)
	}
	if result.Events[0].Datetime != "2024-01-15 09:50:00" {
		t.Errorf("datetime = %q, want %q", result.Events[0].Datetime, "2024-01-15 09:50:00")
	}
}

func TestReadEvents_StringTimestamp(t *testing.T) {
	content := `{"timestamp": "2024-01-15T10:30:00Z", "source_short": "FILE", "message": "string ts", "parser": "mft"}
`
	path := writeTempFile(t, "stringts.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("expected 1 event, got %d", result.Count)
	}
	// Should use string timestamp as-is
	if result.Events[0].Datetime != "2024-01-15T10:30:00Z" {
		t.Errorf("datetime = %q, want %q", result.Events[0].Datetime, "2024-01-15T10:30:00Z")
	}
}

// --- MACB Mapping Tests ---

func TestMACB_Modification(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"Content Modification Time", "M..."},
		{"Last Written", "M..."},
		{"Last Accessed", ".A.."},
		{"Last Access Time", ".A.."},
		{"Metadata Change", "..C."},
		{"Entry Modification", "M.C."},
		{"Creation Time", "...B"},
		{"File Created", "...B"},
		{"Content Modification Time", "M..."},
		{"Unknown Type", ""},
	}

	for _, tc := range tests {
		got := mapTimestampDescToMACB(tc.desc)
		if got != tc.want {
			t.Errorf("mapTimestampDescToMACB(%q) = %q, want %q", tc.desc, got, tc.want)
		}
	}
}

// --- Field Mapping Tests ---

func TestReadEvents_DefaultTimezone(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft"}
`
	path := writeTempFile(t, "notz.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Timezone != "UTC" {
		t.Errorf("timezone = %q, want %q", result.Events[0].Timezone, "UTC")
	}
}

func TestReadEvents_ExplicitTimezone(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "zone": "America/New_York", "source_short": "FILE", "message": "event", "parser": "mft"}
`
	path := writeTempFile(t, "withtz.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Timezone != "America/New_York" {
		t.Errorf("timezone = %q, want %q", result.Events[0].Timezone, "America/New_York")
	}
}

func TestReadEvents_DisplayNameFallback(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft", "display_name": "TSK:/Windows/System32/config/SAM"}
`
	path := writeTempFile(t, "displayname.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Filename != "TSK:/Windows/System32/config/SAM" {
		t.Errorf("filename = %q, want display_name fallback", result.Events[0].Filename)
	}
}

func TestReadEvents_FilenameOverDisplayName(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft", "filename": "/actual/path.txt", "display_name": "TSK:/display/path.txt"}
`
	path := writeTempFile(t, "filepriority.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Filename != "/actual/path.txt" {
		t.Errorf("filename = %q, want %q", result.Events[0].Filename, "/actual/path.txt")
	}
}

func TestReadEvents_SourceFallback(t *testing.T) {
	// When source_short is missing, should fall back to "source"
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source": "LOG", "message": "event", "parser": "syslog"}
`
	path := writeTempFile(t, "sourcefallback.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Source != "LOG" {
		t.Errorf("source = %q, want %q", result.Events[0].Source, "LOG")
	}
}

// --- Extra Fields Tests ---

func TestReadEvents_ExtraFields(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft", "custom_field": "custom_value", "another_field": 42}
`
	path := writeTempFile(t, "extras.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	extra := result.Events[0].Extra
	if !strings.Contains(extra, "custom_field") || !strings.Contains(extra, "custom_value") {
		t.Errorf("extra should contain custom_field: %q", extra)
	}
	if !strings.Contains(extra, "another_field") || !strings.Contains(extra, "42") {
		t.Errorf("extra should contain another_field: %q", extra)
	}
}

func TestReadEvents_NoExtras(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft"}
`
	path := writeTempFile(t, "noextras.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Extra != "" {
		t.Errorf("expected empty extra, got %q", result.Events[0].Extra)
	}
}

// --- Tag Tests ---

func TestReadEvents_TagString(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft", "tag": "malware"}
`
	path := writeTempFile(t, "tagstr.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Tag != "malware" {
		t.Errorf("tag = %q, want %q", result.Events[0].Tag, "malware")
	}
}

func TestReadEvents_TagList(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft", "tag_list": ["malware", "suspicious"]}
`
	path := writeTempFile(t, "taglist.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Tag != "malware, suspicious" {
		t.Errorf("tag = %q, want %q", result.Events[0].Tag, "malware, suspicious")
	}
}

// --- Numeric Field Tests ---

func TestReadEvents_NumericEventIdentifier(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "EVT", "message": "event log", "parser": "winevtx", "event_identifier": 4624, "event_type": 0, "record_number": 12345}
`
	path := writeTempFile(t, "numeric.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := result.Events[0]
	if e.EventID != "4624" {
		t.Errorf("event_identifier = %q, want %q", e.EventID, "4624")
	}
	if e.EventType != "0" {
		t.Errorf("event_type = %q, want %q", e.EventType, "0")
	}
	if e.RecordNumber != "12345" {
		t.Errorf("record_number = %q, want %q", e.RecordNumber, "12345")
	}
}

// --- Progress Callback Tests ---

func TestReadEvents_ProgressCallback(t *testing.T) {
	var lines []string
	for i := 0; i < 25000; i++ {
		lines = append(lines, `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft"}`)
	}
	content := strings.Join(lines, "\n") + "\n"
	path := writeTempFile(t, "progress.jsonl", content)

	callCount := 0
	result, err := ReadEvents(path, func(count int) {
		callCount++
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 25000 {
		t.Errorf("expected 25000 events, got %d", result.Count)
	}
	// Should be called at 10000 and 20000
	if callCount != 2 {
		t.Errorf("expected 2 progress callbacks, got %d", callCount)
	}
}

// --- Helper Function Tests ---

func TestInterfaceToString(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{nil, ""},
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
	}
	for _, tc := range tests {
		got := interfaceToString(tc.input)
		if got != tc.want {
			t.Errorf("interfaceToString(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestInterfaceToInt64(t *testing.T) {
	tests := []struct {
		input interface{}
		want  int64
	}{
		{nil, 0},
		{float64(42), 42},
		{"100", 100},
		{"notanumber", 0},
	}
	for _, tc := range tests {
		got := interfaceToInt64(tc.input)
		if got != tc.want {
			t.Errorf("interfaceToInt64(%v) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// --- Internal Type Filtering Tests ---

func TestReadEvents_SkipsPlasoInternalFields(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "event", "parser": "mft", "__container_type__": "event", "__type__": "AttributeContainer"}
`
	path := writeTempFile(t, "internal.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// __container_type__ and __type__ should NOT appear in Extra
	if strings.Contains(result.Events[0].Extra, "__container_type__") {
		t.Errorf("extra should not contain __container_type__: %q", result.Events[0].Extra)
	}
	if strings.Contains(result.Events[0].Extra, "__type__") {
		t.Errorf("extra should not contain __type__: %q", result.Events[0].Extra)
	}
}
