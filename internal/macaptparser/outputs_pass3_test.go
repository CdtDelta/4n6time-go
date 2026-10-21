package macaptparser

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// --- Fixed-schema outputs: SpotlightConfig and Network_DHCP ---

var spotlightConfigFixture = fixture{
	label:   spotlightConfigLabel,
	columns: spotlightConfigColumns,
	rows: []map[string]string{
		{
			"StoreUUID": "uuid-1", "StoreCreationDate": "2026-03-04 05:06:07", "Version": "2",
			"IndexVersion": "3", "PartialPath": "/System/Volumes/Data/",
			"ConfigurationModificationDate":    "2026-03-05 00:00:00",
			"ConfigurationModificationVersion": "5", "ConfigurationVolumeUUID": "vol-1",
			"Source": "/private/var/db/Spotlight-V100/Stores/1/config",
		},
		{
			"StoreUUID": "uuid-2", "StoreCreationDate": "", "Version": "2", "IndexVersion": "3",
			"PartialPath": "/Volumes/Backup/", "ConfigurationModificationDate": "2026-03-06 00:00:00",
			"ConfigurationModificationVersion": "6", "ConfigurationVolumeUUID": "vol-2",
			"Source": "/private/var/db/Spotlight-V100/Stores/2/config",
		},
	},
}

var networkDHCPFixture = fixture{
	label:   networkDHCPLabel,
	columns: networkDHCPColumns,
	rows: []map[string]string{
		{
			"Interface": "en0", "MAC_Address": "aa:bb:cc:dd:ee:ff", "IPAddress": "192.168.1.20",
			"LeaseLength": "86400", "LeaseStartDate": "2026-03-04 05:06:07",
			"PacketData": "deadbeef0102", "RouterHardwareAddress": "11:22:33:44:55:66",
			"RouterIPAddress": "192.168.1.1", "SSID": "HomeNet",
			"Source": "/private/var/db/dhcpclient/leases/en0",
		},
		{
			"Interface": "en1", "IPAddress": "10.0.0.5", "LeaseStartDate": "",
			"PacketData": "cafe", "Source": "/private/var/db/dhcpclient/leases/en1",
		},
	},
}

// Untimed outputs: one row each, which must produce no events.
var networkDetailsFixture = fixture{
	label:   networkDetailsLabel,
	columns: networkDetailsColumns,
	rows: []map[string]string{
		{"UUID": "u1", "IPv4.ConfigMethod": "DHCP", "DeviceName": "en0", "Source": "/x/net.plist"},
	},
}

var networkInterfacesFixture = fixture{
	label:   networkInterfacesLabel,
	columns: networkInterfacesColumns,
	rows: []map[string]string{
		{"Category": "Ethernet", "Active": "Yes", "BSD Name": "en0", "Source": "/x/ifaces.plist"},
	},
}

var fileSharingFixture = fixture{
	label:   fileSharingLabel,
	columns: fileSharingColumns,
	rows: []map[string]string{
		{"Name": "Public", "Directory_Path": "/Users/tom/Public", "SMB_Guest_Access": "No", "Source": "/x/sharing.plist"},
	},
}

// pass3Fixtures pairs each fixture with the events its rows must produce.
var pass3Fixtures = []struct {
	f      fixture
	events int
}{
	{spotlightConfigFixture, 3},
	{networkDHCPFixture, 1},
	{networkDetailsFixture, 0},
	{networkInterfacesFixture, 0},
	{fileSharingFixture, 0},
}

func TestPass3DetectCSVAndJSONL(t *testing.T) {
	for _, c := range pass3Fixtures {
		csvPath := writeFixture(t, "p3.csv", csvFixture(t, c.f.columns, c.f.csvRows()...))
		if label, ok, err := DetectCSV(csvPath); err != nil || !ok || label != c.f.label {
			t.Errorf("DetectCSV(%s) = %q, %v, %v", c.f.label, label, ok, err)
		}
		jsonPath := writeFixture(t, "p3.jsonl", jsonlFixture(t, c.f.columns, c.f.csvRows()...))
		if label, ok, err := DetectJSONL(jsonPath); err != nil || !ok || label != c.f.label {
			t.Errorf("DetectJSONL(%s) = %q, %v, %v", c.f.label, label, ok, err)
		}
	}
}

// TestPass3RoundTripParity checks CSV and JSONL produce identical events and
// that each output yields its expected count. Untimed outputs must yield none.
func TestPass3RoundTripParity(t *testing.T) {
	for _, c := range pass3Fixtures {
		csvPath := writeFixture(t, "p3rt.csv", csvFixture(t, c.f.columns, c.f.csvRows()...))
		jsonPath := writeFixture(t, "p3rt.jsonl", jsonlFixture(t, c.f.columns, c.f.csvRows()...))

		fromCSV, err := ReadCSV(csvPath, nil)
		if err != nil {
			t.Fatalf("%s: ReadCSV: %v", c.f.label, err)
		}
		fromJSON, err := ReadJSONL(jsonPath, nil)
		if err != nil {
			t.Fatalf("%s: ReadJSONL: %v", c.f.label, err)
		}
		if len(fromCSV.Events) != c.events {
			t.Errorf("%s: %d events, want %d", c.f.label, len(fromCSV.Events), c.events)
		}
		if !reflect.DeepEqual(fromCSV.Events, fromJSON.Events) {
			t.Errorf("%s: CSV and JSONL events differ\nCSV:  %+v\nJSON: %+v",
				c.f.label, derefEvents(fromCSV.Events), derefEvents(fromJSON.Events))
		}
	}
}

// TestPass3NearMissHeadersRejected checks a header or key set one column short
// of each new fixed output is detected as nothing.
func TestPass3NearMissHeadersRejected(t *testing.T) {
	for _, c := range pass3Fixtures {
		for i := range c.f.columns {
			cols := append(append([]string{}, c.f.columns[:i]...), c.f.columns[i+1:]...)
			csvPath := writeFixture(t, "p3near.csv", csvFixture(t, cols))
			if label, ok, err := DetectCSV(csvPath); err != nil || ok {
				t.Errorf("%s minus %q: DetectCSV = %q, %v, %v", c.f.label, c.f.columns[i], label, ok, err)
			}
			jsonPath := writeFixture(t, "p3near.jsonl", jsonlFixture(t, cols, make([]string, len(cols))))
			if label, ok, err := DetectJSONL(jsonPath); err != nil || ok {
				t.Errorf("%s minus %q: DetectJSONL = %q, %v, %v", c.f.label, c.f.columns[i], label, ok, err)
			}
		}
	}
}

// TestPass3PacketDataAbsent checks the raw DHCP packet reaches no event field
// and no extra key.
func TestPass3PacketDataAbsent(t *testing.T) {
	events, _ := mapNetworkDHCP(networkDHCPFixture.rows[0])
	if len(events) != 1 {
		t.Fatalf("%d events, want 1", len(events))
	}
	e := events[0]
	if dump := fmt.Sprintf("%+v", *e); strings.Contains(dump, "deadbeef0102") {
		t.Errorf("packet data reached an event: %s", dump)
	}
	if strings.Contains(e.Extra, "PacketData") {
		t.Errorf("extra = %q, want no PacketData key", e.Extra)
	}
	if e.Desc != "en0 leased 192.168.1.20 from router 192.168.1.1 on SSID HomeNet" {
		t.Errorf("desc = %q", e.Desc)
	}
	if e.Type != networkDHCPType {
		t.Errorf("type = %q, want %q", e.Type, networkDHCPType)
	}
}

// TestSpotlightConfigTypes checks the readable types and the store UUID as
// record number.
func TestSpotlightConfigTypes(t *testing.T) {
	events, _ := mapSpotlightConfig(spotlightConfigFixture.rows[0])
	if len(events) != 2 {
		t.Fatalf("%d events, want 2", len(events))
	}
	if events[0].Type != spotlightConfigStoreCreated || events[1].Type != spotlightConfigConfigModified {
		t.Errorf("types = %q, %q", events[0].Type, events[1].Type)
	}
	for _, e := range events {
		if e.RecordNumber != "uuid-1" {
			t.Errorf("record number = %q, want uuid-1", e.RecordNumber)
		}
		if e.Desc != "Spotlight store uuid-1 at /System/Volumes/Data/" {
			t.Errorf("desc = %q", e.Desc)
		}
	}
}

func TestHasTimelineUntimedLabels(t *testing.T) {
	for _, label := range []string{networkDetailsLabel, networkInterfacesLabel, fileSharingLabel} {
		if HasTimeline(label) {
			t.Errorf("HasTimeline(%s) = true, want false", label)
		}
	}
	for _, label := range []string{networkDHCPLabel, spotlightConfigLabel, fsEventsLabel, spotlightStoreLabel} {
		if !HasTimeline(label) {
			t.Errorf("HasTimeline(%s) = false, want true", label)
		}
	}
}

// --- Spotlight store CSVs ---

var spotlightAColumns = []string{
	"ID", "ID_hex", "Flags", "Parent_ID", "Parent_ID_hex", "Date_Updated",
	"kMDItemDisplayName", "_kMDItemFileName", "kMDItemTitle", "kMDItemContentURL",
	"_kMDItemBundleID", "kMDItemContentCreationDate", "kMDItemLastUsedDate",
	"kMDItemDateAdded", "kMDItemStartDateHour", "kMDItemContentModificationDate",
	"_kMDItemExpirationDate", "kMDItemDueDates", "kMDItemEmbedding", "kMDItemFSName_Ranking",
}

var spotlightAName = "Spotlight-CoreSpotlight_cdtdelta-store.csv"

// spotlightARows. Expected events, by column:
//
//	row 1: Date_Updated                  (1 event)
//	row 1: kMDItemContentCreationDate    (1 event)
//	row 1: kMDItemDateAdded              (1 event)
//	row 1: kMDItemDueDates, two values   (2 events)
//	row 2: kMDItemDateAdded              (1 event)
//
// Row 1 leaves kMDItemLastUsedDate and kMDItemContentModificationDate empty.
// _kMDItemExpirationDate, kMDItemStartDateHour, and kMDItemFSName_Ranking
// produce nothing.
func spotlightARows() [][]string {
	r1 := map[string]string{
		"ID": "1", "ID_hex": "0x1", "Flags": "0", "Parent_ID": "0", "Parent_ID_hex": "0x0",
		"Date_Updated": "2026-03-04 05:06:07", "kMDItemDisplayName": "report.pdf",
		"kMDItemContentURL": "file:///Users/tom/report.pdf", "_kMDItemBundleID": "com.apple.Preview",
		"kMDItemContentCreationDate": "2026-03-01 00:00:00", "kMDItemDateAdded": "2026-03-02 00:00:00",
		"kMDItemStartDateHour": "2026-03-02 10:00:00", "_kMDItemExpirationDate": "2030-01-01 00:00:00",
		"kMDItemDueDates":  "2026-10-06 00:00:00, 2026-10-17 00:00:00",
		"kMDItemEmbedding": "0.1,0.2", "kMDItemFSName_Ranking": "7",
	}
	r2 := map[string]string{
		"ID": "2", "ID_hex": "0x2", "Flags": "0", "Parent_ID": "1", "Parent_ID_hex": "0x1",
		"_kMDItemFileName": "notes.txt", "kMDItemDateAdded": "2026-03-03 00:00:00",
	}
	return [][]string{fill(spotlightAColumns, r1), fill(spotlightAColumns, r2)}
}

var spotlightBColumns = []string{
	"ID", "ID_hex", "Flags", "Parent_ID", "Parent_ID_hex", "Date_Updated", "kMDItemUsedDates",
}

// spotlightBRows: one row, 3 events (Date_Updated plus two Dates values).
func spotlightBRows() [][]string {
	r := map[string]string{
		"ID": "9", "ID_hex": "0x9", "Flags": "0", "Parent_ID": "0", "Parent_ID_hex": "0x0",
		"Date_Updated":     "2026-04-02 00:00:00",
		"kMDItemUsedDates": "2026-04-01 00:00:00, 2026-04-03 00:00:00",
	}
	return [][]string{fill(spotlightBColumns, r)}
}

func TestSpotlightDetection(t *testing.T) {
	keys := spotlightAColumns[:6]
	header := append(append([]string{}, keys...), "kMDItemDisplayName")

	cases := []struct {
		name    string
		file    string
		columns []string
		want    string
	}{
		{"store with prefix and keys", "Spotlight-x.csv", header, spotlightStoreLabel},
		{"store with other prefix-free name", "x.csv", header, ""},
		{"prefix but wrong key order", "Spotlight-x.csv",
			[]string{"ID_hex", "ID", "Flags", "Parent_ID", "Parent_ID_hex", "Date_Updated", "kMDItemDisplayName"}, ""},
		{"prefix but a key missing", "Spotlight-x.csv",
			[]string{"ID", "ID_hex", "Flags", "Parent_ID", "Parent_ID_hex", "kMDItemDisplayName"}, ""},
		{"SpotlightConfig header in a Spotlight- file", "Spotlight-y.csv", spotlightConfigColumns, spotlightConfigLabel},
		{"SpotlightShortcuts header in a Spotlight- file", "Spotlight-z.csv", spotlightColumns, spotlightLabel},
	}
	for _, c := range cases {
		path := writeFixture(t, c.file, csvFixture(t, c.columns))
		label, ok, err := DetectCSV(path)
		if err != nil {
			t.Errorf("%s: DetectCSV error: %v", c.name, err)
			continue
		}
		if c.want == "" {
			if ok {
				t.Errorf("%s: detected as %q, want unrecognized", c.name, label)
			}
			continue
		}
		if !ok || label != c.want {
			t.Errorf("%s: DetectCSV = %q, %v; want %q", c.name, label, ok, c.want)
		}
	}
}

func TestSpotlightStoreEvents(t *testing.T) {
	pathA := writeFixture(t, spotlightAName, csvFixture(t, spotlightAColumns, spotlightARows()...))
	resA, err := ReadCSV(pathA, nil)
	if err != nil {
		t.Fatalf("ReadCSV(A): %v", err)
	}
	if len(resA.Events) != 6 {
		t.Fatalf("store A: %d events, want 6", len(resA.Events))
	}
	types := map[string]int{}
	for _, e := range resA.Events {
		types[e.Type]++
		if e.SourceType != "mac_apt Spotlight CoreSpotlight_cdtdelta-store" {
			t.Errorf("sourcetype = %q", e.SourceType)
		}
		if e.Filename != "" {
			t.Errorf("filename = %q, want empty", e.Filename)
		}
	}
	// See spotlightARows for the expected event per column.
	if types[spotlightRecordType] != 1 || types["kMDItemDueDates"] != 2 ||
		types["kMDItemContentCreationDate"] != 1 || types["kMDItemDateAdded"] != 2 {
		t.Errorf("event types = %v", types)
	}
	if types[spotlightExpiration] != 0 {
		t.Errorf("expiration produced %d events, want 0", types[spotlightExpiration])
	}

	pathB := writeFixture(t, "Spotlight-Other.csv", csvFixture(t, spotlightBColumns, spotlightBRows()...))
	resB, err := ReadCSV(pathB, nil)
	if err != nil {
		t.Fatalf("ReadCSV(B): %v", err)
	}
	if len(resB.Events) != 3 {
		t.Fatalf("store B: %d events, want 3", len(resB.Events))
	}
	if resB.Events[0].SourceType != "mac_apt Spotlight Other" {
		t.Errorf("store B sourcetype = %q", resB.Events[0].SourceType)
	}
	if resB.Events[0].Desc != "Spotlight item 9" {
		t.Errorf("store B desc = %q, want the ID fallback", resB.Events[0].Desc)
	}
}

func TestSpotlightDatesListValues(t *testing.T) {
	path := writeFixture(t, spotlightAName, csvFixture(t, spotlightAColumns, spotlightARows()...))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	var got []string
	for _, e := range res.Events {
		if e.Type == "kMDItemDueDates" {
			got = append(got, e.Datetime)
		}
	}
	want := []string{"2026-10-06 00:00:00", "2026-10-17 00:00:00"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Dates values = %v, want %v", got, want)
	}
}

// extraKeys returns the key names in an extra string, in order.
func extraKeys(extra string) []string {
	var keys []string
	for _, part := range strings.Split(extra, "; ") {
		if i := strings.Index(part, ": "); i >= 0 {
			keys = append(keys, part[:i])
		}
	}
	return keys
}

func TestSpotlightExtraExclusions(t *testing.T) {
	path := writeFixture(t, spotlightAName, csvFixture(t, spotlightAColumns, spotlightARows()...))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	for _, e := range res.Events {
		keys := extraKeys(e.Extra)
		has := map[string]bool{}
		for _, k := range keys {
			has[k] = true
		}
		for _, banned := range []string{
			"kMDItemFSName_Ranking", "kMDItemEmbedding", "kMDItemStartDateHour",
			"_kMDItemExpirationDate", "_kMDItemBundleID", "kMDItemContentURL",
			"kMDItemDisplayName", "ID",
		} {
			if has[banned] {
				t.Errorf("extra keys %v contain excluded %q", keys, banned)
			}
		}
		// Unmapped, non-timestamp columns stay in extra.
		if !has["Flags"] || !has["ID_hex"] {
			t.Errorf("extra keys %v, want Flags and ID_hex kept", keys)
		}
		// The producing timestamp leads extra; no other timestamp column follows it.
		if len(keys) == 0 || !isSpotlightTimestamp(keys[0]) {
			t.Fatalf("extra keys %v, want the producing timestamp first", keys)
		}
		for _, k := range keys[1:] {
			if isSpotlightTimestamp(k) {
				t.Errorf("extra keys %v contain non-producing timestamp %q", keys, k)
			}
		}
	}
	first := res.Events[0]
	if first.Desc != "report.pdf" || first.URL != "file:///Users/tom/report.pdf" || first.EventType != "com.apple.Preview" {
		t.Errorf("row 1 desc/url/event_type = %q/%q/%q", first.Desc, first.URL, first.EventType)
	}
	if !strings.HasPrefix(first.Extra, "Date_Updated: ") {
		t.Errorf("extra = %q, want the producing timestamp first", first.Extra)
	}
}

// TestSpotlightRankingNotTimestamp checks that a column ending in _Ranking is
// never an event, even when its name contains a date suffix, and that
// kMDItemDateAdded is one.
func TestSpotlightRankingNotTimestamp(t *testing.T) {
	if isSpotlightTimestamp("kMDItemDateAdded_Ranking") {
		t.Errorf("kMDItemDateAdded_Ranking selected as a timestamp")
	}
	if !isSpotlightTimestamp("kMDItemDateAdded") {
		t.Errorf("kMDItemDateAdded not selected as a timestamp")
	}

	header := []string{"ID", "ID_hex", "Flags", "Parent_ID", "Parent_ID_hex", "Date_Updated", "kMDItemDateAdded_Ranking"}
	r := map[string]string{
		"ID": "1", "ID_hex": "0x1", "Flags": "0", "Parent_ID": "0", "Parent_ID_hex": "0x0",
		"Date_Updated": "", "kMDItemDateAdded_Ranking": "2026-03-02 00:00:00",
	}
	events, skipped := newSpotlightStore(spotlightAName, header).mapRow(r)
	if len(events) != 0 || skipped != 0 {
		t.Errorf("events = %d, skipped = %d; want 0 and 0", len(events), skipped)
	}
}

// TestSpotlightUnparseableExampleNamesColumn checks that a garbage value in a
// store's own timestamp column is counted and described with that column and
// row. The store's columns are discovered per file, so the example comes from
// the store's cells, not timestampColumnsByLabel.
func TestSpotlightUnparseableExampleNamesColumn(t *testing.T) {
	r := map[string]string{
		"ID": "1", "ID_hex": "0x1", "Flags": "0", "Parent_ID": "0", "Parent_ID_hex": "0x0",
		"Date_Updated": "2026-03-04 05:06:07", "kMDItemLastUsedDate": "not-a-time",
		"kMDItemDisplayName": "report.pdf",
	}
	path := writeFixture(t, spotlightAName, csvFixture(t, spotlightAColumns, fill(spotlightAColumns, r)))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if res.UnparseableTimestamps != 1 {
		t.Errorf("unparseable = %d, want 1", res.UnparseableTimestamps)
	}
	want := []string{`row 2, column kMDItemLastUsedDate: "not-a-time"`}
	if !reflect.DeepEqual(res.UnparseableExamples, want) {
		t.Errorf("examples = %q, want %q", res.UnparseableExamples, want)
	}
}

// TestSpotlightNULInsideQuotedField checks a NUL inside a quoted field does not
// break parsing: the field keeps its comma and the row still imports.
func TestSpotlightNULInsideQuotedField(t *testing.T) {
	header := strings.Join(spotlightAColumns[:7], ",")
	row := `1,0x1,0,0,0x0,2026-03-04 05:06:07,"re` + "\x00" + `port, final.pdf"`
	content := header + "\n" + row + "\n"
	path := writeFixture(t, spotlightAName, content)
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if res.Excluded != 0 {
		t.Errorf("excluded = %d, want 0", res.Excluded)
	}
	if len(res.Events) != 1 {
		t.Fatalf("%d events, want 1", len(res.Events))
	}
	if res.Events[0].Desc != "report, final.pdf" {
		t.Errorf("desc = %q, want %q", res.Events[0].Desc, "report, final.pdf")
	}
}

// --- Unparseable examples ---

func TestUnparseableExamplesCSVRowsAndCap(t *testing.T) {
	cols := termSessionsColumns
	var rows [][]string
	for i := 0; i < 12; i++ {
		r := map[string]string{
			"Source_Type": "zsh", "Session_Start": "bad", "Session_Commands": "ls",
			"User": "tom", "Session_UUID": fmt.Sprint(i), "Source": "/x",
		}
		rows = append(rows, fill(cols, r))
	}
	path := writeFixture(t, "unparse.csv", csvFixture(t, cols, rows...))
	res, err := ReadCSV(path, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if res.UnparseableTimestamps != 12 {
		t.Errorf("unparseable = %d, want 12", res.UnparseableTimestamps)
	}
	if len(res.UnparseableExamples) != maxExamples {
		t.Fatalf("%d examples, want %d", len(res.UnparseableExamples), maxExamples)
	}
	if got, want := res.UnparseableExamples[0], `row 2, column Session_Start: "bad"`; got != want {
		t.Errorf("first example = %q, want %q", got, want)
	}
	if got, want := res.UnparseableExamples[9], `row 11, column Session_Start: "bad"`; got != want {
		t.Errorf("tenth example = %q, want %q", got, want)
	}
}

func TestUnparseableExamplesJSONLLineNumbers(t *testing.T) {
	cols := termSessionsColumns
	var rows [][]string
	for i := 0; i < 2; i++ {
		r := map[string]string{
			"Source_Type": "zsh", "Session_Start": "bad", "Session_Commands": "ls",
			"User": "tom", "Session_UUID": fmt.Sprint(i), "Source": "/x",
		}
		rows = append(rows, fill(cols, r))
	}
	path := writeFixture(t, "unparse.jsonl", jsonlFixture(t, cols, rows...))
	res, err := ReadJSONL(path, nil)
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}
	if len(res.UnparseableExamples) != 2 {
		t.Fatalf("%d examples, want 2", len(res.UnparseableExamples))
	}
	if got, want := res.UnparseableExamples[0], `row 1, column Session_Start: "bad"`; got != want {
		t.Errorf("first example = %q, want %q", got, want)
	}
	if got, want := res.UnparseableExamples[1], `row 2, column Session_Start: "bad"`; got != want {
		t.Errorf("second example = %q, want %q", got, want)
	}
}

func TestUnparseableExampleTruncated(t *testing.T) {
	long := strings.Repeat("x", 120)
	events, skipped := mapTermSessions(map[string]string{
		"Session_Start": long, "Session_Commands": "ls", "User": "tom", "Source": "/x",
	})
	if len(events) != 0 || skipped != 1 {
		t.Fatalf("events = %d, skipped = %d; want 0 and 1", len(events), skipped)
	}
	var r ReadResult
	r.addExamples(&output{label: termSessionsLabel}, map[string]string{"Session_Start": long}, 2)
	if len(r.UnparseableExamples) != 1 {
		t.Fatalf("%d examples, want 1", len(r.UnparseableExamples))
	}
	want := `row 2, column Session_Start: "` + strings.Repeat("x", 80) + `"`
	if r.UnparseableExamples[0] != want {
		t.Errorf("example = %q, want the value cut to 80 characters", r.UnparseableExamples[0])
	}
}

// TestTimestampColumnsByLabelMatchMappers checks that every column listed for
// describing unparseable values is one the mapper counts as skipped when it
// holds garbage. A column the mapper ignores would make the examples wrong.
func TestTimestampColumnsByLabelMatchMappers(t *testing.T) {
	for label, cols := range timestampColumnsByLabel {
		var o *output
		for _, cand := range outputs {
			if cand.label == label {
				o = cand
			}
		}
		if o == nil {
			t.Errorf("%s: no output registered", label)
			continue
		}
		row := map[string]string{}
		for _, c := range cols {
			row[c] = "not-a-time"
		}
		events, skipped := o.mapRow(row)
		if len(events) != 0 || skipped != len(cols) {
			t.Errorf("%s: %d events, %d skipped; want 0 and %d", label, len(events), skipped, len(cols))
		}
	}
}

// TestAllOutputLabelsPresent is the registry guard for every label the package
// registers: the 20 fixed timeline outputs, the three untimed outputs, and the
// Spotlight store. Add each new output's label here.
func TestAllOutputLabelsPresent(t *testing.T) {
	want := []string{
		aslLabel, biomeLabel, cfurlLabel, fsEventsLabel, installHistoryLabel,
		safariLabel, xprotectLabel, dockItemsLabel, sudoLastRunLabel,
		internetAccountsLabel, tccLabel, utmpxLabel, termSessionsLabel,
		quarantineLabel, cookiesLabel, recentItemsLabel, spotlightLabel, usersLabel,
		spotlightConfigLabel, networkDHCPLabel,
		networkDetailsLabel, networkInterfacesLabel, fileSharingLabel,
	}
	for _, label := range want {
		found := false
		for _, o := range outputs {
			if o.label == label {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not registered", label)
		}
	}
	// The Spotlight store has no entry in outputs, because its output is built
	// per file. Build one from the key columns to check its label.
	if got := spotlightStoreOutput("Spotlight-Test.csv", spotlightKeyColumns).label; got != spotlightStoreLabel {
		t.Errorf("Spotlight store output label = %q, want %q", got, spotlightStoreLabel)
	}
}
