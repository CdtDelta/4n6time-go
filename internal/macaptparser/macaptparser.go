// Package macaptparser imports CSV and JSONL output from ydkhatri's mac_apt
// (tested against v1.36.2). Each plugin writes one file per output, and the
// file's header (CSV) or key set (JSONL) identifies which output produced it.
// The outputs are listed in outputs, and the mappers for the later outputs
// live in outputs_more.go.
package macaptparser

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/textutil"
)

// Label values, returned by detection and stored as sourcetype on every
// event from that output.
const (
	fsEventsLabel = "mac_apt FsEvents"
	biomeLabel    = "mac_apt Biome"
	aslLabel      = "mac_apt asl_ver2"
	cfurlLabel    = "mac_apt CFURL_Cache"
)

// Source values, uppercase plugin names stored in the source column.
const (
	fsEventsSource = "FSEVENTS"
	biomeSource    = "BIOME"
	aslSource      = "ASL"
	cfurlSource    = "CFURLCACHE"
)

const (
	// timezoneUTC is written to every event. mac_apt writes UTC timestamps.
	timezoneUTC = "UTC"

	// macb is the generic event-time marker the other parsers use, since
	// these rows carry no file MACB timestamps.
	macb = "...."

	// utcOffset is the only offset accepted after a mac_apt timestamp. ASL
	// writes it; the other outputs write none. Both mean UTC.
	utcOffset = "+00:00"

	// datetimeLayout matches the "YYYY-MM-DD HH:MM:SS" UTC format the other
	// parsers store; fractional seconds are dropped.
	datetimeLayout = "2006-01-02 15:04:05"

	// maxLineSize caps a single JSONL line, as unifiedlogparser does.
	maxLineSize = 64 * 1024 * 1024

	progressInterval = 10000

	// fsEventsType is the type for FsEvents rows. SourceModDate is when the
	// fseventsd log file was last modified, not when the recorded events
	// happened, so the type says so.
	fsEventsType = "FSEvents log modified (approximate)"

	cfurlType = "HTTP request cached"
)

// Column lists from mac_apt v1.36.2. Detection compares them exactly, so
// the spaces in Biome's names are part of the match.
var (
	fsEventsColumns = []string{
		"LogID", "EventFlagsHex", "EventType", "EventFlags", "Filepath",
		"File_ID", "Log_Unknown", "SourceModDate", "Source",
	}
	biomeColumns = []string{
		"Biome Type", "Filename timestamp", "Offset", "State", "Data",
		"Timestamp1", "Timestamp2", "User", "Source",
	}
	aslColumns = []string{
		"Timestamp", "Nano", "ID", "Flags", "Host", "Sender", "Facility",
		"Level", "PID", "UID", "GID", "RefProc", "RefPID", "Message",
		"Session", "Key_Value", "Source",
	}
	cfurlColumns = []string{
		"Date", "URL", "Method", "Request_Header", "HTTP_Status",
		"Response_Header", "isDataOnFS", "Received_Data", "User",
		"App_Bundle_ID", "Source",
	}
)

// ErrNotMacApt is returned by the Read functions when the file's header or
// keys do not match any supported output. Detection reports this as
// unrecognized rather than as an error.
var ErrNotMacApt = errors.New("not a supported mac_apt output")

// ReadResult contains the outcome of a mac_apt import.
type ReadResult struct {
	Events []*model.Event
	Count  int
	// Excluded counts rows skipped because they could not be decoded.
	Excluded int
	// UnparseableTimestamps counts non-empty timestamp values that did not
	// parse under any accepted layout. Each one produces no event; the rest of
	// its row still imports. Empty timestamp values are normal (Biome's
	// Timestamp2 is usually empty) and are not counted.
	UnparseableTimestamps int
	// UnparseableExamples describes up to maxExamples unparseable values, as
	// `row <n>, column <name>: "<value>"`. Row 1 is the CSV header; for JSONL
	// the number is the line.
	UnparseableExamples []string
}

// rowMapper turns one row, keyed by column name, into zero or more events.
// It also reports how many non-empty timestamp values it could not parse.
type rowMapper func(row map[string]string) (events []*model.Event, skipped int)

// output describes one supported mac_apt plugin output.
type output struct {
	label   string
	columns []string
	mapRow  rowMapper
	// stripNUL removes NUL bytes before CSV splitting. Only outputs whose
	// files carry NULs inside quoted fields set it, so their quoting survives.
	stripNUL bool
	// cells lists a row's timestamp values for the unparseable examples. Nil
	// means the output's columns come from timestampColumnsByLabel.
	cells func(row map[string]string) []timeCell
}

// outputs lists every supported output. Mapping functions must not refer to
// this slice, or package initialization reports a cycle.
var outputs = []*output{
	{label: fsEventsLabel, columns: fsEventsColumns, mapRow: mapFsEvents},
	{label: biomeLabel, columns: biomeColumns, mapRow: mapBiome},
	{label: aslLabel, columns: aslColumns, mapRow: mapASL},
	{label: cfurlLabel, columns: cfurlColumns, mapRow: mapCFURLCache},
	{label: installHistoryLabel, columns: installHistoryColumns, mapRow: mapInstallHistory},
	{label: safariLabel, columns: safariColumns, mapRow: mapSafari},
	{label: xprotectLabel, columns: xprotectColumns, mapRow: mapXProtectBehavior},
	{label: dockItemsLabel, columns: dockItemsColumns, mapRow: mapDockItems},
	{label: sudoLastRunLabel, columns: sudoLastRunColumns, mapRow: mapSudoLastRun},
	{label: internetAccountsLabel, columns: internetAccountsColumns, mapRow: mapInternetAccounts},
	{label: tccLabel, columns: tccColumns, mapRow: mapTCC},
	{label: utmpxLabel, columns: utmpxColumns, mapRow: mapUtmpx},
	{label: termSessionsLabel, columns: termSessionsColumns, mapRow: mapTermSessions},
	{label: quarantineLabel, columns: quarantineColumns, mapRow: mapQuarantine},
	{label: cookiesLabel, columns: cookiesColumns, mapRow: mapCookies},
	{label: recentItemsLabel, columns: recentItemsColumns, mapRow: mapRecentItems},
	{label: spotlightLabel, columns: spotlightColumns, mapRow: mapSpotlightShortcuts},
	{label: usersLabel, columns: usersColumns, mapRow: mapUsers},
	{label: spotlightConfigLabel, columns: spotlightConfigColumns, mapRow: mapSpotlightConfig},
	{label: networkDHCPLabel, columns: networkDHCPColumns, mapRow: mapNetworkDHCP},
	{label: networkDetailsLabel, columns: networkDetailsColumns, mapRow: mapNetworkDetails},
	{label: networkInterfacesLabel, columns: networkInterfacesColumns, mapRow: mapNetworkInterfaces},
	{label: fileSharingLabel, columns: fileSharingColumns, mapRow: mapFileSharing},
}

// newCSVReader returns a reader configured for mac_apt CSV. Fields can span
// lines inside quotes (ASL Message, CFURL headers), so LazyQuotes is on and
// rows of differing width are accepted.
func newCSVReader(r io.Reader) *csv.Reader {
	reader := csv.NewReader(textutil.NewBOMStrippingReader(r))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	return reader
}

// newLineScanner returns a scanner over r with a line limit large enough for
// the largest JSONL record.
func newLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(textutil.NewBOMStrippingReader(r))
	scanner.Buffer(make([]byte, 0, 1024*1024), maxLineSize)
	return scanner
}

// outputForHeader returns the output whose columns match header exactly, in
// the same order, or nil.
func outputForHeader(header []string) *output {
	for _, o := range outputs {
		if equalStrings(header, o.columns) {
			return o
		}
	}
	return nil
}

// outputForKeys returns the output whose column list equals keys as a set,
// or nil. JSON object keys are unique, so equal length plus containment is
// set equality.
func outputForKeys(keys []string) *output {
	for _, o := range outputs {
		if len(keys) != len(o.columns) {
			continue
		}
		cols := make(map[string]struct{}, len(o.columns))
		for _, c := range o.columns {
			cols[c] = struct{}{}
		}
		matched := true
		for _, k := range keys {
			if _, ok := cols[k]; !ok {
				matched = false
				break
			}
		}
		if matched {
			return o
		}
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// DetectCSV reports whether path is mac_apt CSV output for a supported
// plugin, based on its header row alone. label is the sourcetype, such as
// "mac_apt FsEvents".
func DetectCSV(path string) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	header, err := newCSVReader(f).Read()
	if err != nil {
		return "", false, fmt.Errorf("reading header: %w", err)
	}
	o := outputForFile(path, header)
	if o == nil {
		return "", false, nil
	}
	return o.label, true, nil
}

// DetectJSONL reports whether path is mac_apt JSONL output for a supported
// plugin, based on the key set of its first non-empty line. A first line
// that is not a JSON object returns an error.
func DetectJSONL(path string) (string, bool, error) {
	o, err := jsonlOutput(path)
	if err != nil {
		return "", false, err
	}
	if o == nil {
		return "", false, nil
	}
	return o.label, true, nil
}

// jsonlOutput reads the first non-empty line of path and returns the output
// its keys identify, or nil when none match.
func jsonlOutput(path string) (*output, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	scanner := newLineScanner(f)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(line, &obj); err != nil {
			return nil, fmt.Errorf("first line is not a JSON object: %w", err)
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		return outputForKeys(keys), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	return nil, fmt.Errorf("empty file")
}

// ReadCSV parses a mac_apt CSV file. onProgress, if non-nil, is called about
// every 10,000 events.
func ReadCSV(path string, onProgress func(int)) (*ReadResult, error) {
	o, err := csvOutput(path)
	if err != nil {
		return nil, err
	}
	return readFile(path, o, eachCSVRow, onProgress)
}

// ReadJSONL parses a mac_apt JSONL file. onProgress, if non-nil, is called
// about every 10,000 events.
func ReadJSONL(path string, onProgress func(int)) (*ReadResult, error) {
	o, err := jsonlOutput(path)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, ErrNotMacApt
	}
	return readFile(path, o, eachJSONLRow, onProgress)
}

// csvOutput returns the output a CSV file's header identifies.
func csvOutput(path string) (*output, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	header, err := newCSVReader(f).Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	o := outputForFile(path, header)
	if o == nil {
		return nil, ErrNotMacApt
	}
	return o, nil
}

// rowIterator calls fn for each row of path that decodes, keyed by the
// output's column names. It returns how many rows failed to decode.
type rowIterator func(path string, o *output, fn func(row map[string]string, rowNum int)) (int, error)

// readFile maps every row through the output's mapper and collects events.
func readFile(path string, o *output, each rowIterator, onProgress func(int)) (*ReadResult, error) {
	result := &ReadResult{}
	nextProgress := progressInterval
	excluded, err := each(path, o, func(row map[string]string, rowNum int) {
		events, skipped := o.mapRow(row)
		result.UnparseableTimestamps += skipped
		if skipped > 0 {
			result.addExamples(o, row, rowNum)
		}
		result.Events = append(result.Events, events...)
		result.Count += len(events)
		if onProgress != nil && result.Count >= nextProgress {
			onProgress(result.Count)
			nextProgress += progressInterval
		}
	})
	result.Excluded = excluded
	if err != nil {
		return nil, err
	}
	return result, nil
}

// eachCSVRow streams the data rows of a mac_apt CSV file. The header must
// match the output's columns exactly.
func eachCSVRow(path string, o *output, fn func(row map[string]string, rowNum int)) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// NUL removal sits below the CSV parser so it cannot unbalance quoting.
	var src io.Reader = f
	if o.stripNUL {
		src = nulStrippingReader{r: f}
	}
	reader := newCSVReader(src)
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("reading header: %w", err)
	}
	if !equalStrings(header, o.columns) {
		return 0, ErrNotMacApt
	}

	excluded := 0
	rowNum := 1 // the header is row 1
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			excluded++
			continue
		}
		fn(rowFromFields(o.columns, fields), rowNum)
	}
	return excluded, nil
}

// rowFromFields pairs column names with field values. Missing trailing
// fields become empty strings.
func rowFromFields(columns, fields []string) map[string]string {
	row := make(map[string]string, len(columns))
	for i, col := range columns {
		v := ""
		if i < len(fields) {
			v = fields[i]
		}
		row[col] = stripNUL(v)
	}
	return row
}

// eachJSONLRow streams the lines of a mac_apt JSONL file.
func eachJSONLRow(path string, o *output, fn func(row map[string]string, rowNum int)) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	scanner := newLineScanner(f)
	excluded := 0
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		row, err := decodeJSONLRow(line)
		if err != nil {
			excluded++
			continue
		}
		fn(row, lineNum)
	}
	if err := scanner.Err(); err != nil {
		return excluded, fmt.Errorf("reading file at line %d: %w", lineNum+1, err)
	}
	return excluded, nil
}

// decodeJSONLRow decodes one JSONL object into string values. UseNumber
// keeps integers exact, so a number becomes its decimal text and matches the
// CSV field for the same value. JSON null becomes an empty string.
func decodeJSONLRow(line []byte) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var obj map[string]interface{}
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decoding JSON object: %w", err)
	}
	row := make(map[string]string, len(obj))
	for k, v := range obj {
		s, err := jsonValueText(v)
		if err != nil {
			return nil, fmt.Errorf("encoding value for %s: %w", k, err)
		}
		row[k] = stripNUL(s)
	}
	return row, nil
}

// jsonValueText renders a decoded JSON value as text. Nested values, which
// mac_apt does not write, are kept as their JSON text.
func jsonValueText(v interface{}) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	case bool:
		return strconv.FormatBool(t), nil
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

// stripNUL removes NUL bytes, which PostgreSQL text columns reject and which
// some mac_apt fields contain.
func stripNUL(s string) string {
	if !strings.ContainsRune(s, 0) {
		return s
	}
	return strings.ReplaceAll(s, "\x00", "")
}

// parseTimestamp parses a mac_apt timestamp as UTC. It accepts
// "YYYY-MM-DD HH:MM:SS" with optional fractional seconds, optionally followed
// by "+00:00". Any other offset fails.
func parseTimestamp(s string) (time.Time, bool) {
	s = strings.TrimSuffix(strings.TrimSpace(s), utcOffset)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse("2006-01-02 15:04:05.999999", s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// parseColumnTimestamp parses one timestamp column. An empty value is not an
// error and not counted: it simply yields no event. A non-empty value that
// fails to parse yields no event and bad is 1.
func parseColumnTimestamp(raw string) (when time.Time, ok bool, bad int) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, false, 0
	}
	when, ok = parseTimestamp(raw)
	if !ok {
		return time.Time{}, false, 1
	}
	return when, true, 0
}

// aslFullPrecision adds ASL's nanoseconds to its second-precision timestamp
// for the extra column. Zero, empty, or non-numeric values leave the
// timestamp unchanged, as does a timestamp that already has a fraction.
func aslFullPrecision(ts, nano string) string {
	n, err := strconv.ParseUint(strings.TrimSpace(nano), 10, 64)
	if err != nil || n == 0 || n >= 1e9 {
		return ts
	}
	base, zone := ts, ""
	if strings.HasSuffix(ts, utcOffset) {
		base, zone = strings.TrimSuffix(ts, utcOffset), utcOffset
	}
	if strings.Contains(base, ".") {
		return ts
	}
	return base + fmt.Sprintf(".%09d", n) + zone
}

// field is one key/value pair for the extra column.
type field struct {
	key   string
	value string
}

var lineBreakRun = regexp.MustCompile(`[\r\n]+`)

// renderExtra writes the unmapped fields in eztoolparser's convention:
// "key: value" pairs joined by "; ", with empty values omitted. Keys are the
// source column names as mac_apt wrote them, not normalized names, so an
// examiner can match an extra entry to the CSV column it came from. Each run
// of CR or LF inside a value becomes one space, so the extra column stays on
// one line.
func renderExtra(fields ...field) string {
	var parts []string
	for _, f := range fields {
		v := strings.TrimSpace(lineBreakRun.ReplaceAllString(f.value, " "))
		if v == "" {
			continue
		}
		parts = append(parts, f.key+": "+v)
	}
	return strings.Join(parts, "; ")
}

// newEvent returns an event with the fields every mac_apt output shares.
func newEvent(source, sourceType string, when time.Time, filename string) *model.Event {
	return &model.Event{
		Datetime:   when.UTC().Format(datetimeLayout),
		Timezone:   timezoneUTC,
		MACB:       macb,
		Source:     source,
		SourceType: sourceType,
		Filename:   filename,
	}
}

// mapFsEvents maps one FsEvents row to one event.
func mapFsEvents(row map[string]string) ([]*model.Event, int) {
	when, ok, bad := parseColumnTimestamp(row["SourceModDate"])
	if !ok {
		return nil, bad
	}
	e := newEvent(fsEventsSource, fsEventsLabel, when, row["Source"])
	e.Type = fsEventsType
	e.Desc = row["EventFlags"] + ": " + row["Filepath"]
	e.Inode = row["File_ID"]
	e.RecordNumber = row["LogID"]
	e.EventType = row["EventType"]
	e.Extra = renderExtra(
		field{"SourceModDate", row["SourceModDate"]},
		field{"EventFlagsHex", row["EventFlagsHex"]},
		field{"Log_Unknown", row["Log_Unknown"]},
	)
	return []*model.Event{e}, 0
}

// biomeTimestampColumns are the columns Biome rows may carry a time in, in
// the order they produce events.
var biomeTimestampColumns = []string{"Timestamp1", "Timestamp2"}

// mapBiome maps one Biome row to one event per timestamp column that parses.
func mapBiome(row map[string]string) ([]*model.Event, int) {
	var events []*model.Event
	skipped := 0
	for _, col := range biomeTimestampColumns {
		when, ok, bad := parseColumnTimestamp(row[col])
		skipped += bad
		if !ok {
			continue
		}
		e := newEvent(biomeSource, biomeLabel, when, row["Source"])
		e.Type = col
		e.Desc = row["Biome Type"] + ": " + row["Data"]
		e.EventType = row["Biome Type"]
		e.RecordNumber = row["Offset"]
		e.User = biomeUser(row["User"])
		e.Extra = renderExtra(
			field{col, row[col]},
			field{"Filename timestamp", row["Filename timestamp"]},
			field{"State", row["State"]},
		)
		events = append(events, e)
	}
	return events, skipped
}

// biomeUser maps Biome's "-" placeholder to an empty user.
func biomeUser(u string) string {
	if u == "-" {
		return ""
	}
	return u
}

// mapASL maps one asl_ver2 row to one event.
func mapASL(row map[string]string) ([]*model.Event, int) {
	when, ok, bad := parseColumnTimestamp(row["Timestamp"])
	if !ok {
		return nil, bad
	}
	e := newEvent(aslSource, aslLabel, when, row["Source"])
	e.Host = row["Host"]
	e.User = row["UID"]
	e.Type = row["Level"]
	e.Desc = row["Message"]
	e.SourceName = row["Sender"]
	e.EventType = row["Facility"]
	e.RecordNumber = row["ID"]
	e.Extra = renderExtra(
		field{"Timestamp", aslFullPrecision(row["Timestamp"], row["Nano"])},
		field{"PID", row["PID"]},
		field{"GID", row["GID"]},
		field{"RefProc", row["RefProc"]},
		field{"RefPID", row["RefPID"]},
		field{"Session", row["Session"]},
		field{"Flags", row["Flags"]},
		field{"Key_Value", row["Key_Value"]},
	)
	return []*model.Event{e}, 0
}

// mapCFURLCache maps one CFURL_Cache row to one event. Received_Data is
// deliberately left out: it is a hex blob that can be large.
func mapCFURLCache(row map[string]string) ([]*model.Event, int) {
	when, ok, bad := parseColumnTimestamp(row["Date"])
	if !ok {
		return nil, bad
	}
	e := newEvent(cfurlSource, cfurlLabel, when, row["Source"])
	e.Type = cfurlType
	e.Desc = fmt.Sprintf("%s %s (%s)", row["Method"], row["URL"], row["HTTP_Status"])
	e.URL = row["URL"]
	e.User = row["User"]
	e.SourceName = row["App_Bundle_ID"]
	e.EventType = row["HTTP_Status"]
	e.Extra = renderExtra(
		field{"Date", row["Date"]},
		field{"isDataOnFS", row["isDataOnFS"]},
		field{"Request_Header", row["Request_Header"]},
		field{"Response_Header", row["Response_Header"]},
	)
	return []*model.Event{e}, 0
}
