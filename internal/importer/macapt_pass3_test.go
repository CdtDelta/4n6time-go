package importer

import (
	"strings"
	"testing"
)

const maNetworkDetailsHeader = "UUID,IPv4.ConfigMethod,IPv6.ConfigMethod,DeviceName,Hardware,Type,SubType," +
	"UserDefinedName,Proxies.ExceptionsList,SMB.NetBIOSName,SMB.Workgroup,PPP,Modem,Source"

const maNetworkDetailsRow = "u1,DHCP,,en0,,,,,,,,,,/x/net.plist"

// TestUntimedSingleFileImportsZeroEvents checks that a single-file import of an
// untimed mac_apt output succeeds with zero events, as an untimed EZ Tools file
// does.
func TestUntimedSingleFileImportsZeroEvents(t *testing.T) {
	dir := t.TempDir()
	path := writeTmpCSV(t, dir, "Network_Details.csv", maNetworkDetailsHeader+"\n"+maNetworkDetailsRow+"\n")

	format, label, err := DetectFile(path)
	if err != nil {
		t.Fatalf("DetectFile: %v", err)
	}
	if label != "mac_apt Network_Details" {
		t.Errorf("label = %q, want mac_apt Network_Details", label)
	}
	res, err := ImportFile(path, format, label, "", &mockStore{}, &Progress{}, nil)
	if err != nil {
		t.Fatalf("ImportFile: %v", err)
	}
	if res.Count != 0 {
		t.Errorf("Count = %d, want 0", res.Count)
	}
}

// TestUntimedRecursiveSkipped checks that the recursive walk lists an untimed
// output as skipped for having no timestamp columns, not as unrecognized.
func TestUntimedRecursiveSkipped(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "Network_Details.csv", maNetworkDetailsHeader+"\n"+maNetworkDetailsRow+"\n")

	store := &mockStore{}
	summary, err := ImportFolderRecursive(root, store, "", nil, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if summary.TotalFilesProcessed != 0 || store.insertedCount != 0 {
		t.Errorf("processed = %d, inserted = %d; want 0 and 0",
			summary.TotalFilesProcessed, store.insertedCount)
	}
	var found bool
	for _, sf := range summary.SkippedFiles {
		if sf.RelativePath == "Network_Details.csv" && strings.Contains(sf.Reason, "no timestamp columns") {
			found = true
		}
		if strings.Contains(sf.Reason, "unrecognized") {
			t.Errorf("untimed file reported as unrecognized: %q", sf.Reason)
		}
	}
	if !found {
		t.Errorf("want Network_Details.csv skipped with 'no timestamp columns'; got %v", summary.SkippedFiles)
	}
}

// TestUnparseableExamplesLoggedOnSingleFile checks that a single-file import
// logs its unparseable examples in one line, with the CSV row number.
func TestUnparseableExamplesLoggedOnSingleFile(t *testing.T) {
	dir := t.TempDir()
	const header = "Source_Type,Session_Start,Session_End,Session_Commands,All_Commands,User,Session_UUID,Source"
	path := writeTmpCSV(t, dir, "TermSessions.csv", header+"\nzsh,bad-time,,ls,ls,tom,u1,/x\n")

	var logs []string
	logf := func(level, msg string) { logs = append(logs, level+" "+msg) }

	format, label, err := DetectFile(path)
	if err != nil {
		t.Fatalf("DetectFile: %v", err)
	}
	if _, err := ImportFile(path, format, label, "", &mockStore{}, &Progress{}, logf); err != nil {
		t.Fatalf("ImportFile: %v", err)
	}
	const want = `INFO TermSessions.csv: unparseable timestamp examples: row 2, column Session_Start: "bad-time"`
	if !containsString(logs, want) {
		t.Errorf("logs = %v, want %q", logs, want)
	}
}
