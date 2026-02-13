package tlnparser

import (
	"os"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "tln_test_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// --- Validation Tests ---

func TestValidateFile_TLNHeader(t *testing.T) {
	path := writeTempFile(t, "Time|Source|Host|User|Description\n1234567890|FILE|HOST1|admin|test\n")
	if err := ValidateFile(path); err != nil {
		t.Errorf("expected valid TLN, got: %v", err)
	}
}

func TestValidateFile_L2TTLNHeader(t *testing.T) {
	path := writeTempFile(t, "Time|Source|Host|User|Description|TZ|Notes\n1234567890|FILE|HOST1|admin|test|UTC|notes\n")
	if err := ValidateFile(path); err != nil {
		t.Errorf("expected valid L2TTLN, got: %v", err)
	}
}

func TestValidateFile_NoHeaderTLN(t *testing.T) {
	path := writeTempFile(t, "1234567890|FILE|HOST1|admin|test event\n")
	if err := ValidateFile(path); err != nil {
		t.Errorf("expected valid TLN without header, got: %v", err)
	}
}

func TestValidateFile_NoHeaderL2TTLN(t *testing.T) {
	path := writeTempFile(t, "1234567890|FILE|HOST1|admin|test event|UTC|notes\n")
	if err := ValidateFile(path); err != nil {
		t.Errorf("expected valid L2TTLN without header, got: %v", err)
	}
}

func TestValidateFile_EmptyFile(t *testing.T) {
	path := writeTempFile(t, "")
	if err := ValidateFile(path); err == nil {
		t.Error("expected error for empty file")
	}
}

func TestValidateFile_InvalidFormat(t *testing.T) {
	path := writeTempFile(t, "this,is,a,csv,file,with,too,many,fields\n")
	if err := ValidateFile(path); err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	if err := ValidateFile("/nonexistent/file.tln"); err == nil {
		t.Error("expected error for missing file")
	}
}

// --- TLN Read Tests ---

func TestReadEvents_TLNWithHeader(t *testing.T) {
	content := "Time|Source|Host|User|Description\n1539100800|FILE|WORKSTATION|admin|2018-10-09T16:00:00+00:00; Content Modification Time; /Users/admin/test.txt\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != "TLN" {
		t.Errorf("format = %q, want TLN", result.Format)
	}
	if result.Count != 1 {
		t.Fatalf("count = %d, want 1", result.Count)
	}

	e := result.Events[0]
	if e.Datetime != "2018-10-09 16:00:00" {
		t.Errorf("datetime = %q, want 2018-10-09 16:00:00", e.Datetime)
	}
	if e.Source != "FILE" {
		t.Errorf("source = %q, want FILE", e.Source)
	}
	if e.Host != "WORKSTATION" {
		t.Errorf("host = %q, want WORKSTATION", e.Host)
	}
	if e.User != "admin" {
		t.Errorf("user = %q, want admin", e.User)
	}
	if e.Type != "Content Modification Time" {
		t.Errorf("type = %q, want Content Modification Time", e.Type)
	}
	if e.MACB != "M..." {
		t.Errorf("MACB = %q, want M...", e.MACB)
	}
	if e.Desc != "/Users/admin/test.txt" {
		t.Errorf("desc = %q, want /Users/admin/test.txt", e.Desc)
	}
	if e.Timezone != "UTC" {
		t.Errorf("timezone = %q, want UTC", e.Timezone)
	}
}

func TestReadEvents_TLNWithoutHeader(t *testing.T) {
	content := "1539100800|FILE|HOST1|admin|2018-10-09T16:00:00; Last Access Time; accessed file\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != "TLN" {
		t.Errorf("format = %q, want TLN", result.Format)
	}
	if result.Count != 1 {
		t.Fatalf("count = %d, want 1", result.Count)
	}
	if result.Events[0].MACB != ".A.." {
		t.Errorf("MACB = %q, want .A..", result.Events[0].MACB)
	}
}

func TestReadEvents_L2TTLNWithHeader(t *testing.T) {
	content := "Time|Source|Host|User|Description|TZ|Notes\n1539100800|EVT|WORKSTATION|SYSTEM|2018-10-09T16:00:00; Creation Time; Event log entry|America/New_York|File: /var/log/syslog inode: 12345\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != "L2TTLN" {
		t.Errorf("format = %q, want L2TTLN", result.Format)
	}
	if result.Count != 1 {
		t.Fatalf("count = %d, want 1", result.Count)
	}

	e := result.Events[0]
	if e.Timezone != "America/New_York" {
		t.Errorf("timezone = %q, want America/New_York", e.Timezone)
	}
	if e.Filename != "/var/log/syslog" {
		t.Errorf("filename = %q, want /var/log/syslog", e.Filename)
	}
	if e.Inode != "12345" {
		t.Errorf("inode = %q, want 12345", e.Inode)
	}
	if e.MACB != "...B" {
		t.Errorf("MACB = %q, want ...B", e.MACB)
	}
	if e.Type != "Creation Time" {
		t.Errorf("type = %q, want Creation Time", e.Type)
	}
}

func TestReadEvents_ZeroTimestamp(t *testing.T) {
	content := "Time|Source|Host|User|Description\n0|FILE|HOST1|admin|no time event\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Datetime != "Not a time" {
		t.Errorf("datetime = %q, want 'Not a time'", result.Events[0].Datetime)
	}
}

func TestReadEvents_InvalidTimestampSkipped(t *testing.T) {
	content := "Time|Source|Host|User|Description\nabc|FILE|HOST1|admin|bad line\n1539100800|FILE|HOST1|admin|good line\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Errorf("count = %d, want 1", result.Count)
	}
	if result.Excluded != 1 {
		t.Errorf("excluded = %d, want 1", result.Excluded)
	}
}

func TestReadEvents_MultipleEvents(t *testing.T) {
	content := "Time|Source|Host|User|Description\n1539100800|FILE|HOST1|admin|event one\n1539187200|REG|HOST1|admin|event two\n1539273600|EVT|HOST2|SYSTEM|event three\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 3 {
		t.Errorf("count = %d, want 3", result.Count)
	}
}

func TestReadEvents_BlankLinesSkipped(t *testing.T) {
	content := "Time|Source|Host|User|Description\n\n1539100800|FILE|HOST1|admin|event\n\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Errorf("count = %d, want 1", result.Count)
	}
}

func TestReadEvents_L2TTLNDashTimezone(t *testing.T) {
	content := "Time|Source|Host|User|Description|TZ|Notes\n1539100800|FILE|HOST1|admin|event|-|-\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Timezone != "UTC" {
		t.Errorf("timezone = %q, want UTC", result.Events[0].Timezone)
	}
	if result.Events[0].Notes != "" {
		t.Errorf("notes = %q, want empty", result.Events[0].Notes)
	}
}

func TestReadEvents_L2TTLNNotesWithoutInode(t *testing.T) {
	content := "Time|Source|Host|User|Description|TZ|Notes\n1539100800|FILE|HOST1|admin|event|UTC|File: /path/to/file\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Filename != "/path/to/file" {
		t.Errorf("filename = %q, want /path/to/file", result.Events[0].Filename)
	}
}

func TestReadEvents_ProgressCallback(t *testing.T) {
	var lines []string
	lines = append(lines, "Time|Source|Host|User|Description")
	for i := 0; i < 20000; i++ {
		lines = append(lines, "1539100800|FILE|HOST1|admin|event")
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
		t.Errorf("callbacks = %d, want 2 (at 10000 and 20000)", len(callbacks))
	}
}

func TestReadEvents_DescriptionNoSemicolons(t *testing.T) {
	content := "Time|Source|Host|User|Description\n1539100800|FILE|HOST1|admin|simple description without semicolons\n"
	path := writeTempFile(t, content)

	result, err := ReadEvents(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := result.Events[0]
	if e.Desc != "simple description without semicolons" {
		t.Errorf("desc = %q, want 'simple description without semicolons'", e.Desc)
	}
	if e.Type != "" {
		t.Errorf("type = %q, want empty", e.Type)
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
		{"Last Written", "M..."},
		{"File Modified", "M..."},
		{"Entry Modified", "M.C."},
		{"UNKNOWN", "...."},
		{"", "...."},
	}

	for _, tt := range tests {
		got := mapTimestampDescToMACB(tt.desc)
		if got != tt.want {
			t.Errorf("mapTimestampDescToMACB(%q) = %q, want %q", tt.desc, got, tt.want)
		}
	}
}
