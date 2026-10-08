// Package unifiedlogparser imports macOS Unified Log records exported by
// Mandiant's macos-UnifiedLogs unifiedlog_iterator (tested against v0.7.0),
// in both its CSV and JSONL output formats.
package unifiedlogparser

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/textutil"
)

const (
	// Source is the normalized source value for every Unified Log event.
	Source = "UNIFIEDLOG"
	// SourceType is the sourcetype value for every Unified Log event.
	SourceType = "macOS Unified Log"

	// macb matches the value the EvtxECmd mapping in eztoolparser produces:
	// its TimeCreated column derives to "....", a generic event time rather
	// than a file MACB timestamp.
	macb = "...."

	// datetimeLayout matches the "YYYY-MM-DD HH:MM:SS" UTC format the other
	// parsers store; fractional seconds are dropped, as eztoolparser does.
	datetimeLayout = "2006-01-02 15:04:05"

	// maxLineSize caps a single JSONL line. Statedump records run to many
	// kilobytes, well past bufio.Scanner's 64KB default.
	maxLineSize = 64 * 1024 * 1024

	progressInterval = 10000
)

// ErrNoRecords is returned by the fingerprint functions when a file holds no
// record with a usable timestamp.
var ErrNoRecords = errors.New("no Unified Log record with a valid timestamp")

// CSV column names from unifiedlog_iterator v0.7.0. Columns are looked up by
// name, so older output without "Parent Activity ID" still imports.
const (
	colTimestamp        = "Timestamp"
	colEventType        = "Event Type"
	colLogType          = "Log Type"
	colSubsystem        = "Subsystem"
	colThreadID         = "Thread ID"
	colPID              = "PID"
	colEUID             = "EUID"
	colLibrary          = "Library"
	colLibraryUUID      = "Library UUID"
	colActivityID       = "Activity ID"
	colParentActivityID = "Parent Activity ID"
	colCategory         = "Category"
	colProcess          = "Process"
	colProcessUUID      = "Process UUID"
	colMessage          = "Message"
	colRawMessage       = "Raw Message"
	colBootUUID         = "Boot UUID"
	colTimezoneName     = "System Timezone Name"
)

// requiredCSVColumns must all be present for a CSV header to be detected as
// Unified Log output.
var requiredCSVColumns = []string{
	colTimestamp, colEventType, colLogType, colProcess, colMessage, colRawMessage, colBootUUID,
}

// ReadResult contains the outcome of a Unified Log import.
type ReadResult struct {
	Events []*model.Event
	Count  int
	// Excluded counts records skipped because they had no usable timestamp
	// or could not be decoded.
	Excluded int
}

// record is the format-neutral form of one Unified Log entry. Both the CSV
// and JSONL readers decode into it, so event mapping and fingerprinting are
// shared.
type record struct {
	when             time.Time
	sourceTimestamp  string // full precision; datetime drops fractional seconds
	eventType        string
	logType          string
	subsystem        string
	threadID         string
	pid              string
	euid             string
	library          string
	libraryUUID      string
	activityID       string
	parentActivityID string
	category         string
	process          string
	processUUID      string
	message          string
	rawMessage       string
	bootUUID         string
	timezoneName     string
	messageFlags     []string // JSONL only
	evidence         string   // JSONL only
}

// toEvent maps a record onto the 4n6time event model.
func (r *record) toEvent() *model.Event {
	desc := r.message
	if desc == "" {
		desc = r.rawMessage
	}
	return &model.Event{
		Datetime:   r.when.UTC().Format(datetimeLayout),
		Timezone:   "UTC",
		MACB:       macb,
		Source:     Source,
		SourceType: SourceType,
		Type:       r.eventType,
		EventType:  r.logType,
		Desc:       desc,
		Filename:   r.process,
		SourceName: r.subsystem,
		User:       r.euid,
		Extra:      r.extra(),
	}
}

// extra renders the unmapped fields with eztoolparser's convention:
// "key: value" pairs joined by "; ", with empty values omitted.
func (r *record) extra() string {
	pairs := []struct{ key, val string }{
		{"timestamp", r.sourceTimestamp},
		{"category", r.category},
		{"pid", r.pid},
		{"thread_id", r.threadID},
		{"library", r.library},
		{"library_uuid", r.libraryUUID},
		{"activity_id", r.activityID},
		{"parent_activity_id", r.parentActivityID},
		{"process_uuid", r.processUUID},
		{"boot_uuid", r.bootUUID},
		{"timezone_name", r.timezoneName},
		{"raw_message", r.rawMessage},
		{"message_flags", strings.Join(r.messageFlags, ",")},
		{"evidence", r.evidence},
	}
	var parts []string
	for _, p := range pairs {
		if p.val != "" {
			parts = append(parts, p.key+": "+p.val)
		}
	}
	return strings.Join(parts, "; ")
}

// fingerprint identifies the log data a record came from. Boot UUID alone is
// not enough: separate collections from the same boot share it. Adding the
// timestamp truncated to milliseconds (the CSV's precision) and the process
// UUID distinguishes collections while letting a CSV and a JSONL export of
// the same data match.
func (r *record) fingerprint() string {
	return strings.ToUpper(r.bootUUID) + "|" +
		r.when.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z") + "|" +
		strings.ToUpper(r.processUUID)
}

// --- CSV ---

// newCSVReader returns a reader configured for unifiedlog_iterator CSV,
// whose Message fields often span lines inside quotes.
func newCSVReader(r io.Reader) *csv.Reader {
	reader := csv.NewReader(textutil.NewBOMStrippingReader(r))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	return reader
}

// readCSVHeader reads and normalizes the header row, returning a column
// index keyed by name.
func readCSVHeader(reader *csv.Reader) (map[string]int, error) {
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	colIndex := make(map[string]int, len(header))
	for i, col := range header {
		colIndex[strings.TrimSpace(col)] = i
	}
	return colIndex, nil
}

func isUnifiedLogHeader(colIndex map[string]int) bool {
	for _, col := range requiredCSVColumns {
		if _, ok := colIndex[col]; !ok {
			return false
		}
	}
	return true
}

// DetectCSV reports whether path is unifiedlog_iterator CSV output, based on
// its header row.
func DetectCSV(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	colIndex, err := readCSVHeader(newCSVReader(f))
	if err != nil {
		return false, err
	}
	return isUnifiedLogHeader(colIndex), nil
}

// recordFromCSV decodes one CSV row. ok is false when the timestamp is
// missing or unparseable.
func recordFromCSV(row []string, colIndex map[string]int) (*record, bool) {
	col := func(name string) string {
		i, ok := colIndex[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(strings.TrimRight(row[i], "\r"))
	}

	timestamp := col(colTimestamp)
	when, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return nil, false
	}

	return &record{
		when:             when,
		sourceTimestamp:  timestamp,
		eventType:        col(colEventType),
		logType:          col(colLogType),
		subsystem:        col(colSubsystem),
		threadID:         normalizeUint(col(colThreadID)),
		pid:              normalizeUint(col(colPID)),
		euid:             normalizeUint(col(colEUID)),
		library:          col(colLibrary),
		libraryUUID:      col(colLibraryUUID),
		activityID:       normalizeUint(col(colActivityID)),
		parentActivityID: normalizeUint(col(colParentActivityID)),
		category:         col(colCategory),
		process:          col(colProcess),
		processUUID:      col(colProcessUUID),
		// Message bodies keep their internal whitespace and newlines; only
		// the trailing CR a CRLF export leaves behind is dropped.
		message:      strings.TrimRight(rawCol(row, colIndex, colMessage), "\r"),
		rawMessage:   strings.TrimRight(rawCol(row, colIndex, colRawMessage), "\r"),
		bootUUID:     col(colBootUUID),
		timezoneName: col(colTimezoneName),
	}, true
}

// rawCol returns a column value without trimming.
func rawCol(row []string, colIndex map[string]int, name string) string {
	i, ok := colIndex[name]
	if !ok || i >= len(row) {
		return ""
	}
	return row[i]
}

// eachCSVRecord decodes every row of a Unified Log CSV, calling fn for each
// usable record until fn returns false. It returns the number of rows that
// were skipped.
func eachCSVRecord(path string, fn func(*record) bool) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := newCSVReader(f)
	colIndex, err := readCSVHeader(reader)
	if err != nil {
		return 0, err
	}
	if !isUnifiedLogHeader(colIndex) {
		return 0, fmt.Errorf("not a Unified Log CSV: missing required columns")
	}

	excluded := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			excluded++
			continue
		}
		rec, ok := recordFromCSV(row, colIndex)
		if !ok {
			excluded++
			continue
		}
		if !fn(rec) {
			break
		}
	}
	return excluded, nil
}

// ReadCSV parses a unifiedlog_iterator CSV file. onProgress, if non-nil, is
// called every 10,000 events.
func ReadCSV(path string, onProgress func(int)) (*ReadResult, error) {
	return read(path, eachCSVRecord, onProgress)
}

// FingerprintCSV returns the fingerprint of the first usable record in a
// Unified Log CSV file.
func FingerprintCSV(path string) (string, error) {
	return firstFingerprint(path, eachCSVRecord)
}

// --- JSONL ---

// jsonRecord mirrors one unifiedlog_iterator JSONL object. Numeric IDs are
// decoded as json.Number and converted with strconv.ParseUint: activity IDs
// in real Statedump records exceed the int64 range, and float64 would lose
// precision. message_entries is deliberately not decoded.
type jsonRecord struct {
	Subsystem        string      `json:"subsystem"`
	ThreadID         json.Number `json:"thread_id"`
	PID              json.Number `json:"pid"`
	EUID             json.Number `json:"euid"`
	Library          string      `json:"library"`
	LibraryUUID      string      `json:"library_uuid"`
	ActivityID       json.Number `json:"activity_id"`
	ParentActivityID json.Number `json:"parent_activity_id"`
	Time             json.Number `json:"time"`
	Category         string      `json:"category"`
	EventType        string      `json:"event_type"`
	LogType          string      `json:"log_type"`
	Process          string      `json:"process"`
	ProcessUUID      string      `json:"process_uuid"`
	Message          string      `json:"message"`
	RawMessage       string      `json:"raw_message"`
	BootUUID         string      `json:"boot_uuid"`
	TimezoneName     string      `json:"timezone_name"`
	Timestamp        string      `json:"timestamp"`
	MessageFlags     []string    `json:"message_flags"`
	Evidence         string      `json:"evidence"`
}

// requiredJSONKeys must all be present, plus "timestamp" or "time", for a
// JSONL line to be detected as Unified Log output.
var requiredJSONKeys = []string{"boot_uuid", "event_type", "log_type", "process"}

// newLineScanner returns a scanner over r with a leading BOM stripped and a
// line limit large enough for statedump records.
func newLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(textutil.NewBOMStrippingReader(r))
	scanner.Buffer(make([]byte, 0, 1024*1024), maxLineSize)
	return scanner
}

// DetectJSONL reports whether path is unifiedlog_iterator JSONL output,
// based on its first non-empty line. A first line that is not a JSON object
// returns an error; a JSON object without the Unified Log keys returns false.
func DetectJSONL(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("opening file: %w", err)
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
			return false, fmt.Errorf("first line is not a JSON object: %w", err)
		}
		for _, key := range requiredJSONKeys {
			if _, ok := obj[key]; !ok {
				return false, nil
			}
		}
		_, hasTimestamp := obj["timestamp"]
		_, hasTime := obj["time"]
		return hasTimestamp || hasTime, nil
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("reading file: %w", err)
	}
	return false, fmt.Errorf("empty file")
}

// recordFromJSON decodes one JSONL line. ok is false when the line is not
// valid JSON or yields no usable timestamp.
func recordFromJSON(line []byte) (*record, bool) {
	var jr jsonRecord
	if err := json.Unmarshal(line, &jr); err != nil {
		return nil, false
	}

	when, sourceTimestamp, ok := jsonTime(jr.Timestamp, jr.Time)
	if !ok {
		return nil, false
	}

	return &record{
		when:             when,
		sourceTimestamp:  sourceTimestamp,
		eventType:        jr.EventType,
		logType:          jr.LogType,
		subsystem:        jr.Subsystem,
		threadID:         normalizeUint(jr.ThreadID.String()),
		pid:              normalizeUint(jr.PID.String()),
		euid:             normalizeUint(jr.EUID.String()),
		library:          jr.Library,
		libraryUUID:      jr.LibraryUUID,
		activityID:       normalizeUint(jr.ActivityID.String()),
		parentActivityID: normalizeUint(jr.ParentActivityID.String()),
		category:         jr.Category,
		process:          jr.Process,
		processUUID:      jr.ProcessUUID,
		message:          jr.Message,
		rawMessage:       jr.RawMessage,
		bootUUID:         jr.BootUUID,
		timezoneName:     jr.TimezoneName,
		messageFlags:     jr.MessageFlags,
		evidence:         trimEvidence(jr.Evidence),
	}, true
}

// eachJSONLRecord decodes every line of a Unified Log JSONL file, calling fn
// for each usable record until fn returns false. It returns the number of
// lines that were skipped.
func eachJSONLRecord(path string, fn func(*record) bool) (int, error) {
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
		rec, ok := recordFromJSON(line)
		if !ok {
			excluded++
			continue
		}
		if !fn(rec) {
			return excluded, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return excluded, fmt.Errorf("reading file at line %d: %w", lineNum+1, err)
	}
	return excluded, nil
}

// ReadJSONL parses a unifiedlog_iterator JSONL file. onProgress, if non-nil,
// is called every 10,000 events.
func ReadJSONL(path string, onProgress func(int)) (*ReadResult, error) {
	return read(path, eachJSONLRecord, onProgress)
}

// FingerprintJSONL returns the fingerprint of the first usable record in a
// Unified Log JSONL file.
func FingerprintJSONL(path string) (string, error) {
	return firstFingerprint(path, eachJSONLRecord)
}

// --- shared ---

type recordIterator func(path string, fn func(*record) bool) (int, error)

func read(path string, each recordIterator, onProgress func(int)) (*ReadResult, error) {
	result := &ReadResult{}
	excluded, err := each(path, func(rec *record) bool {
		result.Events = append(result.Events, rec.toEvent())
		result.Count++
		if onProgress != nil && result.Count%progressInterval == 0 {
			onProgress(result.Count)
		}
		return true
	})
	result.Excluded = excluded
	if err != nil {
		return nil, err
	}
	return result, nil
}

func firstFingerprint(path string, each recordIterator) (string, error) {
	var fp string
	if _, err := each(path, func(rec *record) bool {
		fp = rec.fingerprint()
		return false
	}); err != nil {
		return "", err
	}
	if fp == "" {
		return "", ErrNoRecords
	}
	return fp, nil
}

// jsonTime resolves a JSONL record's time. The RFC3339Nano "timestamp"
// string is preferred; "time" (float nanoseconds since the Unix epoch) is
// the fallback. It also returns the full-precision source value for extra:
// the "timestamp" string verbatim, or the converted "time" value rendered as
// RFC3339Nano in UTC when the fallback was used.
func jsonTime(timestamp string, nanos json.Number) (time.Time, string, bool) {
	if timestamp != "" {
		if t, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
			return t, timestamp, true
		}
	}
	if nanos != "" {
		if t, ok := timeFromNanos(nanos.String()); ok {
			return t, t.UTC().Format(time.RFC3339Nano), true
		}
	}
	return time.Time{}, "", false
}

// timeFromNanos converts a decimal or exponent-form count of nanoseconds
// since the Unix epoch into a time. It parses with math/big so values like
// 1.791384828542682e+18 convert without float64 rounding of the integer.
func timeFromNanos(s string) (time.Time, bool) {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		f, _, err := big.ParseFloat(s, 10, 256, big.ToNearestEven)
		if err != nil {
			return time.Time{}, false
		}
		n, _ = f.Int(nil)
	}
	if n.Sign() <= 0 || !n.IsInt64() {
		return time.Time{}, false
	}
	ns := n.Int64()
	return time.Unix(ns/int64(time.Second), ns%int64(time.Second)).UTC(), true
}

// normalizeUint renders a numeric ID as a plain decimal string. Values that
// do not parse as an unsigned 64-bit integer are kept as given rather than
// dropped.
func normalizeUint(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		return strconv.FormatUint(v, 10)
	}
	return s
}

// trimEvidence shortens an evidence path to the part inside the
// .logarchive bundle, with forward slashes, so the same archive reads the
// same regardless of where it sat on the examiner's machine. Paths without
// .logarchive are returned unchanged.
func trimEvidence(path string) string {
	const marker = ".logarchive"
	idx := strings.LastIndex(strings.ToLower(path), marker)
	if idx < 0 {
		return path
	}
	rest := strings.ReplaceAll(path[idx+len(marker):], `\`, "/")
	return strings.TrimLeft(rest, "/")
}
