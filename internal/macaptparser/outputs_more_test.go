package macaptparser

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Fixtures for the fourteen outputs added after the first four. Each has at
// least one row that exercises a timestamp, and the event counts in
// moreFixtures are the counts those rows must produce.

var installHistoryFixture = fixture{
	label:   installHistoryLabel,
	columns: installHistoryColumns,
	rows: []map[string]string{
		{
			"ContentType": "Software Update", "Date": "2026-03-04 05:06:07",
			"DisplayName": "Xcode", "DisplayVersion": "15.4",
			"PackageIdentifiers": "com.apple.pkg.Xcode", "ProcessName": "softwareupdated",
			"Source": "/Library/Receipts/InstallHistory.plist",
		},
		{
			"ContentType": "Software Update", "Date": "2026-03-05 10:00:00",
			"DisplayName": "Safari", "Source": "/Library/Receipts/InstallHistory.plist",
		},
	},
}

var safariFixture = fixture{
	label:   safariLabel,
	columns: safariColumns,
	rows: []map[string]string{
		{
			"Type": "History", "Name_or_Title": "Example", "URL": "https://example.com/",
			"Date": "2026-03-04 05:06:07", "Other_Info": "visits=3", "User": "tom",
			"Source": "/Users/tom/Library/Safari/History.db",
		},
		{
			"Type": "Download", "Name_or_Title": "", "URL": "", "Date": "2026-03-04 05:08:00",
			"Other_Info": "", "User": "-", "Source": "/Users/tom/Library/Safari/History.db",
		},
	},
}

var xprotectFixture = fixture{
	label:   xprotectLabel,
	columns: xprotectColumns,
	rows: []map[string]string{
		{
			"Id": "17", "Date": "2026-03-04 05:06:07", "Violated_Rule": "XProtect.Blocked",
			"Exec_Path": "/tmp/payload", "Exec_Cdhash": "abcd", "Exec_Signing_ID": "",
			"Exec_Team_ID": "", "Exec_Sha256": "ff00", "Exec_Is_Notarized": "0",
			"Responsible_Path": "/Applications/Browser.app", "Responsible_Cdhash": "",
			"Responsible_Signing_ID": "", "Responsible_Team_ID": "", "Responsible_Sha256": "",
			"Responsible_Is_Notarized": "1", "Reported": "1", "Profile_Hash": "p1",
			"Source": "/var/protected/xprotect.db",
		},
	},
}

var dockItemsFixture = fixture{
	label:   dockItemsLabel,
	columns: dockItemsColumns,
	rows: []map[string]string{
		{
			"File Label": "Terminal", "Parent Modified": "2026-03-01 00:00:00",
			"File Modified": "2026-03-02 00:00:00", "Recently Used": "Yes",
			"File Type": "1", "File Path": "/System/Applications/Utilities/Terminal.app",
			"GUID": "guid-1", "Bundle Identifier": "com.apple.Terminal", "User": "tom",
			"Source": "/Users/tom/Library/Preferences/com.apple.dock.plist",
		},
		{
			"File Label": "Notes", "File Modified": "2026-03-06 09:00:00",
			"File Path": "/Applications/Notes.app", "User": "N/A",
			"Source": "/Users/tom/Library/Preferences/com.apple.dock.plist",
		},
	},
}

var sudoLastRunFixture = fixture{
	label:   sudoLastRunLabel,
	columns: sudoLastRunColumns,
	rows: []map[string]string{
		{"Uid": "501", "Run Date": "2026-03-04 05:06:07", "User": "tom", "Source": "/var/db/sudo/ts/tom"},
		{"Uid": "0", "Run Date": "", "User": "N/A", "Source": "/var/db/sudo/ts/root"},
	},
}

var internetAccountsFixture = fixture{
	label:   internetAccountsLabel,
	columns: internetAccountsColumns,
	rows: []map[string]string{
		{
			"ID": "3", "Type": "iCloud", "Name": "Test Account", "Date": "2026-03-04 05:06:07",
			"UserDisplayName": "Test User", "Username": "testuser", "Parent_ID": "0",
			"Bundle": "com.apple.account", "UUID_or_AccountDSID": "dsid-9", "User": "tom",
			"Source": "/Users/tom/Library/Accounts/Accounts4.sqlite",
		},
	},
}

var tccFixture = fixture{
	label:   tccLabel,
	columns: tccColumns,
	rows: []map[string]string{
		{
			"Last_Modified": "2026-03-04 05:06:07", "Service": "kTCCServiceCamera",
			"Client": "com.example.app", "Client_Type": "0", "Allowed": "1",
			"Auth_Reason": "2", "Indirect_Object_Identifier": "UNUSED", "User": "tom",
			"Source": "/Library/Application Support/com.apple.TCC/TCC.db",
		},
		{
			"Last_Modified": "", "Service": "kTCCServiceMicrophone", "Client": "com.example.b",
			"User": "N/A", "Source": "/Library/Application Support/com.apple.TCC/TCC.db",
		},
	},
}

var utmpxFixture = fixture{
	label:   utmpxLabel,
	columns: utmpxColumns,
	rows: []map[string]string{
		{
			"User": "tom", "Terminal_ID": "0", "Terminal": "console", "PID": "123",
			"Type": "7", "Type_Name": "USER_PROCESS", "Date": "2026-03-04 05:06:07",
			"Hostname": "tom-mbp",
		},
		{
			"User": "-", "Terminal_ID": "0", "Terminal": "N/A", "PID": "0", "Type": "2",
			"Type_Name": "BOOT_TIME", "Date": "2026-03-04 05:00:00", "Hostname": "",
		},
	},
}

var termSessionsFixture = fixture{
	label:   termSessionsLabel,
	columns: termSessionsColumns,
	rows: []map[string]string{
		{
			"Source_Type": "zsh_history", "Session_Start": "2026-03-04 05:06:07",
			"Session_End": "2026-03-04 05:20:00", "Session_Commands": "ls\ncd x",
			"All_Commands": "ls\ncd x", "User": "tom", "Session_UUID": "uuid-1",
			"Source": "/Users/tom/.zsh_history",
		},
		{
			"Source_Type": "bash_history", "Session_Start": "2026-03-04 06:00:00",
			"Session_End": "", "Session_Commands": "make", "All_Commands": "make\nmake test",
			"User": "tom", "Session_UUID": "uuid-2", "Source": "/Users/tom/.bash_history",
		},
	},
}

var quarantineFixture = fixture{
	label:   quarantineLabel,
	columns: quarantineColumns,
	rows: []map[string]string{
		{
			"EventID": "ev1", "TimeStamp": "2026-03-04 05:06:07", "AgentBundleID": "com.apple.Safari",
			"AgentName": "Safari", "DataUrl": "https://example.com/file.zip", "SenderName": "",
			"SenderAddress": "", "TypeNumber": "0", "OriginTitle": "",
			"OriginUrl": "https://example.com/", "OriginAlias": "", "User": "tom",
			"Source": "/Users/tom/Library/Preferences/com.apple.LaunchServices.QuarantineEventsV2",
		},
		{
			"EventID": "ev2", "TimeStamp": "2026-03-04 05:07:00", "AgentName": "Mail",
			"User": "tom", "Source": "/Users/tom/Library/Preferences/com.apple.LaunchServices.QuarantineEventsV2",
		},
	},
}

var cookiesFixture = fixture{
	label:   cookiesLabel,
	columns: cookiesColumns,
	rows: []map[string]string{
		{
			"Host": "example.com", "Create Time": "2026-03-04 05:06:07",
			"Expiry Time": "2027-03-04 05:06:07", "Name": "sid", "Path": "/",
			"Value": "abc123", "User": "tom", "Source": "/Users/tom/Library/Cookies/Cookies.binarycookies",
		},
		{
			"Host": "example.org", "Create Time": "", "Expiry Time": "2027-01-01 00:00:00",
			"Name": "x", "Path": "/a", "Value": "", "User": "tom",
			"Source": "/Users/tom/Library/Cookies/Cookies.binarycookies",
		},
	},
}

var recentItemsFixture = fixture{
	label:   recentItemsLabel,
	columns: recentItemsColumns,
	rows: []map[string]string{
		{
			"Type": "Document", "Name": "report.pdf", "URL": "file:///Users/tom/report.pdf",
			"Info": "", "Date Last Seen": "2026-03-04 05:06:07",
			"Date Target Created": "2026-03-01 00:00:00", "Volume Name": "Macintosh HD",
			"Volume Size": "500000000000", "Volume UUID": "UUID-1",
			"Volume Creation Date": "2020-01-01 00:00:00", "User": "tom",
			"Source": "/Users/tom/Library/Preferences/com.apple.recentitems.plist",
		},
	},
}

var spotlightFixture = fixture{
	label:   spotlightLabel,
	columns: spotlightColumns,
	rows: []map[string]string{
		{
			"User": "tom", "UserTyped": "ter", "DisplayName": "Terminal",
			"LastUsed": "2026-03-04 05:06:07", "URL": "file:///Applications/Utilities/Terminal.app",
			"Identifier": "com.apple.Terminal", "Source": "/Users/tom/Library/Spotlight/shortcuts.plist",
		},
		{
			"User": "tom", "UserTyped": "old", "DisplayName": "Old", "LastUsed": "",
			"Source": "/Users/tom/Library/Spotlight/shortcuts.plist",
		},
	},
}

var usersFixture = fixture{
	label:   usersLabel,
	columns: usersColumns,
	rows: []map[string]string{
		{
			"Username": "tom", "Realname": "Tom", "Homedir": "/Users/tom", "UID": "501",
			"GID": "20", "UUID": "U-1", "CreationDate": "2020-01-01 00:00:00",
			"DeletedDate": "", "FailedLoginCount": "0", "FailedLoginTime": "",
			"LastLoginTime": "2026-03-04 05:06:07", "PasswordLastSetTime": "2026-02-01 00:00:00",
			"PasswordHint": "my hint", "Password": "SECRETHASH",
			"DARWIN_USER_DIR": "/var/folders/a", "DARWIN_USER_TEMP_DIR": "/var/folders/b",
			"DARWIN_USER_CACHE_DIR": "/var/folders/c",
			"Source":                "/private/var/db/dslocal/nodes/Default/users/tom.plist",
		},
	},
}

// moreFixtures pairs each new output's fixture with the number of events its
// rows must produce, one per non-empty timestamp that parses.
var moreFixtures = []struct {
	f      fixture
	events int
}{
	{installHistoryFixture, 2},
	{safariFixture, 2},
	{xprotectFixture, 1},
	{dockItemsFixture, 3},
	{sudoLastRunFixture, 1},
	{internetAccountsFixture, 1},
	{tccFixture, 1},
	{utmpxFixture, 2},
	{termSessionsFixture, 3},
	{quarantineFixture, 2},
	{cookiesFixture, 3},
	{recentItemsFixture, 3},
	{spotlightFixture, 1},
	{usersFixture, 3},
}

func TestMoreDetectCSVEachOutput(t *testing.T) {
	for _, c := range moreFixtures {
		path := writeFixture(t, "out.csv", csvFixture(t, c.f.columns, c.f.csvRows()...))
		label, ok, err := DetectCSV(path)
		if err != nil || !ok || label != c.f.label {
			t.Errorf("DetectCSV(%s) = %q, %v, %v; want %q", c.f.label, label, ok, err, c.f.label)
		}
	}
}

func TestMoreDetectJSONLEachOutput(t *testing.T) {
	for _, c := range moreFixtures {
		path := writeFixture(t, "out.jsonl", jsonlFixture(t, c.f.columns, c.f.csvRows()...))
		label, ok, err := DetectJSONL(path)
		if err != nil || !ok || label != c.f.label {
			t.Errorf("DetectJSONL(%s) = %q, %v, %v; want %q", c.f.label, label, ok, err, c.f.label)
		}
	}
}

// TestMoreRoundTripCSVAndJSONL checks that the same rows produce identical
// events from CSV and JSONL, and that each output yields its expected count.
func TestMoreRoundTripCSVAndJSONL(t *testing.T) {
	for _, c := range moreFixtures {
		csvPath := writeFixture(t, "rt.csv", csvFixture(t, c.f.columns, c.f.csvRows()...))
		jsonlPath := writeFixture(t, "rt.jsonl", jsonlFixture(t, c.f.columns, c.f.csvRows()...))

		fromCSV, err := ReadCSV(csvPath, nil)
		if err != nil {
			t.Fatalf("%s: ReadCSV: %v", c.f.label, err)
		}
		fromJSON, err := ReadJSONL(jsonlPath, nil)
		if err != nil {
			t.Fatalf("%s: ReadJSONL: %v", c.f.label, err)
		}
		if len(fromCSV.Events) != c.events {
			t.Errorf("%s: %d events from CSV, want %d", c.f.label, len(fromCSV.Events), c.events)
		}
		if !reflect.DeepEqual(fromCSV.Events, fromJSON.Events) {
			t.Errorf("%s: CSV and JSONL events differ\nCSV:  %+v\nJSON: %+v",
				c.f.label, derefEvents(fromCSV.Events), derefEvents(fromJSON.Events))
		}
	}
}

// TestMoreNearMissHeadersRejected checks that a header or key set missing one
// column from a new output is not detected as any output.
func TestMoreNearMissHeadersRejected(t *testing.T) {
	for _, c := range moreFixtures {
		for i := range c.f.columns {
			cols := append(append([]string{}, c.f.columns[:i]...), c.f.columns[i+1:]...)

			csvPath := writeFixture(t, "near.csv", csvFixture(t, cols))
			if label, ok, err := DetectCSV(csvPath); err != nil || ok {
				t.Errorf("%s minus %q: DetectCSV = %q, %v, %v; want not detected",
					c.f.label, c.f.columns[i], label, ok, err)
			}

			jsonPath := writeFixture(t, "near.jsonl", jsonlFixture(t, cols, make([]string, len(cols))))
			if label, ok, err := DetectJSONL(jsonPath); err != nil || ok {
				t.Errorf("%s minus %q: DetectJSONL = %q, %v, %v; want not detected",
					c.f.label, c.f.columns[i], label, ok, err)
			}
		}
	}
}

// TestExistingOutputsStillRegistered confirms the first four outputs keep
// their label, column list, and detection after the registry grew.
func TestExistingOutputsStillRegistered(t *testing.T) {
	cases := []struct {
		label   string
		columns []string
	}{
		{fsEventsLabel, fsEventsColumns},
		{biomeLabel, biomeColumns},
		{aslLabel, aslColumns},
		{cfurlLabel, cfurlColumns},
	}
	for _, c := range cases {
		o := outputForHeader(c.columns)
		if o == nil || o.label != c.label {
			t.Errorf("outputForHeader(%s) = %v; want label %q", c.label, o, c.label)
		}
	}
}

// TestMoreUserPlaceholders checks that "-" and "N/A" become an empty user and
// that real user values pass through.
func TestMoreUserPlaceholders(t *testing.T) {
	cases := []struct {
		name   string
		mapRow rowMapper
		row    map[string]string
		want   string
	}{
		{"Safari dash", mapSafari, map[string]string{"Date": "2026-03-04 05:06:07", "User": "-"}, ""},
		{"Safari user", mapSafari, map[string]string{"Date": "2026-03-04 05:06:07", "User": "tom"}, "tom"},
		{"TCC N/A", mapTCC, map[string]string{"Last_Modified": "2026-03-04 05:06:07", "User": "N/A"}, ""},
		{"Utmpx dash", mapUtmpx, map[string]string{"Date": "2026-03-04 05:06:07", "User": "-"}, ""},
		{"SudoLastRun N/A", mapSudoLastRun, map[string]string{"Run Date": "2026-03-04 05:06:07", "User": "N/A"}, ""},
		{"Dock dash", mapDockItems, map[string]string{"File Modified": "2026-03-04 05:06:07", "User": "-"}, ""},
		{"Cookies N/A", mapCookies, map[string]string{"Create Time": "2026-03-04 05:06:07", "User": "N/A"}, ""},
		{"TermSessions dash", mapTermSessions, map[string]string{"Session_Start": "2026-03-04 05:06:07", "User": "-"}, ""},
		{"Quarantine N/A", mapQuarantine, map[string]string{"TimeStamp": "2026-03-04 05:06:07", "User": "N/A"}, ""},
		{"InternetAccounts dash", mapInternetAccounts, map[string]string{"Date": "2026-03-04 05:06:07", "User": "-"}, ""},
		{"RecentItems N/A", mapRecentItems, map[string]string{"Date Last Seen": "2026-03-04 05:06:07", "User": "N/A"}, ""},
		{"Spotlight dash", mapSpotlightShortcuts, map[string]string{"LastUsed": "2026-03-04 05:06:07", "User": "-"}, ""},
		{"Users dash", mapUsers, map[string]string{"CreationDate": "2026-03-04 05:06:07", "Username": "-"}, ""},
		{"Users user", mapUsers, map[string]string{"CreationDate": "2026-03-04 05:06:07", "Username": "tom"}, "tom"},
	}
	for _, c := range cases {
		events, _ := c.mapRow(c.row)
		if len(events) == 0 {
			t.Errorf("%s: no events", c.name)
			continue
		}
		for _, e := range events {
			if e.User != c.want {
				t.Errorf("%s: user = %q, want %q", c.name, e.User, c.want)
			}
		}
	}
}

// TestUsersPasswordNeverMapped checks that the Password column reaches no
// event field, extra included. PasswordHint is a different column and stays.
func TestUsersPasswordNeverMapped(t *testing.T) {
	const secret = "SECRET-PW-HASH-VALUE"
	row := map[string]string{}
	for k, v := range usersFixture.rows[0] {
		row[k] = v
	}
	row["Password"] = secret

	events, _ := mapUsers(row)
	if len(events) == 0 {
		t.Fatal("no events from Users row")
	}
	for _, e := range events {
		if dump := fmt.Sprintf("%+v", *e); strings.Contains(dump, secret) {
			t.Errorf("Password value reached an event: %s", dump)
		}
		if strings.Contains(e.Extra, "Password:") {
			t.Errorf("extra has a Password key: %q", e.Extra)
		}
	}
	if !strings.Contains(events[0].Extra, "PasswordHint: my hint") {
		t.Errorf("extra = %q, want PasswordHint kept", events[0].Extra)
	}
}

// TestTermSessionsAllCommands checks that All_Commands appears in extra only
// when it differs from Session_Commands, and that line breaks in commands
// become " ; " in the description.
func TestTermSessionsAllCommands(t *testing.T) {
	same := map[string]string{
		"Source_Type": "zsh", "Session_Start": "2026-03-04 05:06:07",
		"Session_Commands": "ls\ncd x", "All_Commands": "ls\ncd x",
		"User": "tom", "Session_UUID": "u1", "Source": "/x",
	}
	events, _ := mapTermSessions(same)
	if len(events) != 1 {
		t.Fatalf("same: %d events, want 1", len(events))
	}
	if strings.Contains(events[0].Extra, "All_Commands") {
		t.Errorf("same: extra = %q, want no All_Commands", events[0].Extra)
	}
	if events[0].Desc != "ls ; cd x" {
		t.Errorf("same: desc = %q, want %q", events[0].Desc, "ls ; cd x")
	}

	diff := map[string]string{
		"Source_Type": "bash", "Session_Start": "2026-03-04 05:06:07",
		"Session_Commands": "make", "All_Commands": "make\nmake test",
		"User": "tom", "Session_UUID": "u2", "Source": "/x",
	}
	events, _ = mapTermSessions(diff)
	if len(events) != 1 {
		t.Fatalf("diff: %d events, want 1", len(events))
	}
	if !strings.Contains(events[0].Extra, "All_Commands: make make test") {
		t.Errorf("diff: extra = %q, want All_Commands with its line breaks flattened", events[0].Extra)
	}
}

// TestQuarantineURLFallback checks the URL and description fallbacks when
// DataUrl is empty or both URLs are empty.
func TestQuarantineURLFallback(t *testing.T) {
	originOnly := map[string]string{
		"TimeStamp": "2026-03-04 05:06:07", "AgentName": "Safari",
		"OriginUrl": "https://origin.example/", "EventID": "e1", "Source": "/x",
	}
	events, _ := mapQuarantine(originOnly)
	if len(events) != 1 {
		t.Fatalf("origin only: %d events, want 1", len(events))
	}
	e := events[0]
	if e.URL != "https://origin.example/" {
		t.Errorf("origin only: URL = %q, want the origin URL", e.URL)
	}
	if e.Desc != "Safari downloaded https://origin.example/" {
		t.Errorf("origin only: desc = %q", e.Desc)
	}
	if strings.Contains(e.Extra, "OriginUrl") {
		t.Errorf("origin only: extra = %q, want OriginUrl used, not repeated", e.Extra)
	}

	none := map[string]string{"TimeStamp": "2026-03-04 05:06:07", "AgentName": "Safari", "EventID": "e2", "Source": "/x"}
	events, _ = mapQuarantine(none)
	if len(events) != 1 {
		t.Fatalf("no URLs: %d events, want 1", len(events))
	}
	if events[0].URL != "" || events[0].Desc != "Safari quarantine event" {
		t.Errorf("no URLs: URL = %q, desc = %q", events[0].URL, events[0].Desc)
	}

	both := map[string]string{
		"TimeStamp": "2026-03-04 05:06:07", "AgentName": "Safari",
		"DataUrl": "https://data.example/f", "OriginUrl": "https://origin.example/", "EventID": "e3",
	}
	events, _ = mapQuarantine(both)
	if len(events) != 1 {
		t.Fatalf("both URLs: %d events, want 1", len(events))
	}
	if events[0].URL != "https://data.example/f" {
		t.Errorf("both URLs: URL = %q, want DataUrl", events[0].URL)
	}
	if !strings.Contains(events[0].Extra, "OriginUrl: https://origin.example/") {
		t.Errorf("both URLs: extra = %q, want the unused OriginUrl kept", events[0].Extra)
	}
}

// TestUtmpxNoSource checks that Utmpx, which has no Source column, leaves the
// filename empty and still maps its other fields.
func TestUtmpxNoSource(t *testing.T) {
	csvPath := writeFixture(t, "utmpx.csv", csvFixture(t, utmpxColumns, fill(utmpxColumns, utmpxFixture.rows[0])))
	result, err := ReadCSV(csvPath, nil)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("%d events, want 1", len(result.Events))
	}
	e := result.Events[0]
	if e.Filename != "" {
		t.Errorf("filename = %q, want empty", e.Filename)
	}
	if e.Source != utmpxSource || e.SourceType != utmpxLabel {
		t.Errorf("source/sourcetype = %q/%q", e.Source, e.SourceType)
	}
	if e.Type != "USER_PROCESS" || e.Host != "tom-mbp" || e.User != "tom" {
		t.Errorf("type/host/user = %q/%q/%q", e.Type, e.Host, e.User)
	}
	if e.Desc != "USER_PROCESS tom console" {
		t.Errorf("desc = %q, want %q", e.Desc, "USER_PROCESS tom console")
	}
}

// TestUtmpxOmitsNABeforeDesc checks that N/A parts drop out of the Utmpx
// description rather than appearing as text.
func TestUtmpxOmitsNABeforeDesc(t *testing.T) {
	events, _ := mapUtmpx(utmpxFixture.rows[1])
	if len(events) != 1 {
		t.Fatalf("%d events, want 1", len(events))
	}
	if events[0].Desc != "BOOT_TIME" {
		t.Errorf("desc = %q, want %q", events[0].Desc, "BOOT_TIME")
	}
}

// TestDockItemsRecentlyUsedIsFlag checks that Recently Used is a Yes/empty
// flag, not a timestamp: a Yes value is never counted as unparseable, and it
// appears in extra instead of producing an event.
func TestDockItemsRecentlyUsedIsFlag(t *testing.T) {
	row := map[string]string{
		"File Label": "Terminal", "Parent Modified": "2026-03-01 00:00:00",
		"File Modified": "2026-03-02 00:00:00", "Recently Used": "Yes",
		"File Path": "/System/Applications/Utilities/Terminal.app", "User": "tom",
		"Source": "/Users/tom/Library/Preferences/com.apple.dock.plist",
	}
	events, skipped := mapDockItems(row)
	if len(events) != 2 {
		t.Fatalf("%d events, want 2", len(events))
	}
	if skipped != 0 {
		t.Errorf("unparseable = %d, want 0", skipped)
	}
	for _, e := range events {
		if !strings.Contains(e.Extra, "Recently Used: Yes") {
			t.Errorf("extra = %q, want Recently Used: Yes", e.Extra)
		}
		if e.Type == "Recently Used" {
			t.Errorf("event type = %q, want a date column only", e.Type)
		}
	}
}

// TestMoreUnparseableCountedNotFatal checks that a bad timestamp in one column
// is counted and skipped while the good column still produces its event.
func TestMoreUnparseableCountedNotFatal(t *testing.T) {
	row := map[string]string{
		"Source_Type": "zsh", "Session_Start": "not-a-time",
		"Session_End": "2026-03-04 05:20:00", "Session_Commands": "x",
		"User": "tom", "Session_UUID": "u", "Source": "/x",
	}
	events, skipped := mapTermSessions(row)
	if len(events) != 1 || skipped != 1 {
		t.Errorf("events = %d, skipped = %d; want 1 and 1", len(events), skipped)
	}
}
