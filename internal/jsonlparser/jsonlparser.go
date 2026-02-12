package jsonlparser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cdtdelta/4n6time/internal/model"
)

// ReadResult contains the outcome of a JSONL import operation.
type ReadResult struct {
	Events   []*model.Event
	Count    int
	Excluded int
}

// plasoEvent represents the JSON structure of a Plaso json_line event.
// Fields are loosely typed since Plaso outputs vary by parser and version.
type plasoEvent struct {
	// Core timestamp fields
	Timestamp     interface{} `json:"timestamp"`
	Datetime      string      `json:"datetime"`
	TimestampDesc string      `json:"timestamp_desc"`

	// Source identification
	SourceShort string `json:"source_short"`
	SourceLong  string `json:"source_long"`
	Source      string `json:"source"`

	// Event content
	Message     string `json:"message"`
	DisplayName string `json:"display_name"`
	Filename    string `json:"filename"`
	Inode       string `json:"inode"`
	Parser      string `json:"parser"`

	// System context
	Hostname string `json:"hostname"`
	Username string `json:"username"`

	// Event identifiers
	EventIdentifier interface{} `json:"event_identifier"`
	EventType       interface{} `json:"event_type"`
	SourceName      string      `json:"source_name"`
	UserSID         string      `json:"user_sid"`
	ComputerName    string      `json:"computer_name"`
	RecordNumber    interface{} `json:"record_number"`

	// Storage identifiers
	StoreNumber    interface{} `json:"store_number"`
	StoreIndex     interface{} `json:"store_index"`
	VSSStoreNumber interface{} `json:"vss_store_number"`

	// URL
	URL string `json:"url"`

	// Timezone
	Zone    string      `json:"zone"`
	Offset  interface{} `json:"offset"`
	INode   interface{} `json:"inode_number"`
	PathSep string      `json:"path_separator"`

	// Tag
	Tag     string      `json:"tag"`
	TagList interface{} `json:"tag_list"`

	// Catch-all for extra fields
	Extra map[string]interface{} `json:"-"`
}

// ValidateFile checks if a file looks like Plaso JSONL by reading the first line.
func ValidateFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	if !scanner.Scan() {
		return fmt.Errorf("empty file")
	}

	line := strings.TrimSpace(scanner.Text())
	if len(line) == 0 || line[0] != '{' {
		return fmt.Errorf("first line is not a JSON object")
	}

	// Try to parse it
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return fmt.Errorf("first line is not valid JSON: %w", err)
	}

	// Check for at least one Plaso-typical field
	_, hasTimestamp := raw["timestamp"]
	_, hasDatetime := raw["datetime"]
	_, hasMessage := raw["message"]
	_, hasSourceShort := raw["source_short"]

	if !hasTimestamp && !hasDatetime {
		return fmt.Errorf("no timestamp or datetime field found; does not appear to be Plaso JSONL")
	}
	if !hasMessage && !hasSourceShort {
		return fmt.Errorf("no message or source_short field found; does not appear to be Plaso JSONL")
	}

	return nil
}

// ReadEvents reads all events from a Plaso JSONL file.
// An onProgress callback is called every 10,000 events if non-nil.
func ReadEvents(path string, onProgress func(count int)) (*ReadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Allow up to 10MB per line (some Plaso events can be very large)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var events []*model.Event
	count := 0
	excluded := 0
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse into raw map first to capture all fields
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			excluded++
			continue
		}

		// Parse known fields
		var pe plasoEvent
		if err := json.Unmarshal([]byte(line), &pe); err != nil {
			excluded++
			continue
		}

		event := mapToEvent(&pe, raw)
		if event == nil {
			excluded++
			continue
		}

		events = append(events, event)
		count++

		if onProgress != nil && count%10000 == 0 {
			onProgress(count)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file at line %d: %w", lineNum, err)
	}

	return &ReadResult{
		Events:   events,
		Count:    count,
		Excluded: excluded,
	}, nil
}

// mapToEvent converts a Plaso JSON event to our Event model.
func mapToEvent(pe *plasoEvent, raw map[string]interface{}) *model.Event {
	e := &model.Event{}

	// Datetime: prefer "datetime" string, fall back to converting timestamp
	e.Datetime = pe.Datetime
	if e.Datetime == "" && pe.Timestamp != nil {
		e.Datetime = convertTimestamp(pe.Timestamp)
	}

	// Timezone
	e.Timezone = pe.Zone
	if e.Timezone == "" {
		e.Timezone = "UTC"
	}

	// MACB from timestamp_desc
	e.MACB = mapTimestampDescToMACB(pe.TimestampDesc)

	// Source (short source)
	e.Source = pe.SourceShort
	if e.Source == "" {
		e.Source = pe.Source
	}

	// Source type (long source)
	e.SourceType = pe.SourceLong

	// Type (timestamp description)
	e.Type = pe.TimestampDesc

	// User
	e.User = pe.Username

	// Host
	e.Host = pe.Hostname

	// Description (message)
	e.Desc = pe.Message

	// Filename
	e.Filename = pe.Filename
	if e.Filename == "" {
		e.Filename = pe.DisplayName
	}

	// Inode
	e.Inode = pe.Inode
	if e.Inode == "" {
		e.Inode = interfaceToString(pe.INode)
	}

	// Format (parser name)
	e.Format = pe.Parser

	// URL
	e.URL = pe.URL

	// Event identifiers
	e.EventID = interfaceToString(pe.EventIdentifier)
	e.EventType = interfaceToString(pe.EventType)
	e.SourceName = pe.SourceName
	e.UserSID = pe.UserSID
	e.ComputerName = pe.ComputerName
	e.RecordNumber = interfaceToString(pe.RecordNumber)

	// Storage fields
	e.StoreNumber = interfaceToInt64(pe.StoreNumber)
	e.StoreIndex = interfaceToInt64(pe.StoreIndex)
	e.VSSStoreNumber = interfaceToInt64(pe.VSSStoreNumber)
	e.Offset = interfaceToInt64(pe.Offset)

	// Tag
	e.Tag = pe.Tag
	if e.Tag == "" && pe.TagList != nil {
		if tagList, ok := pe.TagList.([]interface{}); ok {
			tags := make([]string, 0, len(tagList))
			for _, t := range tagList {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
			e.Tag = strings.Join(tags, ", ")
		}
	}

	// Extra: collect any unrecognized fields into the Extra column
	knownFields := map[string]bool{
		"timestamp": true, "datetime": true, "timestamp_desc": true,
		"source_short": true, "source_long": true, "source": true,
		"message": true, "display_name": true, "filename": true,
		"inode": true, "parser": true, "hostname": true, "username": true,
		"event_identifier": true, "event_type": true, "source_name": true,
		"user_sid": true, "computer_name": true, "record_number": true,
		"store_number": true, "store_index": true, "vss_store_number": true,
		"url": true, "zone": true, "offset": true, "inode_number": true,
		"path_separator": true, "tag": true, "tag_list": true,
		"__container_type__": true, "__type__": true,
	}

	var extras []string
	for k, v := range raw {
		if !knownFields[k] {
			extras = append(extras, fmt.Sprintf("%s: %v", k, v))
		}
	}
	if len(extras) > 0 {
		e.Extra = strings.Join(extras, "; ")
	}

	return e
}

// convertTimestamp converts a Plaso timestamp (Unix epoch microseconds) to datetime string.
func convertTimestamp(ts interface{}) string {
	switch v := ts.(type) {
	case float64:
		// Plaso timestamps are in microseconds
		sec := int64(v) / 1000000
		usec := int64(v) % 1000000
		t := time.Unix(sec, usec*1000).UTC()
		return t.Format("2006-01-02 15:04:05")
	case string:
		// Already a string, try to use as-is
		return v
	default:
		return ""
	}
}

// mapTimestampDescToMACB converts a Plaso timestamp_desc to MACB notation.
func mapTimestampDescToMACB(desc string) string {
	desc = strings.ToLower(desc)
	macb := [4]byte{'.', '.', '.', '.'}

	if strings.Contains(desc, "modification") || strings.Contains(desc, "modified") || strings.Contains(desc, "written") {
		macb[0] = 'M'
	}
	if strings.Contains(desc, "access") || strings.Contains(desc, "visited") {
		macb[1] = 'A'
	}
	if strings.Contains(desc, "change") || strings.Contains(desc, "entry") || strings.Contains(desc, "metadata") {
		macb[2] = 'C'
	}
	if strings.Contains(desc, "creation") || strings.Contains(desc, "birth") || strings.Contains(desc, "created") {
		macb[3] = 'B'
	}

	result := string(macb[:])
	if result == "...." {
		// No MACB mapping found, return empty
		return ""
	}
	return result
}

// interfaceToString converts various types to string.
func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// interfaceToInt64 converts various types to int64.
func interfaceToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	default:
		return 0
	}
}
