package macaptparser

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// maxExamples caps the unparseable-value examples kept per file. The log line
// shows them all, so the cap keeps a badly broken file from flooding the log.
const maxExamples = 10

// maxExampleValue is the longest value shown in an example, in characters.
const maxExampleValue = 80

// timeCell is one non-empty timestamp value in a row and the column it is in.
type timeCell struct {
	column string
	value  string
}

// timestampColumnsByLabel lists the timestamp columns each fixed-schema output
// parses. Used only to describe unparseable values; the mappers decide what
// becomes an event. A test checks that each listed column is counted as
// skipped when it holds garbage.
var timestampColumnsByLabel = map[string][]string{
	fsEventsLabel:         {"SourceModDate"},
	biomeLabel:            {"Timestamp1", "Timestamp2"},
	aslLabel:              {"Timestamp"},
	cfurlLabel:            {"Date"},
	installHistoryLabel:   {"Date"},
	safariLabel:           {"Date"},
	xprotectLabel:         {"Date"},
	dockItemsLabel:        {"Parent Modified", "File Modified"},
	sudoLastRunLabel:      {"Run Date"},
	internetAccountsLabel: {"Date"},
	tccLabel:              {"Last_Modified"},
	utmpxLabel:            {"Date"},
	termSessionsLabel:     {"Session_Start", "Session_End"},
	quarantineLabel:       {"TimeStamp"},
	cookiesLabel:          {"Create Time", "Expiry Time"},
	recentItemsLabel:      {"Date Last Seen", "Date Target Created", "Volume Creation Date"},
	spotlightLabel:        {"LastUsed"},
	usersLabel:            {"CreationDate", "DeletedDate", "FailedLoginTime", "LastLoginTime", "PasswordLastSetTime"},
	spotlightConfigLabel:  {"StoreCreationDate", "ConfigurationModificationDate"},
	networkDHCPLabel:      {"LeaseStartDate"},
}

// cellsFor returns the non-empty timestamp cells of row for output o.
func cellsFor(o *output, row map[string]string) []timeCell {
	if o.cells != nil {
		return o.cells(row)
	}
	var cells []timeCell
	for _, col := range timestampColumnsByLabel[o.label] {
		if v := strings.TrimSpace(row[col]); v != "" {
			cells = append(cells, timeCell{column: col, value: v})
		}
	}
	return cells
}

// addExamples records the unparseable cells of one row, up to maxExamples in
// total for the file. It runs only on rows the mapper reported as skipped.
func (r *ReadResult) addExamples(o *output, row map[string]string, rowNum int) {
	for _, c := range cellsFor(o, row) {
		if len(r.UnparseableExamples) >= maxExamples {
			return
		}
		if _, ok := parseTimestamp(c.value); ok {
			continue
		}
		r.UnparseableExamples = append(r.UnparseableExamples,
			fmt.Sprintf(`row %d, column %s: "%s"`, rowNum, c.column, shortValue(c.value)))
	}
}

// shortValue flattens line breaks and cuts a value to maxExampleValue
// characters, so one example fits on one log line.
func shortValue(s string) string {
	s = lineBreakRun.ReplaceAllString(s, " ")
	if utf8.RuneCountInString(s) > maxExampleValue {
		s = string([]rune(s)[:maxExampleValue])
	}
	return s
}

// untimedLabels are recognized outputs that carry no timestamp columns. They
// produce no events, so recursive import skips them with an explanation.
var untimedLabels = map[string]bool{
	networkDetailsLabel:    true,
	networkInterfacesLabel: true,
	fileSharingLabel:       true,
}

// HasTimeline reports whether a detected label produces timeline events. The
// importer's recursive walk skips labels for which it returns false.
func HasTimeline(label string) bool {
	return !untimedLabels[label]
}
