package unifiedlogparser

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

const bom = "\xEF\xBB\xBF"

// v070Header is the exact unifiedlog_iterator v0.7.0 CSV header.
var v070Header = []string{
	"Timestamp", "Event Type", "Log Type", "Subsystem", "Thread ID", "PID", "EUID",
	"Library", "Library UUID", "Activity ID", "Parent Activity ID", "Category",
	"Process", "Process UUID", "Message", "Raw Message", "Boot UUID", "System Timezone Name",
}

// sampleJSONLine is the real single-line record from the format notes.
const sampleJSONLine = `{"subsystem":"","thread_id":2093925,"pid":5856,"euid":501,"library":"/usr/bin/logger","library_uuid":"C644A1F3927539A881F135389AFCCB11","activity_id":0,"parent_activity_id":0,"time":1.791384828542682e+18,"category":"","event_type":"Log","log_type":"Default","process":"/usr/bin/logger","process_uuid":"C644A1F3927539A881F135389AFCCB11","message":"4n6time-test-marker","raw_message":"%s","boot_uuid":"BDC29971B7954B4C84BB0F66C67C7F58","timezone_name":"UTC","message_entries":[{"item_type":33,"item_type_size":4,"offset":0,"item_size":20,"message_strings":"4n6time-test-marker","item":"PrivateString"}],"timestamp":"2026-10-07T14:53:48.542682112Z","message_flags":["HasPrivateData","MainExe"],"evidence":"D:\\cases\\testdata\\ul-sample.logarchive\\Persist\\0000000000000078.tracev3"}`

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

// csvContent renders header and rows with encoding/csv so multiline fields
// are quoted exactly as a real export would quote them.
func csvContent(t *testing.T, header []string, rows [][]string) string {
	t.Helper()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		t.Fatalf("writing header: %v", err)
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			t.Fatalf("writing row: %v", err)
		}
	}
	w.Flush()
	return buf.String()
}

// v070Row builds a v0.7.0 CSV row from the fields the tests vary.
func v070Row(ts, subsystem, message, rawMessage string) []string {
	return []string{
		ts, "Log", "Default", subsystem, "2093925", "5856", "501",
		"/usr/bin/logger", "C644A1F3927539A881F135389AFCCB11", "0", "0", "",
		"/usr/bin/logger", "C644A1F3927539A881F135389AFCCB11", message, rawMessage,
		"BDC29971B7954B4C84BB0F66C67C7F58", "UTC",
	}
}

func extraHas(t *testing.T, e *model.Event, pair string) {
	t.Helper()
	if !strings.Contains(e.Extra, pair) {
		t.Errorf("extra %q missing %q", e.Extra, pair)
	}
}

// extraTimestamp asserts that the first extra entry is the full-precision
// source timestamp.
func extraTimestamp(t *testing.T, e *model.Event, want string) {
	t.Helper()
	first := strings.SplitN(e.Extra, "; ", 2)[0]
	if first != "timestamp: "+want {
		t.Errorf("first extra entry = %q, want %q (extra: %q)", first, "timestamp: "+want, e.Extra)
	}
}

// --- CSV ---

func TestCSVDetectAndRoundTrip(t *testing.T) {
	content := bom + csvContent(t, v070Header, [][]string{
		v070Row("2026-10-07T14:53:48.542Z", "com.apple.test", "line one\nline two", "%s"),
		v070Row("2026-10-07T14:53:49.000Z", "com.apple.test", "", "raw fallback %d"),
		v070Row("2026-10-07T14:53:50.125Z", "", "no subsystem here", "%s"),
		v070Row("not-a-time", "com.apple.test", "bad row", "%s"),
	})
	path := writeFile(t, "ul.csv", content)

	ok, err := DetectCSV(path)
	if err != nil || !ok {
		t.Fatalf("DetectCSV = %v, %v; want true, nil", ok, err)
	}

	result, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if result.Count != 3 {
		t.Fatalf("Count = %d, want 3", result.Count)
	}
	if result.Excluded != 1 {
		t.Errorf("Excluded = %d, want 1 (unparseable timestamp)", result.Excluded)
	}

	multi := result.Events[0]
	if multi.Desc != "line one\nline two" {
		t.Errorf("multiline desc = %q", multi.Desc)
	}
	if multi.Datetime != "2026-10-07 14:53:48" {
		t.Errorf("datetime = %q, want %q", multi.Datetime, "2026-10-07 14:53:48")
	}
	if multi.Timezone != "UTC" || multi.MACB != "...." {
		t.Errorf("timezone/MACB = %q/%q, want UTC/....", multi.Timezone, multi.MACB)
	}
	if multi.Source != "UNIFIEDLOG" || multi.SourceType != "macOS Unified Log" {
		t.Errorf("source/sourcetype = %q/%q", multi.Source, multi.SourceType)
	}
	if multi.Type != "Log" || multi.EventType != "Default" {
		t.Errorf("type/event_type = %q/%q, want Log/Default", multi.Type, multi.EventType)
	}
	if multi.Filename != "/usr/bin/logger" {
		t.Errorf("filename = %q", multi.Filename)
	}
	if multi.SourceName != "com.apple.test" {
		t.Errorf("source_name = %q", multi.SourceName)
	}
	if multi.User != "501" {
		t.Errorf("user = %q, want 501", multi.User)
	}
	if multi.Host != "" {
		t.Errorf("host = %q, want empty", multi.Host)
	}
	extraTimestamp(t, multi, "2026-10-07T14:53:48.542Z")
	extraHas(t, multi, "pid: 5856")
	extraHas(t, multi, "thread_id: 2093925")
	extraHas(t, multi, "activity_id: 0")
	extraHas(t, multi, "boot_uuid: BDC29971B7954B4C84BB0F66C67C7F58")
	extraHas(t, multi, "raw_message: %s")
	if strings.Contains(multi.Extra, "category:") {
		t.Errorf("empty category should be omitted from extra: %q", multi.Extra)
	}
	if strings.Contains(multi.Extra, "evidence:") || strings.Contains(multi.Extra, "message_flags:") {
		t.Errorf("CSV extra should not carry JSONL-only keys: %q", multi.Extra)
	}

	extraTimestamp(t, result.Events[1], "2026-10-07T14:53:49.000Z")
	extraTimestamp(t, result.Events[2], "2026-10-07T14:53:50.125Z")

	if got := result.Events[1].Desc; got != "raw fallback %d" {
		t.Errorf("empty Message desc = %q, want Raw Message fallback", got)
	}
	if got := result.Events[2].SourceName; got != "" {
		t.Errorf("empty Subsystem source_name = %q, want blank", got)
	}
}

func TestCSVDetectWithoutParentActivityID(t *testing.T) {
	var header []string
	for _, h := range v070Header {
		if h != "Parent Activity ID" {
			header = append(header, h)
		}
	}
	row := []string{
		"2026-10-07T14:53:48.542Z", "Activity", "Create", "com.apple.test", "1", "2", "0",
		"/usr/lib/x", "AA", "42", "", "/usr/bin/y", "BB", "hello", "%s", "CC", "UTC",
	}
	path := writeFile(t, "old.csv", csvContent(t, header, [][]string{row}))

	ok, err := DetectCSV(path)
	if err != nil || !ok {
		t.Fatalf("DetectCSV(old header) = %v, %v; want true, nil", ok, err)
	}
	result, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1", result.Count)
	}
	e := result.Events[0]
	if e.Desc != "hello" || e.Filename != "/usr/bin/y" || e.Type != "Activity" {
		t.Errorf("columns mapped by position, not name: %+v", e)
	}
	extraTimestamp(t, e, "2026-10-07T14:53:48.542Z")
	extraHas(t, e, "activity_id: 42")
	if strings.Contains(e.Extra, "parent_activity_id") {
		t.Errorf("missing column should not appear in extra: %q", e.Extra)
	}
}

func TestCSVDetectRejectsEZToolsHeaders(t *testing.T) {
	headers := map[string]string{
		"EvtxECmd": "RecordNumber,EventRecordId,TimeCreated,EventId,Level,Provider,Channel,ProcessId,ThreadId,Computer,UserId,MapDescription,UserName,RemoteHost,PayloadData1,PayloadData2,PayloadData3,PayloadData4,PayloadData5,PayloadData6,ExecutableInfo,HiddenRecord,SourceFile,Keywords,ExtraDataOffset,Payload\n",
		"LECmd":    "SourceFile,TargetCreated,TargetModified,LocalPath,DriveType\n",
		"MFTECmd":  "EntryNumber,SequenceNumber,InUse,ParentEntryNumber,ParentSequenceNumber,ParentPath,FileName,Extension,FileSize,ReferenceCount,ReparseTarget,IsDirectory,HasAds,IsAds,SI<FN,uSecZeros,Copied,SiFlags,NameType,Created0x10,Created0x30,LastModified0x10,LastModified0x30,LastRecordChange0x10,LastRecordChange0x30,LastAccess0x10,LastAccess0x30,UpdateSequenceNumber,LogfileSequenceNumber,SecurityId,ObjectIdFileDroid,LoggedUtilStream,ZoneIdContents\n",
	}
	for name, header := range headers {
		path := writeFile(t, name+".csv", header)
		ok, err := DetectCSV(path)
		if err != nil {
			t.Errorf("%s: DetectCSV error: %v", name, err)
		}
		if ok {
			t.Errorf("%s header detected as Unified Log CSV", name)
		}
	}
}

// --- JSONL ---

func TestJSONLDetectAndRoundTrip(t *testing.T) {
	bigMessage := strings.Repeat("A", 150*1024)
	lines := []string{
		// Sample record, with a BOM on the first line.
		bom + sampleJSONLine,
		// Statedump with an activity_id beyond the int64 range.
		`{"subsystem":"com.apple.sd","thread_id":1,"pid":1,"euid":0,"library":"","library_uuid":"","activity_id":9223372036856713060,"parent_activity_id":9223372036856713061,"category":"","event_type":"Statedump","log_type":"Default","process":"/sbin/launchd","process_uuid":"AB","message":"state","raw_message":"","boot_uuid":"BDC29971B7954B4C84BB0F66C67C7F58","timezone_name":"UTC","timestamp":"2026-10-07T14:53:49.000000001Z","message_flags":[],"evidence":"/Volumes/x/Persist/1.tracev3"}`,
		// A single line over 100KB.
		`{"subsystem":"","thread_id":2,"pid":2,"euid":0,"activity_id":0,"parent_activity_id":0,"event_type":"Log","log_type":"Info","process":"/usr/bin/big","process_uuid":"CD","message":"` + bigMessage + `","raw_message":"%s","boot_uuid":"BDC29971B7954B4C84BB0F66C67C7F58","timezone_name":"UTC","timestamp":"2026-10-07T14:53:50Z"}`,
		// No "timestamp": falls back to "time".
		`{"subsystem":"","thread_id":3,"pid":3,"euid":0,"activity_id":0,"parent_activity_id":0,"time":1.791384828542682e+18,"event_type":"Log","log_type":"Error","process":"/usr/bin/fallback","process_uuid":"EF","message":"from time","raw_message":"","boot_uuid":"BDC29971B7954B4C84BB0F66C67C7F58","timezone_name":"UTC"}`,
		// Neither "timestamp" nor "time": skipped and counted.
		`{"subsystem":"","thread_id":4,"pid":4,"euid":0,"activity_id":0,"parent_activity_id":0,"event_type":"Log","log_type":"Fault","process":"/usr/bin/notime","process_uuid":"01","message":"no time","raw_message":"","boot_uuid":"BDC29971B7954B4C84BB0F66C67C7F58","timezone_name":"UTC"}`,
		"",
	}
	path := writeFile(t, "ul.jsonl", strings.Join(lines, "\n"))

	ok, err := DetectJSONL(path)
	if err != nil || !ok {
		t.Fatalf("DetectJSONL = %v, %v; want true, nil", ok, err)
	}

	result, err := ReadJSONL(path, nil)
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}
	if result.Count != 4 {
		t.Fatalf("Count = %d, want 4", result.Count)
	}
	if result.Excluded != 1 {
		t.Errorf("Excluded = %d, want 1 (record with no time)", result.Excluded)
	}

	sample := result.Events[0]
	if sample.Datetime != "2026-10-07 14:53:48" {
		t.Errorf("datetime = %q", sample.Datetime)
	}
	if sample.Desc != "4n6time-test-marker" || sample.User != "501" || sample.SourceName != "" {
		t.Errorf("sample mapping wrong: desc=%q user=%q source_name=%q", sample.Desc, sample.User, sample.SourceName)
	}
	// The "timestamp" string is stored verbatim, nanoseconds included.
	extraTimestamp(t, sample, "2026-10-07T14:53:48.542682112Z")
	extraHas(t, sample, "message_flags: HasPrivateData,MainExe")
	extraHas(t, sample, "evidence: Persist/0000000000000078.tracev3")
	extraHas(t, sample, "library_uuid: C644A1F3927539A881F135389AFCCB11")
	if strings.Contains(sample.Extra, "message_entries") || strings.Contains(sample.Extra, "PrivateString") {
		t.Errorf("message_entries must not be stored: %q", sample.Extra)
	}

	statedump := result.Events[1]
	if statedump.Type != "Statedump" {
		t.Errorf("statedump type = %q", statedump.Type)
	}
	extraTimestamp(t, statedump, "2026-10-07T14:53:49.000000001Z")
	extraHas(t, statedump, "activity_id: 9223372036856713060")
	extraHas(t, statedump, "parent_activity_id: 9223372036856713061")
	extraHas(t, statedump, "evidence: /Volumes/x/Persist/1.tracev3")

	big := result.Events[2]
	if len(big.Desc) != len(bigMessage) {
		t.Errorf("large line desc length = %d, want %d", len(big.Desc), len(bigMessage))
	}
	extraTimestamp(t, big, "2026-10-07T14:53:50Z")

	fallback := result.Events[3]
	if fallback.Desc != "from time" || fallback.Datetime != "2026-10-07 14:53:48" {
		t.Errorf("time fallback: desc=%q datetime=%q, want %q", fallback.Desc, fallback.Datetime, "2026-10-07 14:53:48")
	}
	// No "timestamp" string: the converted "time" value, as RFC3339Nano UTC.
	// 1.791384828542682e+18 ns is exactly ...48.542682 seconds.
	extraTimestamp(t, fallback, "2026-10-07T14:53:48.542682Z")
}

func TestDetectJSONLRejectsPlaso(t *testing.T) {
	plaso := `{"timestamp": 1705312200000000, "datetime": "2024-01-15T10:30:00+00:00", "timestamp_desc": "Last Written", "source_short": "FILE", "message": "test event", "parser": "mft"}` + "\n"
	ok, err := DetectJSONL(writeFile(t, "plaso.jsonl", plaso))
	if err != nil {
		t.Fatalf("DetectJSONL(plaso): %v", err)
	}
	if ok {
		t.Error("Plaso JSONL detected as Unified Log")
	}
}

func TestDetectJSONLNotJSON(t *testing.T) {
	if ok, err := DetectJSONL(writeFile(t, "bad.jsonl", "not json\n")); ok || err == nil {
		t.Errorf("DetectJSONL(not json) = %v, %v; want false, error", ok, err)
	}
}

func TestTrimEvidence(t *testing.T) {
	tests := []struct{ in, want string }{
		{`D:\cases\testdata\ul-sample.logarchive\Persist\0000000000000078.tracev3`, "Persist/0000000000000078.tracev3"},
		{"/Users/a/case.logarchive/Special/0000000000000001.tracev3", "Special/0000000000000001.tracev3"},
		{`C:\collection\Persist\0000000000000078.tracev3`, `C:\collection\Persist\0000000000000078.tracev3`},
		{"/private/var/db/diagnostics/Persist/1.tracev3", "/private/var/db/diagnostics/Persist/1.tracev3"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := trimEvidence(tt.in); got != tt.want {
			t.Errorf("trimEvidence(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTimeFromNanos(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"1.791384828542682e+18", "2026-10-07T14:53:48.542682Z", true},
		{"1791384828542682112", "2026-10-07T14:53:48.542682112Z", true},
		{"0", "", false},
		{"-5", "", false},
		{"abc", "", false},
	}
	for _, tt := range tests {
		got, ok := timeFromNanos(tt.in)
		if ok != tt.ok {
			t.Errorf("timeFromNanos(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && got.Format("2006-01-02T15:04:05.999999999Z") != tt.want {
			t.Errorf("timeFromNanos(%q) = %s, want %s", tt.in, got.Format("2006-01-02T15:04:05.999999999Z"), tt.want)
		}
	}
}

// TestFingerprintCSVMatchesJSONL confirms a CSV and a JSONL export of the
// same record fingerprint identically despite the precision difference.
func TestFingerprintCSVMatchesJSONL(t *testing.T) {
	csvPath := writeFile(t, "a.csv", csvContent(t, v070Header, [][]string{
		v070Row("2026-10-07T14:53:48.542Z", "", "4n6time-test-marker", "%s"),
	}))
	jsonPath := writeFile(t, "b.jsonl", sampleJSONLine+"\n")

	csvFP, err := FingerprintCSV(csvPath)
	if err != nil {
		t.Fatalf("FingerprintCSV: %v", err)
	}
	jsonFP, err := FingerprintJSONL(jsonPath)
	if err != nil {
		t.Fatalf("FingerprintJSONL: %v", err)
	}
	if csvFP != jsonFP {
		t.Errorf("fingerprints differ: csv %q, jsonl %q", csvFP, jsonFP)
	}
}
