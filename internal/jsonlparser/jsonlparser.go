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

	// Check for Plaso-typical fields.
	// Raw format: has "data_type" and "date_time" (nested object)
	// psort format: has "timestamp"/"datetime" and "source_short"/"message"
	_, hasTimestamp := raw["timestamp"]
	_, hasDatetime := raw["datetime"]
	_, hasDataType := raw["data_type"]
	_, hasDateTimeObj := raw["date_time"]
	_, hasMessage := raw["message"]
	_, hasSourceShort := raw["source_short"]

	isRaw := hasDataType && hasDateTimeObj
	isPsort := (hasTimestamp || hasDatetime) && (hasMessage || hasSourceShort)

	if !isRaw && !isPsort {
		return fmt.Errorf("does not appear to be Plaso JSONL (missing expected fields)")
	}

	return nil
}

// ReadEvents reads all events from a Plaso JSONL file.
// Supports both raw Plaso storage format and psort json_line output.
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

		// Parse into raw map to capture all fields
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			excluded++
			continue
		}

		event := mapRawToEvent(raw)
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

// mapRawToEvent converts a raw JSON map to our Event model.
// Detects format (raw Plaso vs psort) and handles accordingly.
func mapRawToEvent(raw map[string]interface{}) *model.Event {
	e := &model.Event{}

	// Detect format: raw Plaso has "data_type" and nested "date_time" object
	_, hasDataType := raw["data_type"]
	_, hasDateTimeObj := raw["date_time"]
	isRaw := hasDataType && hasDateTimeObj

	if isRaw {
		mapRawPlasoFields(e, raw)
	} else {
		mapPsortFields(e, raw)
	}

	// Collect extra fields
	e.Extra = collectExtras(raw)

	return e
}

// mapRawPlasoFields handles the raw Plaso storage JSON format.
// This is what you get from directly exporting Plaso storage, not from psort.
func mapRawPlasoFields(e *model.Event, raw map[string]interface{}) {
	dataType := getStr(raw, "data_type")

	// Datetime from nested date_time object
	e.Datetime = convertDateTimeObject(raw["date_time"])

	// Timezone
	e.Timezone = getStr(raw, "zone")
	if e.Timezone == "" {
		e.Timezone = "UTC"
	}

	// timestamp_desc
	tsDesc := getStr(raw, "timestamp_desc")
	e.Type = tsDesc
	e.MACB = mapTimestampDescToMACB(tsDesc)

	// Source from data_type mapping
	short, long := mapDataTypeToSource(dataType)
	e.Source = short
	e.SourceType = long

	// User
	e.User = getStr(raw, "username")

	// Host
	e.Host = getStr(raw, "hostname")
	if e.Host == "" {
		e.Host = getStr(raw, "computer_name")
	}

	// Description (message)
	e.Desc = getStr(raw, "message")

	// Filename
	e.Filename = getStr(raw, "filename")
	if e.Filename == "" {
		e.Filename = getStr(raw, "display_name")
	}

	// Inode
	e.Inode = getStr(raw, "inode")

	// Format (parser name)
	e.Format = getStr(raw, "parser")

	// URL
	e.URL = getStr(raw, "url")

	// Event identifiers
	e.EventID = interfaceToString(raw["event_identifier"])
	e.EventType = interfaceToString(raw["event_type"])
	e.SourceName = getStr(raw, "source_name")
	e.UserSID = getStr(raw, "user_sid")
	e.ComputerName = getStr(raw, "computer_name")
	e.RecordNumber = interfaceToString(raw["record_number"])

	// Storage fields
	e.StoreNumber = interfaceToInt64(raw["store_number"])
	e.StoreIndex = interfaceToInt64(raw["store_index"])
	e.VSSStoreNumber = interfaceToInt64(raw["vss_store_number"])
	e.Offset = interfaceToInt64(raw["offset"])

	// Tag
	e.Tag = getStr(raw, "tag")
	if e.Tag == "" {
		e.Tag = extractTagList(raw["tag_list"])
	}
}

// mapPsortFields handles the psort json_line output format.
// This is the output from: psort.py -o json_line
func mapPsortFields(e *model.Event, raw map[string]interface{}) {
	// Datetime: prefer "datetime" string, fall back to converting timestamp
	e.Datetime = getStr(raw, "datetime")
	if e.Datetime == "" && raw["timestamp"] != nil {
		e.Datetime = convertTimestamp(raw["timestamp"])
	}
	// Normalize ISO format to space-separated for consistency with CSV imports
	e.Datetime = normalizeDatetime(e.Datetime)

	// Timezone
	e.Timezone = getStr(raw, "zone")
	if e.Timezone == "" {
		e.Timezone = "UTC"
	}

	// MACB from timestamp_desc
	tsDesc := getStr(raw, "timestamp_desc")
	e.MACB = mapTimestampDescToMACB(tsDesc)
	e.Type = tsDesc

	// Source (short source)
	e.Source = getStr(raw, "source_short")
	if e.Source == "" {
		e.Source = getStr(raw, "source")
	}
	// If still empty and we have data_type, derive from that
	if e.Source == "" {
		dataType := getStr(raw, "data_type")
		if dataType != "" {
			short, long := mapDataTypeToSource(dataType)
			e.Source = short
			e.SourceType = long
		}
	}

	// Source type (long source)
	if e.SourceType == "" {
		e.SourceType = getStr(raw, "source_long")
	}

	// User
	e.User = getStr(raw, "username")

	// Host
	e.Host = getStr(raw, "hostname")
	if e.Host == "" {
		e.Host = getStr(raw, "computer_name")
	}

	// Description (message)
	e.Desc = getStr(raw, "message")

	// Filename
	e.Filename = getStr(raw, "filename")
	if e.Filename == "" {
		e.Filename = getStr(raw, "display_name")
	}

	// Inode
	e.Inode = getStr(raw, "inode")

	// Format (parser name)
	e.Format = getStr(raw, "parser")

	// URL
	e.URL = getStr(raw, "url")

	// Event identifiers
	e.EventID = interfaceToString(raw["event_identifier"])
	e.EventType = interfaceToString(raw["event_type"])
	e.SourceName = getStr(raw, "source_name")
	e.UserSID = getStr(raw, "user_sid")
	e.ComputerName = getStr(raw, "computer_name")
	e.RecordNumber = interfaceToString(raw["record_number"])

	// Storage fields
	e.StoreNumber = interfaceToInt64(raw["store_number"])
	e.StoreIndex = interfaceToInt64(raw["store_index"])
	e.VSSStoreNumber = interfaceToInt64(raw["vss_store_number"])
	e.Offset = interfaceToInt64(raw["offset"])

	// Tag
	e.Tag = getStr(raw, "tag")
	if e.Tag == "" {
		e.Tag = extractTagList(raw["tag_list"])
	}
}

// convertDateTimeObject converts the raw Plaso nested date_time object to a datetime string.
// Raw format: {"__class_name__": "Filetime", "__type__": "DateTimeValues", "timestamp": 12345}
// Also handles: {"__class_name__": "NotSet", ...} for placeholder events.
func convertDateTimeObject(v interface{}) string {
	dtMap, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}

	className := ""
	if cn, ok := dtMap["__class_name__"].(string); ok {
		className = cn
	}

	// Handle NotSet before checking for timestamp
	if strings.ToLower(className) == "notset" || strings.ToLower(className) == "not set" {
		return "Not a time"
	}

	ticks, ok := dtMap["timestamp"]
	if !ok {
		return ""
	}

	ticksFloat, ok := ticks.(float64)
	if !ok {
		return ""
	}

	// Convert based on date_time class
	switch className {
	case "Filetime":
		return convertFiletime(int64(ticksFloat))
	case "PosixTimeInMicroseconds":
		sec := int64(ticksFloat) / 1000000
		usec := int64(ticksFloat) % 1000000
		t := time.Unix(sec, usec*1000).UTC()
		return t.Format("2006-01-02 15:04:05")
	case "PosixTime":
		t := time.Unix(int64(ticksFloat), 0).UTC()
		return t.Format("2006-01-02 15:04:05")
	case "WebKitTime":
		// WebKit timestamps are microseconds since 1601-01-01 (same epoch as Filetime)
		return convertFiletime(int64(ticksFloat) * 10)
	case "CocoaTime":
		// Seconds since 2001-01-01 00:00:00 UTC
		cocoaEpoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
		t := cocoaEpoch.Add(time.Duration(int64(ticksFloat)) * time.Second)
		return t.Format("2006-01-02 15:04:05")
	case "FATDateTime":
		// FAT timestamps stored as seconds since epoch in Plaso
		t := time.Unix(int64(ticksFloat), 0).UTC()
		return t.Format("2006-01-02 15:04:05")
	case "JavaTime":
		// Milliseconds since Unix epoch
		sec := int64(ticksFloat) / 1000
		msec := int64(ticksFloat) % 1000
		t := time.Unix(sec, msec*1000000).UTC()
		return t.Format("2006-01-02 15:04:05")
	default:
		// Unknown class, try treating as Filetime ticks if large enough
		if ticksFloat > 100000000000 {
			return convertFiletime(int64(ticksFloat))
		}
		// Otherwise try as Unix seconds
		if ticksFloat > 0 {
			t := time.Unix(int64(ticksFloat), 0).UTC()
			return t.Format("2006-01-02 15:04:05")
		}
		return ""
	}
}

// convertFiletime converts Windows FILETIME ticks (100-nanosecond intervals since 1601-01-01)
// to a datetime string.
func convertFiletime(ticks int64) string {
	if ticks <= 0 {
		return "Not a time"
	}
	// FILETIME epoch to Unix epoch offset in ticks (100-ns intervals)
	// 1601-01-01 to 1970-01-01 = 11644473600 seconds = 116444736000000000 ticks
	const filetimeToUnixTicks int64 = 116444736000000000
	unixTicks := ticks - filetimeToUnixTicks
	unixSec := unixTicks / 10000000
	remainderNanos := (unixTicks % 10000000) * 100
	t := time.Unix(unixSec, remainderNanos).UTC()

	return t.Format("2006-01-02 15:04:05")
}

// convertTimestamp converts a psort-format timestamp (Unix epoch microseconds) to datetime string.
func convertTimestamp(ts interface{}) string {
	switch v := ts.(type) {
	case float64:
		// Plaso psort timestamps are in microseconds
		sec := int64(v) / 1000000
		usec := int64(v) % 1000000
		t := time.Unix(sec, usec*1000).UTC()
		return t.Format("2006-01-02 15:04:05")
	case string:
		// Already a string, use as-is
		return v
	default:
		return ""
	}
}

// dataTypeToSource maps Plaso data_type values to source short and long names.
// Based on Plaso's data/sources.config and formatter YAML files.
var dataTypeToSource = map[string][2]string{
	// File system
	"fs:stat":               {"FILE", "File stat"},
	"fs:stat:ntfs":          {"FILE", "NTFS File stat"},
	"fs:ntfs:usn_change":    {"FILE", "NTFS USN Change"},
	"fs:bodyfile:entry":     {"FILE", "Bodyfile Entry"},
	"fs:mactime:line":       {"FILE", "Mactime Line"},
	"fs:stat:ext":           {"FILE", "EXT File stat"},
	"fs:stat:hfs":           {"FILE", "HFS File stat"},

	// Windows Registry
	"windows:registry:key_value":        {"REG", "Registry Key"},
	"windows:registry:appcompatcache":   {"REG", "AppCompatCache"},
	"windows:registry:bagmru":           {"REG", "BagMRU"},
	"windows:registry:bam":              {"REG", "BAM/DAM"},
	"windows:registry:mrulist":          {"REG", "MRUList Registry"},
	"windows:registry:mrulistex":        {"REG", "MRUListEx Registry"},
	"windows:registry:msie_zone":        {"REG", "MSIE Zone Settings"},
	"windows:registry:mount_points2":    {"REG", "Mount Points"},
	"windows:registry:networks":         {"REG", "Networks Registry"},
	"windows:registry:run":              {"REG", "Run/RunOnce Registry"},
	"windows:registry:sam_users":        {"REG", "SAM Users"},
	"windows:registry:services":         {"REG", "Services Registry"},
	"windows:registry:shutdown":         {"REG", "Shutdown Registry"},
	"windows:registry:timezone":         {"REG", "Timezone Registry"},
	"windows:registry:typedurls":        {"REG", "TypedURLs Registry"},
	"windows:registry:userassist":       {"REG", "UserAssist Registry"},
	"windows:registry:usb":              {"REG", "USB Registry"},
	"windows:registry:winlogon":         {"REG", "Winlogon Registry"},

	// Windows Event Log
	"windows:evt:record":    {"EVT", "WinEVT"},
	"windows:evtx:record":   {"EVT", "WinEVTX"},

	// Web Browser
	"chrome:history:file_downloaded":      {"WEBHIST", "Chrome History"},
	"chrome:history:page_visited":         {"WEBHIST", "Chrome History"},
	"chrome:cache:entry":                  {"WEBHIST", "Chrome Cache"},
	"chrome:cookie:entry":                 {"WEBHIST", "Chrome Cookies"},
	"chrome:autofill:entry":               {"WEBHIST", "Chrome Autofill"},
	"firefox:places:bookmark":             {"WEBHIST", "Firefox History"},
	"firefox:places:bookmark_annotation":  {"WEBHIST", "Firefox History"},
	"firefox:places:bookmark_folder":      {"WEBHIST", "Firefox History"},
	"firefox:places:page_visited":         {"WEBHIST", "Firefox History"},
	"firefox:downloads:download":          {"WEBHIST", "Firefox Downloads"},
	"firefox:cache:record":                {"WEBHIST", "Firefox Cache"},
	"firefox:cookie:entry":                {"WEBHIST", "Firefox Cookies"},
	"msiecf:leak":                         {"WEBHIST", "MSIE CF"},
	"msiecf:redirected":                   {"WEBHIST", "MSIE CF"},
	"msiecf:url":                          {"WEBHIST", "MSIE CF"},
	"msie:webcache:container":             {"WEBHIST", "MSIE WebCache"},
	"msie:webcache:containers":            {"WEBHIST", "MSIE WebCache"},
	"msie:webcache:leak":                  {"WEBHIST", "MSIE WebCache"},
	"opera:history:entry":                 {"WEBHIST", "Opera History"},
	"opera:history:typed_entry":           {"WEBHIST", "Opera History"},
	"safari:history:visit":                {"WEBHIST", "Safari History"},

	// Log files
	"syslog:line":                   {"LOG", "Syslog"},
	"syslog:comment":                {"LOG", "Syslog"},
	"syslog:cron:task_run":          {"LOG", "Syslog Cron"},
	"syslog:ssh:login":              {"LOG", "Syslog SSH"},
	"syslog:ssh:failed_connection":  {"LOG", "Syslog SSH"},
	"syslog:ssh:opened_connection":  {"LOG", "Syslog SSH"},
	"bash:history:command":          {"LOG", "Bash History"},
	"apt:history:line":              {"LOG", "APT History"},
	"selinux:line":                  {"LOG", "SELinux"},
	"mac:appfirewall:line":          {"LOG", "Mac AppFirewall"},
	"mac:wifi:line":                 {"LOG", "Mac Wifi"},
	"android:logcat":                {"LOG", "Android Logcat"},

	// OLE CF (Compound File)
	"olecf:dest_list:entry":         {"OLECF", "OLECF Dest List"},
	"olecf:document_summary_info":   {"OLECF", "OLECF Document Summary"},
	"olecf:summary_info":            {"OLECF", "OLECF Summary Info"},
	"olecf:item":                    {"OLECF", "OLECF Item"},

	// LNK (Shortcut)
	"windows:lnk:link":              {"LNK", "Windows Shortcut"},

	// Prefetch
	"windows:prefetch:execution":    {"PREFETCH", "Windows Prefetch"},

	// PE
	"pe_coff:file":                  {"PE", "PE File"},
	"pe:compilation:timestamp":      {"PE", "PE Compilation Time"},
	"pe:import:timestamp":           {"PE", "PE Import Time"},
	"pe:delay_import:timestamp":     {"PE", "PE Delay Import Time"},

	// SQLite databases
	"skype:event:call":              {"LOG", "Skype"},
	"skype:event:chat":              {"LOG", "Skype"},
	"skype:event:account":           {"LOG", "Skype"},
	"skype:event:transferfile":      {"LOG", "Skype"},

	// Recycle Bin
	"windows:metadata:deleted_item": {"RECBIN", "Recycle Bin"},

	// Task Scheduler
	"task_scheduler:task_cache:entry": {"TASK", "Task Scheduler"},

	// Plist
	"plist:key":                     {"PLIST", "Plist Key"},
	"mac:notificationcenter:db":     {"PLIST", "Notification Center"},

	// Systemd Journal
	"systemd:journal":               {"LOG", "Systemd Journal"},

	// Winjob / At
	"windows:tasks:job":             {"TASK", "Windows Job"},

	// Restore point
	"windows:restore_point:info":    {"RP", "Restore Point"},

	// Popularity Contest
	"popularity_contest:session:event": {"LOG", "Popularity Contest"},
	"popularity_contest:log:event":     {"LOG", "Popularity Contest"},

	// MacOS
	"macos:fseventsd:record":        {"LOG", "macOS FSEvents"},
	"macos:spotlight_store:entry":   {"LOG", "macOS Spotlight"},
}

// mapDataTypeToSource returns (short_source, source) for a given data_type.
// Falls back to deriving from the data_type string itself.
func mapDataTypeToSource(dataType string) (string, string) {
	if pair, ok := dataTypeToSource[dataType]; ok {
		return pair[0], pair[1]
	}

	// Try prefix matching (e.g. "chrome:history:whatever" -> check "chrome:history")
	parts := strings.Split(dataType, ":")
	if len(parts) >= 2 {
		prefix := parts[0] + ":" + parts[1]
		if pair, ok := dataTypeToSource[prefix]; ok {
			return pair[0], pair[1]
		}
	}

	// Fall back: derive from data_type string
	short := strings.ToUpper(parts[0])
	long := dataType

	return short, long
}

// collectExtras collects any fields not in the known set into a single string.
func collectExtras(raw map[string]interface{}) string {
	var extras []string
	for k, v := range raw {
		if !knownFields[k] {
			// Skip nested objects (like pathspec) and internal Plaso fields
			switch v.(type) {
			case map[string]interface{}:
				continue
			}
			extras = append(extras, fmt.Sprintf("%s: %v", k, v))
		}
	}
	if len(extras) > 0 {
		return strings.Join(extras, "; ")
	}
	return ""
}

// knownFields lists field names that map to specific Event model columns
// and should not appear in the Extra field.
var knownFields = map[string]bool{
	// Timestamps
	"timestamp": true, "datetime": true, "timestamp_desc": true,
	"date_time": true,
	// Source
	"source_short": true, "source_long": true, "source": true,
	"data_type": true,
	// Content
	"message": true, "display_name": true, "filename": true,
	"inode": true, "parser": true,
	// System
	"hostname": true, "username": true, "computer_name": true,
	// Event identifiers
	"event_identifier": true, "event_type": true, "source_name": true,
	"user_sid": true, "record_number": true,
	// Storage
	"store_number": true, "store_index": true, "vss_store_number": true,
	"offset": true,
	// Other mapped fields
	"url": true, "zone": true, "inode_number": true,
	"path_separator": true, "tag": true, "tag_list": true,
	// Plaso internal fields (skip entirely)
	"__container_type__": true, "__type__": true,
	// Nested objects (skip as extras)
	"pathspec": true,
}

// extractTagList converts a tag_list interface to a comma-separated string.
func extractTagList(v interface{}) string {
	if v == nil {
		return ""
	}
	if tagList, ok := v.([]interface{}); ok {
		tags := make([]string, 0, len(tagList))
		for _, t := range tagList {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
		return strings.Join(tags, ", ")
	}
	return ""
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
		return ""
	}
	return result
}

// normalizeDatetime converts various datetime string formats to "YYYY-MM-DD HH:MM:SS"
// for consistency with CSV imports and proper date range filtering.
func normalizeDatetime(dt string) string {
	if dt == "" || dt == "Not a time" {
		return dt
	}
	// Try common ISO formats and normalize to space-separated
	formats := []string{
		"2006-01-02T15:04:05+00:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.000000+00:00",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, dt); err == nil {
			return t.UTC().Format("2006-01-02 15:04:05")
		}
	}
	// Already in correct format or unknown, return as-is
	return dt
}

// getStr safely extracts a string from a raw JSON map.
func getStr(raw map[string]interface{}, key string) string {
	v, ok := raw[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return interfaceToString(v)
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
