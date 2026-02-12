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

func TestValidateFile_ValidPsort(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "timestamp_desc": "Last Written", "source_short": "FILE", "message": "test event", "parser": "mft"}
`
	path := writeTempFile(t, "valid.jsonl", content)
	if err := ValidateFile(path); err != nil {
		t.Errorf("expected valid file, got error: %v", err)
	}
}

func TestValidateFile_ValidRaw(t *testing.T) {
	content := `{"__container_type__": "event", "__type__": "AttributeContainer", "data_type": "fs:stat", "date_time": {"__class_name__": "Filetime", "__type__": "DateTimeValues", "timestamp": 132500000000000000}, "message": "test", "parser": "filestat", "timestamp_desc": "Metadata Modification Time"}
`
	path := writeTempFile(t, "raw.jsonl", content)
	if err := ValidateFile(path); err != nil {
		t.Errorf("expected valid raw file, got error: %v", err)
	}
}

func TestValidateFile_Empty(t *testing.T) {
	path := writeTempFile(t, "empty.jsonl", "")
	if err := ValidateFile(path); err == nil {
		t.Error("expected error for empty file, got nil")
	}
}

func TestValidateFile_NotJSON(t *testing.T) {
	path := writeTempFile(t, "notjson.jsonl", "this is not json\n")
	if err := ValidateFile(path); err == nil {
		t.Error("expected error for non-JSON file, got nil")
	}
}

func TestValidateFile_NoPlasoFields(t *testing.T) {
	content := `{"random_field": "value", "another": 123}
`
	path := writeTempFile(t, "noplaso.jsonl", content)
	if err := ValidateFile(path); err == nil {
		t.Error("expected error for non-Plaso JSON, got nil")
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	if err := ValidateFile("/nonexistent/path.jsonl"); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestValidateFile_CSVNotJSON(t *testing.T) {
	content := "date,time,timezone,MACB\n01/15/2025,10:30:00,UTC,MACB\n"
	path := writeTempFile(t, "actually_csv.jsonl", content)
	if err := ValidateFile(path); err == nil {
		t.Error("expected error for CSV content in JSONL file, got nil")
	}
}

// --- Raw Plaso Format Tests ---

func TestReadEvents_RawPlasoFsStat(t *testing.T) {
	content := `{"__container_type__": "event", "__type__": "AttributeContainer", "data_type": "fs:stat", "date_time": {"__class_name__": "Filetime", "__type__": "DateTimeValues", "timestamp": 132500000000000000}, "display_name": "NTFS:\\Windows\\System32\\test.dll", "filename": "\\Windows\\System32\\test.dll", "inode": "12345", "message": "NTFS:\\Windows\\System32\\test.dll Type: file", "parser": "filestat", "timestamp": -11644473599704022, "timestamp_desc": "Metadata Modification Time"}
`
	path := writeTempFile(t, "raw_fsstat.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("expected 1 event, got %d", result.Count)
	}
	e := result.Events[0]
	if e.Source != "FILE" {
		t.Errorf("source = %q, want %q", e.Source, "FILE")
	}
	if e.SourceType != "File stat" {
		t.Errorf("sourcetype = %q, want %q", e.SourceType, "File stat")
	}
	if e.Format != "filestat" {
		t.Errorf("format = %q, want %q", e.Format, "filestat")
	}
	if e.Filename != "\\Windows\\System32\\test.dll" {
		t.Errorf("filename = %q, want %q", e.Filename, "\\Windows\\System32\\test.dll")
	}
	if e.Type != "Metadata Modification Time" {
		t.Errorf("type = %q, want %q", e.Type, "Metadata Modification Time")
	}
	if e.MACB != "M.C." {
		t.Errorf("MACB = %q, want %q", e.MACB, "M.C.")
	}
}

func TestReadEvents_RawPlasoOlecf(t *testing.T) {
	content := `{"__container_type__": "event", "__type__": "AttributeContainer", "data_type": "olecf:summary_info", "date_time": {"__class_name__": "Filetime", "__type__": "DateTimeValues", "timestamp": 132500000000000000}, "message": "Title: Test Doc", "parser": "olecf/olecf_summary", "timestamp_desc": "Document Creation Time"}
`
	path := writeTempFile(t, "raw_olecf.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := result.Events[0]
	if e.Source != "OLECF" {
		t.Errorf("source = %q, want %q", e.Source, "OLECF")
	}
	if e.SourceType != "OLECF Summary Info" {
		t.Errorf("sourcetype = %q, want %q", e.SourceType, "OLECF Summary Info")
	}
	if e.MACB != "...B" {
		t.Errorf("MACB = %q, want %q", e.MACB, "...B")
	}
}

func TestReadEvents_RawPlasoPE_NotSet(t *testing.T) {
	content := `{"__container_type__": "event", "__type__": "AttributeContainer", "data_type": "pe_coff:file", "date_time": {"__class_name__": "NotSet", "__type__": "DateTimeValues", "timestamp": 0}, "message": "PE test", "parser": "pe", "timestamp_desc": "Not a time"}
`
	path := writeTempFile(t, "raw_pe.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := result.Events[0]
	if e.Source != "PE" {
		t.Errorf("source = %q, want %q", e.Source, "PE")
	}
	if e.SourceType != "PE File" {
		t.Errorf("sourcetype = %q, want %q", e.SourceType, "PE File")
	}
	if e.Datetime != "Not a time" {
		t.Errorf("datetime = %q, want %q", e.Datetime, "Not a time")
	}
}

func TestReadEvents_RawPlasoUnknownDataType(t *testing.T) {
	content := `{"__container_type__": "event", "__type__": "AttributeContainer", "data_type": "custom:parser:output", "date_time": {"__class_name__": "PosixTime", "__type__": "DateTimeValues", "timestamp": 1705312200}, "message": "custom event", "parser": "custom", "timestamp_desc": "Creation Time"}
`
	path := writeTempFile(t, "raw_unknown.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := result.Events[0]
	if e.Source != "CUSTOM" {
		t.Errorf("source = %q, want %q", e.Source, "CUSTOM")
	}
	if e.Datetime != "2024-01-15 09:50:00" {
		t.Errorf("datetime = %q, want %q", e.Datetime, "2024-01-15 09:50:00")
	}
}

func TestReadEvents_RawPlasoDisplayNameFallback(t *testing.T) {
	content := `{"__container_type__": "event", "__type__": "AttributeContainer", "data_type": "fs:stat", "date_time": {"__class_name__": "Filetime", "__type__": "DateTimeValues", "timestamp": 132500000000000000}, "display_name": "TSK:/Windows/System32/config/SAM", "message": "test", "parser": "filestat", "timestamp_desc": "Creation Time"}
`
	path := writeTempFile(t, "raw_displayname.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Filename != "TSK:/Windows/System32/config/SAM" {
		t.Errorf("filename = %q, want display_name fallback", result.Events[0].Filename)
	}
}

func TestReadEvents_RawPlasoSkipsPathspec(t *testing.T) {
	content := `{"__container_type__": "event", "__type__": "AttributeContainer", "data_type": "fs:stat", "date_time": {"__class_name__": "Filetime", "__type__": "DateTimeValues", "timestamp": 132500000000000000}, "message": "test", "parser": "filestat", "timestamp_desc": "Creation Time", "pathspec": {"__type__": "PathSpec", "location": "/test"}}
`
	path := writeTempFile(t, "raw_pathspec.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.Events[0].Extra, "pathspec") {
		t.Errorf("extra should not contain pathspec: %q", result.Events[0].Extra)
	}
}

func TestReadEvents_RawPlasoSkipsInternalFields(t *testing.T) {
	content := `{"__container_type__": "event", "__type__": "AttributeContainer", "data_type": "fs:stat", "date_time": {"__class_name__": "Filetime", "__type__": "DateTimeValues", "timestamp": 132500000000000000}, "message": "test", "parser": "filestat", "timestamp_desc": "Creation Time"}
`
	path := writeTempFile(t, "raw_internal.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	extra := result.Events[0].Extra
	if strings.Contains(extra, "__container_type__") {
		t.Errorf("extra should not contain __container_type__: %q", extra)
	}
	if strings.Contains(extra, "__type__") {
		t.Errorf("extra should not contain __type__: %q", extra)
	}
}

// --- Filetime Conversion Tests ---

func TestConvertFiletime_ValidDate(t *testing.T) {
	// 132500000000000000 ticks ~ 2020-11-17 UTC
	result := convertFiletime(132500000000000000)
	if result == "Not a time" || result == "" {
		t.Errorf("expected valid date, got %q", result)
	}
	if !strings.HasPrefix(result, "2020-") {
		t.Errorf("expected date starting with 2020-, got %q", result)
	}
}

func TestConvertFiletime_Zero(t *testing.T) {
	if result := convertFiletime(0); result != "Not a time" {
		t.Errorf("expected 'Not a time' for zero, got %q", result)
	}
}

func TestConvertFiletime_Negative(t *testing.T) {
	if result := convertFiletime(-1); result != "Not a time" {
		t.Errorf("expected 'Not a time' for negative, got %q", result)
	}
}

func TestConvertFiletime_TooSmall(t *testing.T) {
	// Very small tick value results in date near 1601, should still convert
	result := convertFiletime(2959780)
	if !strings.HasPrefix(result, "1601-") {
		t.Errorf("expected date near 1601, got %q", result)
	}
}

func TestConvertDateTimeObject_NotSet(t *testing.T) {
	dtObj := map[string]interface{}{
		"__class_name__": "NotSet",
		"__type__":       "DateTimeValues",
		"timestamp":      float64(0),
	}
	if result := convertDateTimeObject(dtObj); result != "Not a time" {
		t.Errorf("expected 'Not a time' for NotSet, got %q", result)
	}
}

func TestConvertDateTimeObject_PosixTime(t *testing.T) {
	dtObj := map[string]interface{}{
		"__class_name__": "PosixTime",
		"__type__":       "DateTimeValues",
		"timestamp":      float64(1705312200),
	}
	if result := convertDateTimeObject(dtObj); result != "2024-01-15 09:50:00" {
		t.Errorf("datetime = %q, want %q", result, "2024-01-15 09:50:00")
	}
}

func TestConvertDateTimeObject_PosixMicroseconds(t *testing.T) {
	dtObj := map[string]interface{}{
		"__class_name__": "PosixTimeInMicroseconds",
		"__type__":       "DateTimeValues",
		"timestamp":      float64(1705312200000000),
	}
	if result := convertDateTimeObject(dtObj); result != "2024-01-15 09:50:00" {
		t.Errorf("datetime = %q, want %q", result, "2024-01-15 09:50:00")
	}
}

func TestConvertDateTimeObject_NotAMap(t *testing.T) {
	if result := convertDateTimeObject("not a map"); result != "" {
		t.Errorf("expected empty for non-map, got %q", result)
	}
}

// --- Psort Format Tests ---

func TestReadEvents_PsortSingleEvent(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "timestamp_desc": "Content Modification Time", "source_short": "FILE", "source_long": "NTFS MFT", "message": "test file event", "parser": "mft", "filename": "/Users/admin/test.txt", "hostname": "WORKSTATION1", "username": "admin"}
`
	path := writeTempFile(t, "psort_single.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := result.Events[0]
	if e.Source != "FILE" {
		t.Errorf("source = %q, want %q", e.Source, "FILE")
	}
	if e.SourceType != "NTFS MFT" {
		t.Errorf("sourcetype = %q, want %q", e.SourceType, "NTFS MFT")
	}
	if e.Host != "WORKSTATION1" {
		t.Errorf("host = %q, want %q", e.Host, "WORKSTATION1")
	}
	if e.User != "admin" {
		t.Errorf("user = %q, want %q", e.User, "admin")
	}
}

func TestReadEvents_PsortMultipleEvents(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "timestamp_desc": "Last Written", "source_short": "FILE", "message": "one", "parser": "mft"}
{"timestamp": 1705398600000000, "datetime": "2024-01-16T10:30:00+00:00", "timestamp_desc": "Last Accessed", "source_short": "FILE", "message": "two", "parser": "mft"}
{"timestamp": 1705485000000000, "datetime": "2024-01-17T10:30:00+00:00", "timestamp_desc": "Creation Time", "source_short": "REG", "message": "three", "parser": "winreg"}
`
	path := writeTempFile(t, "psort_multi.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 3 {
		t.Errorf("expected 3, got %d", result.Count)
	}
}

func TestReadEvents_SkipsInvalidLines(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source_short": "FILE", "message": "good", "parser": "mft"}
this is not json at all
{"timestamp": 1705398600000000, "datetime": "2024-01-16T10:30:00+00:00", "source_short": "FILE", "message": "also good", "parser": "mft"}
`
	path := writeTempFile(t, "mixed.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 2 {
		t.Errorf("count = %d, want 2", result.Count)
	}
	if result.Excluded != 1 {
		t.Errorf("excluded = %d, want 1", result.Excluded)
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
		t.Errorf("count = %d, want 2", result.Count)
	}
}

func TestReadEvents_EmptyFile(t *testing.T) {
	path := writeTempFile(t, "empty.jsonl", "")
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("count = %d, want 0", result.Count)
	}
}

func TestReadEvents_PsortTimestampFallback(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "source_short": "FILE", "message": "ts only", "parser": "mft"}
`
	path := writeTempFile(t, "tsonly.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Datetime != "2024-01-15 09:50:00" {
		t.Errorf("datetime = %q, want %q", result.Events[0].Datetime, "2024-01-15 09:50:00")
	}
}

func TestReadEvents_PsortStringTimestamp(t *testing.T) {
	content := `{"timestamp": "2024-01-15T10:30:00Z", "source_short": "FILE", "message": "string ts", "parser": "mft"}
`
	path := writeTempFile(t, "stringts.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Datetime != "2024-01-15 10:30:00" {
		t.Errorf("datetime = %q, want %q", result.Events[0].Datetime, "2024-01-15 10:30:00")
	}
}

// --- MACB Mapping Tests ---

func TestMACB_Mapping(t *testing.T) {
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
		{"Document Creation Time", "...B"},
		{"Metadata Modification Time", "M.C."},
		{"Unknown Type", ""},
		{"Not a time", ""},
	}
	for _, tc := range tests {
		if got := mapTimestampDescToMACB(tc.desc); got != tc.want {
			t.Errorf("mapTimestampDescToMACB(%q) = %q, want %q", tc.desc, got, tc.want)
		}
	}
}

// --- data_type to Source Mapping Tests ---

func TestMapDataTypeToSource(t *testing.T) {
	tests := []struct {
		dataType  string
		wantShort string
		wantLong  string
	}{
		{"fs:stat", "FILE", "File stat"},
		{"fs:stat:ntfs", "FILE", "NTFS File stat"},
		{"windows:evtx:record", "EVT", "WinEVTX"},
		{"chrome:history:page_visited", "WEBHIST", "Chrome History"},
		{"olecf:summary_info", "OLECF", "OLECF Summary Info"},
		{"pe_coff:file", "PE", "PE File"},
		{"windows:prefetch:execution", "PREFETCH", "Windows Prefetch"},
		{"windows:lnk:link", "LNK", "Windows Shortcut"},
	}
	for _, tc := range tests {
		short, long := mapDataTypeToSource(tc.dataType)
		if short != tc.wantShort {
			t.Errorf("mapDataTypeToSource(%q) short = %q, want %q", tc.dataType, short, tc.wantShort)
		}
		if long != tc.wantLong {
			t.Errorf("mapDataTypeToSource(%q) long = %q, want %q", tc.dataType, long, tc.wantLong)
		}
	}
}

func TestMapDataTypeToSource_Fallback(t *testing.T) {
	short, _ := mapDataTypeToSource("unknown:parser:type")
	if short != "UNKNOWN" {
		t.Errorf("fallback source = %q, want %q", short, "UNKNOWN")
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
		t.Errorf("timezone = %q, want UTC", result.Events[0].Timezone)
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
		t.Errorf("timezone = %q, want America/New_York", result.Events[0].Timezone)
	}
}

func TestReadEvents_PsortSourceFallback(t *testing.T) {
	content := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "source": "LOG", "message": "event", "parser": "syslog"}
`
	path := writeTempFile(t, "sourcefallback.jsonl", content)
	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Events[0].Source != "LOG" {
		t.Errorf("source = %q, want LOG", result.Events[0].Source)
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
	if !strings.Contains(extra, "custom_field") {
		t.Errorf("extra should contain custom_field: %q", extra)
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
		t.Errorf("tag = %q, want malware", result.Events[0].Tag)
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
		t.Errorf("tag = %q, want 'malware, suspicious'", result.Events[0].Tag)
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
		t.Errorf("event_identifier = %q, want 4624", e.EventID)
	}
	if e.EventType != "0" {
		t.Errorf("event_type = %q, want 0", e.EventType)
	}
	if e.RecordNumber != "12345" {
		t.Errorf("record_number = %q, want 12345", e.RecordNumber)
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
		t.Errorf("count = %d, want 25000", result.Count)
	}
	if callCount != 2 {
		t.Errorf("progress callbacks = %d, want 2", callCount)
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
		if got := interfaceToString(tc.input); got != tc.want {
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
		if got := interfaceToInt64(tc.input); got != tc.want {
			t.Errorf("interfaceToInt64(%v) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
