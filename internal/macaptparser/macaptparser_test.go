package macaptparser

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cdtdelta/4n6time/internal/model"
)

// fixture is one output with the rows a test writes for it. Rows are keyed by
// column name so fixtures stay readable; fill turns them into positional
// slices in column order.
type fixture struct {
	label   string
	columns []string
	rows    []map[string]string
}

var fsEventsFixture = fixture{
	label:   fsEventsLabel,
	columns: fsEventsColumns,
	rows: []map[string]string{
		{
			"LogID": "42", "EventFlagsHex": "0x00000002", "EventType": "Create",
			"EventFlags": "Created", "Filepath": "/Users/tom/Documents/a.txt",
			"File_ID": "1234", "Log_Unknown": "7",
			"SourceModDate": "2026-03-04 05:06:07",
			"Source":        "/private/var/.fseventsd/0000000000000001",
		},
		{
			"LogID": "43", "EventFlagsHex": "0x00000100", "EventType": "Remove",
			"EventFlags": "Removed", "Filepath": "/Users/tom/Documents/b.txt",
			"File_ID": "1235", "Log_Unknown": "",
			"SourceModDate": "2026-03-04 05:06:08.5",
			"Source":        "/private/var/.fseventsd/0000000000000002",
		},
	},
}

var biomeFixture = fixture{
	label:   biomeLabel,
	columns: biomeColumns,
	rows: []map[string]string{
		{
			"Biome Type": "App.InFocus", "Filename timestamp": "2026-03-04 05:06:07.123456",
			"Offset": "99", "State": "Ended", "Data": `b'\x08\x00\x10\x01'`,
			"Timestamp1": "2026-03-04 05:06:07", "Timestamp2": "2026-03-04 05:06:09",
			"User": "tom", "Source": "/x/biome/App.InFocus/segment",
		},
		{
			"Biome Type": "Device.Lock", "Filename timestamp": "2026-03-04 05:07:00",
			"Offset": "100", "State": "", "Data": "plain text",
			"Timestamp1": "2026-03-04 05:07:00", "Timestamp2": "",
			"User": "-", "Source": "/x/biome/Device.Lock/segment",
		},
	},
}

var aslFixture = fixture{
	label:   aslLabel,
	columns: aslColumns,
	rows: []map[string]string{
		{
			"Timestamp": "2026-03-04 05:06:07+00:00", "Nano": "123456789", "ID": "5",
			"Flags": "8", "Host": "tom-mbp", "Sender": "loginwindow", "Facility": "auth",
			"Level": "5", "PID": "321", "UID": "501", "GID": "20", "RefProc": "",
			"RefPID": "0", "Message": "line one\nline two", "Session": "com.apple.x",
			"Key_Value": "k=v", "Source": "/private/var/log/asl/x.asl",
		},
		{
			"Timestamp": "2026-03-04 05:06:09+00:00", "Nano": "0", "ID": "6",
			"Flags": "0", "Host": "tom-mbp", "Sender": "sshd", "Facility": "auth",
			"Level": "4", "PID": "400", "UID": "0", "GID": "0", "RefProc": "launchd",
			"RefPID": "1", "Message": "accepted", "Session": "", "Key_Value": "",
			"Source": "/private/var/log/asl/y.asl",
		},
	},
}

var cfurlFixture = fixture{
	label:   cfurlLabel,
	columns: cfurlColumns,
	rows: []map[string]string{
		{
			"Date": "2026-03-04 05:06:07", "URL": "https://example.com/a", "Method": "GET",
			"Request_Header": "GET / HTTP/1.1\nHost: x", "HTTP_Status": "200",
			"Response_Header": "HTTP/1.1 200 OK\nContent-Type: text/html",
			"isDataOnFS":      "1", "Received_Data": "deadbeefcafe", "User": "501",
			"App_Bundle_ID": "com.apple.Safari", "Source": "/x/cfurl.db",
		},
		{
			"Date": "2026-03-04 05:06:10", "URL": "https://example.com/b", "Method": "POST",
			"Request_Header": "", "HTTP_Status": "404",
			"Response_Header": "", "isDataOnFS": "0", "Received_Data": "",
			"User": "501", "App_Bundle_ID": "", "Source": "/x/cfurl.db",
		},
	},
}

var allFixtures = []fixture{fsEventsFixture, biomeFixture, aslFixture, cfurlFixture}

// fill returns row values in column order; missing keys become "".
func fill(columns []string, row map[string]string) []string {
	out := make([]string, len(columns))
	for i, c := range columns {
		out[i] = row[c]
	}
	return out
}

func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

// csvFixture renders a header and rows as CSV, quoting as needed.
func csvFixture(t *testing.T, header []string, rows ...[]string) string {
	t.Helper()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, r := range append([][]string{header}, rows...) {
		if err := w.Write(r); err != nil {
			t.Fatalf("writing CSV fixture: %v", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatalf("flushing CSV fixture: %v", err)
	}
	return buf.String()
}

// jsonValue maps a fixture value to its JSON type the way mac_apt's JSONL
// writes it: digit-only text becomes a number, empty becomes null, and
// everything else stays a string.
func jsonValue(v string) interface{} {
	switch {
	case v == "":
		return nil
	case strings.Trim(v, "0123456789") == "":
		return json.Number(v)
	default:
		return v
	}
}

// jsonObject renders keys and values as one JSON object, keeping the order
// given so tests can shuffle keys.
func jsonObject(t *testing.T, keys, values []string) string {
	t.Helper()
	parts := make([]string, 0, len(keys))
	for i, k := range keys {
		kb, err := json.Marshal(k)
		if err != nil {
			t.Fatalf("marshaling key: %v", err)
		}
		vb, err := json.Marshal(jsonValue(values[i]))
		if err != nil {
			t.Fatalf("marshaling value: %v", err)
		}
		parts = append(parts, string(kb)+":"+string(vb))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// jsonlFixture renders rows as JSONL in column order.
func jsonlFixture(t *testing.T, columns []string, rows ...[]string) string {
	t.Helper()
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(jsonObject(t, columns, r))
		b.WriteString("\n")
	}
	return b.String()
}

// csvRows returns fixture rows as positional slices.
func (f fixture) csvRows() [][]string {
	out := make([][]string, 0, len(f.rows))
	for _, r := range f.rows {
		out = append(out, fill(f.columns, r))
	}
	return out
}

// --- Detection ---

func TestDetectCSVEachOutput(t *testing.T) {
	for _, f := range allFixtures {
		path := writeFixture(t, "out.csv", csvFixture(t, f.columns, f.csvRows()...))
		label, ok, err := DetectCSV(path)
		if err != nil || !ok || label != f.label {
			t.Errorf("DetectCSV(%s) = (%q, %v, %v), want (%q, true, nil)", f.label, label, ok, err, f.label)
		}
	}
}

func TestDetectJSONLEachOutput(t *testing.T) {
	for _, f := range allFixtures {
		path := writeFixture(t, "out.jsonl", jsonlFixture(t, f.columns, f.csvRows()...))
		label, ok, err := DetectJSONL(path)
		if err != nil || !ok || label != f.label {
			t.Errorf("DetectJSONL(%s) = (%q, %v, %v), want (%q, true, nil)", f.label, label, ok, err, f.label)
		}
	}
}

func TestDetectCSVRejectsChangedHeader(t *testing.T) {
	renamed := append([]string(nil), fsEventsColumns...)
	renamed[4] = "FilePath"
	missing := fsEventsColumns[:len(fsEventsColumns)-1]
	reordered := append([]string(nil), fsEventsColumns...)
	reordered[0], reordered[1] = reordered[1], reordered[0]

	cases := map[string][]string{
		"renamed column": renamed,
		"missing column": missing,
		"reordered":      reordered,
	}
	for name, header := range cases {
		path := writeFixture(t, "bad.csv", csvFixture(t, header, fill(header, nil)))
		if _, ok, err := DetectCSV(path); ok || err != nil {
			t.Errorf("%s: DetectCSV = (%v, %v), want not detected", name, ok, err)
		}
	}
}

func TestDetectCSVBiomeSpacesMatter(t *testing.T) {
	// The Biome column names contain spaces; removing them must not match.
	header := append([]string(nil), biomeColumns...)
	header[0] = "BiomeType"
	path := writeFixture(t, "biome.csv", csvFixture(t, header))
	if _, ok, _ := DetectCSV(path); ok {
		t.Errorf("DetectCSV accepted Biome header with \"BiomeType\"")
	}
}

func TestDetectJSONLKeyOrderIgnored(t *testing.T) {
	keys := append([]string(nil), cfurlColumns...)
	values := fill(cfurlColumns, cfurlFixture.rows[0])
	for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
		keys[i], keys[j] = keys[j], keys[i]
		values[i], values[j] = values[j], values[i]
	}
	path := writeFixture(t, "shuffled.jsonl", jsonObject(t, keys, values)+"\n")

	label, ok, err := DetectJSONL(path)
	if err != nil || !ok || label != cfurlLabel {
		t.Errorf("DetectJSONL(shuffled) = (%q, %v, %v), want (%q, true, nil)", label, ok, err, cfurlLabel)
	}
}

func TestDetectJSONLRejectsKeyMismatch(t *testing.T) {
	extra := append(append([]string(nil), aslColumns...), "Extra_Key")
	values := append(fill(aslColumns, aslFixture.rows[0]), "x")
	path := writeFixture(t, "extra.jsonl", jsonObject(t, extra, values)+"\n")
	if _, ok, _ := DetectJSONL(path); ok {
		t.Errorf("DetectJSONL accepted object with an extra key")
	}

	missing := aslColumns[:len(aslColumns)-1]
	mValues := fill(aslColumns, aslFixture.rows[0])[:len(aslColumns)-1]
	path = writeFixture(t, "missing.jsonl", jsonObject(t, missing, mValues)+"\n")
	if _, ok, _ := DetectJSONL(path); ok {
		t.Errorf("DetectJSONL accepted object missing a key")
	}
}

func TestDetectJSONLNonObjectErrors(t *testing.T) {
	path := writeFixture(t, "array.jsonl", "[1,2,3]\n")
	if _, _, err := DetectJSONL(path); err == nil {
		t.Errorf("DetectJSONL on a non-object first line returned no error")
	}
}

// --- Negative detection against neighboring formats ---
//
// The cross-format checks that reach other packages live in the importer
// tests, where the neighbors are already wired up.

// --- Round trip ---

// TestRoundTripCSVAndJSONL verifies that the same rows produce identical
// events from either format, for every output.
func TestRoundTripCSVAndJSONL(t *testing.T) {
	for _, f := range allFixtures {
		csvPath := writeFixture(t, "rt.csv", csvFixture(t, f.columns, f.csvRows()...))
		jsonlPath := writeFixture(t, "rt.jsonl", jsonlFixture(t, f.columns, f.csvRows()...))

		fromCSV, err := ReadCSV(csvPath, nil)
		if err != nil {
			t.Fatalf("%s: ReadCSV: %v", f.label, err)
		}
		fromJSON, err := ReadJSONL(jsonlPath, nil)
		if err != nil {
			t.Fatalf("%s: ReadJSONL: %v", f.label, err)
		}
		if len(fromCSV.Events) == 0 {
			t.Fatalf("%s: no events read from CSV", f.label)
		}
		if !reflect.DeepEqual(fromCSV.Events, fromJSON.Events) {
			t.Errorf("%s: CSV and JSONL events differ\nCSV:  %+v\nJSON: %+v",
				f.label, derefEvents(fromCSV.Events), derefEvents(fromJSON.Events))
		}
	}
}

func derefEvents(events []*model.Event) []model.Event {
	out := make([]model.Event, 0, len(events))
	for _, e := range events {
		out = append(out, *e)
	}
	return out
}

func TestFsEventsMapping(t *testing.T) {
	path := writeFixture(t, "fs.csv", csvFixture(t, fsEventsColumns, fsEventsFixture.csvRows()...))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if res.Count != 2 || len(res.Events) != 2 {
		t.Fatalf("Count = %d, events = %d, want 2", res.Count, len(res.Events))
	}
	e := res.Events[0]
	want := model.Event{
		Datetime:     "2026-03-04 05:06:07",
		Timezone:     "UTC",
		MACB:         "....",
		Source:       "FSEVENTS",
		SourceType:   "mac_apt FsEvents",
		Type:         "FSEvents log modified (approximate)",
		Desc:         "Created: /Users/tom/Documents/a.txt",
		Filename:     "/private/var/.fseventsd/0000000000000001",
		Inode:        "1234",
		RecordNumber: "42",
		EventType:    "Create",
		Extra:        "SourceModDate: 2026-03-04 05:06:07; EventFlagsHex: 0x00000002; Log_Unknown: 7",
	}
	if !reflect.DeepEqual(*e, want) {
		t.Errorf("FsEvents event =\n%+v\nwant\n%+v", *e, want)
	}
	// The second row has an empty Log_Unknown, which must drop out of extra,
	// and a fractional source timestamp, which must keep its full precision.
	if got := res.Events[1].Extra; got != "SourceModDate: 2026-03-04 05:06:08.5; EventFlagsHex: 0x00000100" {
		t.Errorf("second FsEvents extra = %q", got)
	}
}

func TestBiomeMapping(t *testing.T) {
	path := writeFixture(t, "biome.csv", csvFixture(t, biomeColumns, biomeFixture.csvRows()...))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	// Row one has two timestamps, row two has one plus an empty one. The
	// empty Timestamp2 yields no event and is not an unparseable value.
	if res.Count != 3 {
		t.Fatalf("Count = %d, want 3", res.Count)
	}
	if res.UnparseableTimestamps != 0 {
		t.Errorf("UnparseableTimestamps = %d, want 0 (empty values are not counted)", res.UnparseableTimestamps)
	}

	first := res.Events[0]
	if first.Type != "Timestamp1" || first.Datetime != "2026-03-04 05:06:07" {
		t.Errorf("first Biome event type/datetime = %q/%q", first.Type, first.Datetime)
	}
	if first.Desc != `App.InFocus: b'\x08\x00\x10\x01'` {
		t.Errorf("Biome Data changed: desc = %q", first.Desc)
	}
	if first.User != "tom" || first.RecordNumber != "99" || first.EventType != "App.InFocus" {
		t.Errorf("Biome user/record/event_type = %q/%q/%q", first.User, first.RecordNumber, first.EventType)
	}
	if got, want := first.Extra, "Timestamp1: 2026-03-04 05:06:07; Filename timestamp: 2026-03-04 05:06:07.123456; State: Ended"; got != want {
		t.Errorf("Biome extra = %q, want %q", got, want)
	}
	if second := res.Events[1]; second.Type != "Timestamp2" || second.Datetime != "2026-03-04 05:06:09" {
		t.Errorf("second Biome event type/datetime = %q/%q", second.Type, second.Datetime)
	}

	last := res.Events[2]
	if last.User != "" {
		t.Errorf("Biome user \"-\" mapped to %q, want empty", last.User)
	}
	if last.Desc != "Device.Lock: plain text" {
		t.Errorf("row two desc = %q", last.Desc)
	}
}

func TestASLMapping(t *testing.T) {
	path := writeFixture(t, "asl.csv", csvFixture(t, aslColumns, aslFixture.csvRows()...))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	e := res.Events[0]
	if e.Datetime != "2026-03-04 05:06:07" || e.Host != "tom-mbp" || e.User != "501" {
		t.Errorf("ASL datetime/host/user = %q/%q/%q", e.Datetime, e.Host, e.User)
	}
	if e.Type != "5" || e.SourceName != "loginwindow" || e.EventType != "auth" || e.RecordNumber != "5" {
		t.Errorf("ASL type/source_name/event_type/record = %q/%q/%q/%q", e.Type, e.SourceName, e.EventType, e.RecordNumber)
	}
	if e.Desc != "line one\nline two" {
		t.Errorf("ASL desc = %q", e.Desc)
	}
	want := "Timestamp: 2026-03-04 05:06:07.123456789+00:00; PID: 321; GID: 20; RefPID: 0; Session: com.apple.x; Flags: 8; Key_Value: k=v"
	if e.Extra != want {
		t.Errorf("ASL extra =\n%q\nwant\n%q", e.Extra, want)
	}
	// Nano of zero leaves the raw timestamp unchanged, offset included.
	if got := res.Events[1].Extra; !strings.HasPrefix(got, "Timestamp: 2026-03-04 05:06:09+00:00; ") {
		t.Errorf("zero-nano ASL extra = %q", got)
	}
}

func TestCFURLCacheMapping(t *testing.T) {
	path := writeFixture(t, "cfurl.csv", csvFixture(t, cfurlColumns, cfurlFixture.csvRows()...))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	e := res.Events[0]
	if e.Type != "HTTP request cached" || e.Desc != "GET https://example.com/a (200)" {
		t.Errorf("CFURL type/desc = %q/%q", e.Type, e.Desc)
	}
	if e.URL != "https://example.com/a" || e.User != "501" || e.SourceName != "com.apple.Safari" || e.EventType != "200" {
		t.Errorf("CFURL url/user/source_name/event_type = %q/%q/%q/%q", e.URL, e.User, e.SourceName, e.EventType)
	}
	want := "Date: 2026-03-04 05:06:07; isDataOnFS: 1; Request_Header: GET / HTTP/1.1 Host: x; Response_Header: HTTP/1.1 200 OK Content-Type: text/html"
	if e.Extra != want {
		t.Errorf("CFURL extra =\n%q\nwant\n%q", e.Extra, want)
	}
}

// TestCFURLReceivedDataAbsent verifies the hex blob never reaches any event
// field, in either format.
func TestCFURLReceivedDataAbsent(t *testing.T) {
	csvPath := writeFixture(t, "cfurl.csv", csvFixture(t, cfurlColumns, cfurlFixture.csvRows()...))
	jsonPath := writeFixture(t, "cfurl.jsonl", jsonlFixture(t, cfurlColumns, cfurlFixture.csvRows()...))
	for _, read := range []func() (*ReadResult, error){
		func() (*ReadResult, error) { return ReadCSV(csvPath, nil) },
		func() (*ReadResult, error) { return ReadJSONL(jsonPath, nil) },
	} {
		res, err := read()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		for _, e := range res.Events {
			if strings.Contains(fmt.Sprintf("%+v", *e), "deadbeefcafe") {
				t.Errorf("Received_Data leaked into event: %+v", *e)
			}
		}
	}
}

// --- Timestamps ---

func TestParseTimestampLayouts(t *testing.T) {
	base := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"2026-03-04 05:06:07", base, true},
		{"2026-03-04 05:06:07.5", base.Add(500 * time.Millisecond), true},
		{"2026-03-04 05:06:07.123456", base.Add(123456 * time.Microsecond), true},
		{"2026-03-04 05:06:07+00:00", base, true},
		{"2026-03-04 05:06:07.123456+00:00", base.Add(123456 * time.Microsecond), true},
		{"2026-03-04 05:06:07+05:00", time.Time{}, false},
		{"2026-03-04T05:06:07Z", time.Time{}, false},
		{"", time.Time{}, false},
		{"not a time", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := parseTimestamp(c.in)
		if ok != c.ok || (ok && !got.Equal(c.want)) {
			t.Errorf("parseTimestamp(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestBadTimestampsSkipWithoutFailing verifies that a non-empty value that
// does not parse drops its event and is counted, while the import continues.
// Empty values drop their event without being counted.
func TestBadTimestampsSkipWithoutFailing(t *testing.T) {
	good := fsEventsFixture.rows[0]
	empty := map[string]string{"LogID": "50", "SourceModDate": "", "Source": "/s"}
	bogus := map[string]string{"LogID": "51", "SourceModDate": "yesterday", "Source": "/s"}

	csvPath := writeFixture(t, "bad.csv", csvFixture(t, fsEventsColumns,
		fill(fsEventsColumns, good), fill(fsEventsColumns, empty), fill(fsEventsColumns, bogus)))
	res, err := ReadCSV(csvPath, nil)
	if err != nil {
		t.Fatalf("ReadCSV with bad timestamps: %v", err)
	}
	if res.Count != 1 || res.UnparseableTimestamps != 1 {
		t.Errorf("CSV Count = %d, UnparseableTimestamps = %d, want 1 and 1", res.Count, res.UnparseableTimestamps)
	}

	jsonPath := writeFixture(t, "bad.jsonl", jsonlFixture(t, fsEventsColumns,
		fill(fsEventsColumns, good), fill(fsEventsColumns, empty), fill(fsEventsColumns, bogus)))
	res, err = ReadJSONL(jsonPath, nil)
	if err != nil {
		t.Fatalf("ReadJSONL with bad timestamps: %v", err)
	}
	if res.Count != 1 || res.UnparseableTimestamps != 1 {
		t.Errorf("JSONL Count = %d, UnparseableTimestamps = %d, want 1 and 1", res.Count, res.UnparseableTimestamps)
	}
}

// TestEmptyTimestampNotCounted verifies an empty timestamp yields no event
// and leaves the unparseable count at zero.
func TestEmptyTimestampNotCounted(t *testing.T) {
	empty := map[string]string{"LogID": "50", "SourceModDate": "", "Source": "/s"}
	path := writeFixture(t, "empty.csv", csvFixture(t, fsEventsColumns, fill(fsEventsColumns, empty)))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if res.Count != 0 || res.UnparseableTimestamps != 0 {
		t.Errorf("Count = %d, UnparseableTimestamps = %d, want 0 and 0", res.Count, res.UnparseableTimestamps)
	}
}

// TestBiomeEmptyTimestamp2 verifies the common Biome case: Timestamp2 empty,
// Timestamp1 valid. That row yields one event and no unparseable count.
func TestBiomeEmptyTimestamp2(t *testing.T) {
	row := fill(biomeColumns, map[string]string{
		"Biome Type": "App.InFocus", "Timestamp1": "2026-03-04 05:06:07", "Timestamp2": "",
		"Data": "x", "User": "tom", "Source": "/s",
	})
	path := writeFixture(t, "biome.csv", csvFixture(t, biomeColumns, row))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(res.Events) != 1 || res.UnparseableTimestamps != 0 {
		t.Errorf("events = %d, UnparseableTimestamps = %d, want 1 and 0", len(res.Events), res.UnparseableTimestamps)
	}
}

// TestBiomeGarbageTimestampPerColumn verifies a garbage value in one column
// drops only that column's event, counts once, and keeps the other column.
func TestBiomeGarbageTimestampPerColumn(t *testing.T) {
	row := fill(biomeColumns, map[string]string{
		"Biome Type": "App.InFocus", "Timestamp1": "not-a-time", "Timestamp2": "2026-03-04 05:06:09",
		"Data": "x", "User": "tom", "Source": "/s",
	})
	path := writeFixture(t, "biome.csv", csvFixture(t, biomeColumns, row))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(res.Events) != 1 || res.UnparseableTimestamps != 1 {
		t.Fatalf("events = %d, UnparseableTimestamps = %d, want 1 and 1", len(res.Events), res.UnparseableTimestamps)
	}
	if res.Events[0].Type != "Timestamp2" {
		t.Errorf("surviving event type = %q, want Timestamp2", res.Events[0].Type)
	}
}

func TestJSONLMalformedLineCountedNotFatal(t *testing.T) {
	content := jsonObject(t, fsEventsColumns, fill(fsEventsColumns, fsEventsFixture.rows[0])) +
		"\n{not json\n" +
		jsonObject(t, fsEventsColumns, fill(fsEventsColumns, fsEventsFixture.rows[1])) + "\n"
	path := writeFixture(t, "malformed.jsonl", content)
	res, err := ReadJSONL(path, nil)
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}
	if res.Count != 2 || res.Excluded != 1 {
		t.Errorf("Count = %d, Excluded = %d, want 2 and 1", res.Count, res.Excluded)
	}
}

// --- CSV edge cases ---

// TestCSVEmbeddedCRLF verifies a quoted field holding CRLF stays inside its
// row, and the rows after it still parse.
func TestCSVEmbeddedCRLF(t *testing.T) {
	header := strings.Join(cfurlColumns, ",")
	first := `"2026-03-04 05:06:07","https://example.com/a","GET","GET / HTTP/1.1` + "\r\nHost: x" +
		`","200","HTTP/1.1 200 OK` + "\r\n" + `Content-Type: text/html","1","","501","com.apple.Safari","/x/cfurl.db"`
	second := `"2026-03-04 05:06:10","https://example.com/b","POST","","404","","0","","501","","/x/cfurl.db"`
	path := writeFixture(t, "crlf.csv", header+"\r\n"+first+"\r\n"+second+"\r\n")

	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if res.Count != 2 || res.Excluded != 0 {
		t.Fatalf("Count = %d, Excluded = %d, want 2 and 0", res.Count, res.Excluded)
	}
	if got := strings.ReplaceAll(res.Events[0].Desc, "\r", ""); got != "GET https://example.com/a (200)" {
		t.Errorf("first desc = %q", got)
	}
	if got := strings.ReplaceAll(res.Events[0].Extra, "\r", ""); !strings.Contains(got, "Request_Header: GET / HTTP/1.1 Host: x") {
		t.Errorf("CRLF in request header not flattened in extra: %q", got)
	}
	if res.Events[1].URL != "https://example.com/b" {
		t.Errorf("second row URL = %q, want https://example.com/b", res.Events[1].URL)
	}
}

// TestNULBytesStripped verifies NUL bytes never reach an event field.
func TestNULBytesStripped(t *testing.T) {
	row := fill(cfurlColumns, cfurlFixture.rows[0])
	row[cfurlIndex(t, "App_Bundle_ID")] = "com.apple\x00Safari"
	path := writeFixture(t, "nul.csv", csvFixture(t, cfurlColumns, row))

	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	e := res.Events[0]
	if e.SourceName != "com.appleSafari" {
		t.Errorf("SourceName = %q, want NUL removed", e.SourceName)
	}
	if strings.ContainsRune(fmt.Sprintf("%+v", *e), 0) {
		t.Errorf("event still holds a NUL byte")
	}
}

func cfurlIndex(t *testing.T, column string) int {
	t.Helper()
	for i, c := range cfurlColumns {
		if c == column {
			return i
		}
	}
	t.Fatalf("no CFURL column %q", column)
	return -1
}

// TestProgressCallbackFires verifies the progress hook fires once per
// 10,000 events during a streaming read.
func TestProgressCallbackFires(t *testing.T) {
	var rows []string
	for i := 0; i < 25000; i++ {
		row := fill(fsEventsColumns, fsEventsFixture.rows[0])
		row[0] = fmt.Sprint(i)
		rows = append(rows, strings.Join(row, ","))
	}
	content := strings.Join(fsEventsColumns, ",") + "\n" + strings.Join(rows, "\n") + "\n"
	path := writeFixture(t, "many.csv", content)

	var calls []int
	res, err := ReadCSV(path, func(n int) { calls = append(calls, n) })
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if res.Count != 25000 {
		t.Errorf("Count = %d, want 25000", res.Count)
	}
	if want := []int{10000, 20000}; !reflect.DeepEqual(calls, want) {
		t.Errorf("progress calls = %v, want %v", calls, want)
	}
}

// --- Helpers ---

func TestASLFullPrecision(t *testing.T) {
	cases := []struct{ ts, nano, want string }{
		{"2026-03-04 05:06:07+00:00", "123456789", "2026-03-04 05:06:07.123456789+00:00"},
		{"2026-03-04 05:06:07+00:00", "5", "2026-03-04 05:06:07.000000005+00:00"},
		{"2026-03-04 05:06:07", "10", "2026-03-04 05:06:07.000000010"},
		{"2026-03-04 05:06:07+00:00", "0", "2026-03-04 05:06:07+00:00"},
		{"2026-03-04 05:06:07+00:00", "", "2026-03-04 05:06:07+00:00"},
		{"2026-03-04 05:06:07.5+00:00", "123", "2026-03-04 05:06:07.5+00:00"},
	}
	for _, c := range cases {
		if got := aslFullPrecision(c.ts, c.nano); got != c.want {
			t.Errorf("aslFullPrecision(%q, %q) = %q, want %q", c.ts, c.nano, got, c.want)
		}
	}
}

func TestRenderExtraFlattensLines(t *testing.T) {
	got := renderExtra(
		field{"a", "x\r\ny"},
		field{"b", ""},
		field{"c", "\n\n lead and trail \r\n"},
	)
	if want := "a: x y; c: lead and trail"; got != want {
		t.Errorf("renderExtra = %q, want %q", got, want)
	}
}
