package dynamicparser

import (
	"os"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "dynamic_test_*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// --- Validation Tests ---

func TestValidateFile_DefaultFields(t *testing.T) {
	content := "datetime,timestamp_desc,source,source_long,message,parser,display_name,tag\n"
	path := writeTempFile(t, content)
	if err := ValidateFile(path); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateFile_MinimalFields(t *testing.T) {
	content := "datetime,message\n"
	path := writeTempFile(t, content)
	if err := ValidateFile(path); err != nil {
		t.Errorf("expected valid with minimal fields, got: %v", err)
	}
}

func TestValidateFile_UnrecognizedFields(t *testing.T) {
	content := "foo,bar,baz\n"
	path := writeTempFile(t, content)
	if err := ValidateFile(path); err == nil {
		t.Error("expected error for unrecognized fields")
	}
}

func TestValidateFile_EmptyFile(t *testing.T) {
	path := writeTempFile(t, "")
	if err := ValidateFile(path); err == nil {
		t.Error("expected error for empty file")
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	if err := ValidateFile("/nonexistent/file.csv"); err == nil {
		t.Error("expected error for missing file")
	}
}

// --- Read Tests ---

func TestReadEvents_DefaultPlasoFields(t *testing.T) {
	content := `datetime,timestamp_desc,source,source_long,message,parser,display_name,tag
2018-10-09T16:00:00+00:00,Content Modification Time,FILE,NTFS MFT,test file event,mft,TSK:/Users/admin/test.txt,malware
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Fatalf("count = %d, want 1", result.Count)
	}

	e := result.Events[0]
	if e.Datetime != "2018-10-09 16:00:00" {
		t.Errorf("datetime = %q, want 2018-10-09 16:00:00", e.Datetime)
	}
	if e.Type != "Content Modification Time" {
		t.Errorf("type = %q, want Content Modification Time", e.Type)
	}
	if e.Source != "FILE" {
		t.Errorf("source = %q, want FILE", e.Source)
	}
	if e.SourceType != "NTFS MFT" {
		t.Errorf("sourcetype = %q, want NTFS MFT", e.SourceType)
	}
	if e.Desc != "test file event" {
		t.Errorf("desc = %q, want test file event", e.Desc)
	}
	if e.Format != "mft" {
		t.Errorf("format = %q, want mft", e.Format)
	}
	if e.Filename != "TSK:/Users/admin/test.txt" {
		t.Errorf("filename = %q, want TSK:/Users/admin/test.txt", e.Filename)
	}
	if e.Tag != "malware" {
		t.Errorf("tag = %q, want malware", e.Tag)
	}
	if e.MACB != "M..." {
		t.Errorf("MACB = %q, want M...", e.MACB)
	}
}

func TestReadEvents_CustomFields(t *testing.T) {
	content := `datetime,timestamp_desc,source,message,hostname,username,event_identifier
2018-10-09T16:00:00+00:00,Creation Time,EVT,Security event logged,WORKSTATION,admin,4624
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Fatalf("count = %d, want 1", result.Count)
	}

	e := result.Events[0]
	if e.Host != "WORKSTATION" {
		t.Errorf("host = %q, want WORKSTATION", e.Host)
	}
	if e.User != "admin" {
		t.Errorf("user = %q, want admin", e.User)
	}
	if e.EventID != "4624" {
		t.Errorf("event_identifier = %q, want 4624", e.EventID)
	}
	if e.MACB != "...B" {
		t.Errorf("MACB = %q, want ...B", e.MACB)
	}
}

func TestReadEvents_MultipleEvents(t *testing.T) {
	content := `datetime,timestamp_desc,source,message
2018-10-09T16:00:00+00:00,Content Modification Time,FILE,event one
2018-10-10T12:00:00+00:00,Last Access Time,REG,event two
2018-10-11T08:00:00+00:00,Creation Time,EVT,event three
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 3 {
		t.Errorf("count = %d, want 3", result.Count)
	}
}

func TestReadEvents_UnmappedFieldsInExtra(t *testing.T) {
	content := `datetime,message,custom_field,another_field
2018-10-09T16:00:00+00:00,test event,custom_value,42
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := result.Events[0]
	if !strings.Contains(e.Extra, "custom_field: custom_value") {
		t.Errorf("extra = %q, expected to contain 'custom_field: custom_value'", e.Extra)
	}
	if !strings.Contains(e.Extra, "another_field: 42") {
		t.Errorf("extra = %q, expected to contain 'another_field: 42'", e.Extra)
	}
}

func TestReadEvents_DashValuesIgnoredInExtra(t *testing.T) {
	content := `datetime,message,custom_field
2018-10-09T16:00:00+00:00,test event,-
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Extra != "" {
		t.Errorf("extra = %q, want empty (dash should be ignored)", result.Events[0].Extra)
	}
}

func TestReadEvents_DefaultTimezone(t *testing.T) {
	content := `datetime,message
2018-10-09T16:00:00+00:00,event
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Timezone != "UTC" {
		t.Errorf("timezone = %q, want UTC", result.Events[0].Timezone)
	}
}

func TestReadEvents_ExplicitTimezone(t *testing.T) {
	content := `datetime,message,zone
2018-10-09T16:00:00+00:00,event,America/Chicago
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Timezone != "America/Chicago" {
		t.Errorf("timezone = %q, want America/Chicago", result.Events[0].Timezone)
	}
}

func TestReadEvents_DatetimeNormalization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2018-10-09T16:00:00+00:00", "2018-10-09 16:00:00"},
		{"2018-10-09T16:00:00Z", "2018-10-09 16:00:00"},
		{"2018-10-09 16:00:00", "2018-10-09 16:00:00"},
		{"2018-10-09T16:00:00.123456+00:00", "2018-10-09 16:00:00"},
		{"", ""},
		{"-", ""},
	}

	for _, tt := range tests {
		got := normalizeDatetime(tt.input)
		if got != tt.want {
			t.Errorf("normalizeDatetime(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestReadEvents_SourceAliases(t *testing.T) {
	// source_short maps to source, source_long maps to sourcetype
	content := `datetime,source_short,source_long,message
2018-10-09T16:00:00+00:00,FILE,NTFS MFT,test
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := result.Events[0]
	if e.Source != "FILE" {
		t.Errorf("source = %q, want FILE", e.Source)
	}
	if e.SourceType != "NTFS MFT" {
		t.Errorf("sourcetype = %q, want NTFS MFT", e.SourceType)
	}
}

func TestReadEvents_ProgressCallback(t *testing.T) {
	var lines []string
	lines = append(lines, "datetime,message")
	for i := 0; i < 20000; i++ {
		lines = append(lines, "2018-10-09T16:00:00+00:00,event")
	}
	content := strings.Join(lines, "\n")
	path := writeTempFile(t, content)

	var callbacks []int
	result, err := ReadEvents(path, func(count int) {
		callbacks = append(callbacks, count)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 20000 {
		t.Errorf("count = %d, want 20000", result.Count)
	}
	if len(callbacks) != 2 {
		t.Errorf("callbacks = %d, want 2", len(callbacks))
	}
}

func TestReadEvents_StoreFields(t *testing.T) {
	// From the sample Plaso dynamic output
	content := `datetime,timestamp_desc,source,source_long,message,parser,display_name,tag,store_number,store_index
2004-09-20T15:18:38+00:00,Expiration Time,WEBHIST,MSIE Cache File URL record,some URL visit,msiecf,TSK:/path/to/file,-,1,143624
`
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := result.Events[0]
	if e.Source != "WEBHIST" {
		t.Errorf("source = %q, want WEBHIST", e.Source)
	}
	if e.SourceType != "MSIE Cache File URL record" {
		t.Errorf("sourcetype = %q, want 'MSIE Cache File URL record'", e.SourceType)
	}
	// store_number and store_index are unmapped (numeric, would go to Extra)
	if !strings.Contains(e.Extra, "store_number: 1") {
		t.Errorf("extra = %q, expected to contain 'store_number: 1'", e.Extra)
	}
}

// --- MACB Mapping Tests ---

func TestMapTimestampDescToMACB(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"Content Modification Time", "M..."},
		{"Last Access Time", ".A.."},
		{"Metadata Change Time", "..C."},
		{"Creation Time", "...B"},
		{"Expiration Time", "...."},
		{"", "...."},
	}

	for _, tt := range tests {
		got := mapTimestampDescToMACB(tt.desc)
		if got != tt.want {
			t.Errorf("mapTimestampDescToMACB(%q) = %q, want %q", tt.desc, got, tt.want)
		}
	}
}
