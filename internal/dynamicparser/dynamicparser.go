package dynamicparser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cdtdelta/4n6time/internal/model"
)

// ReadResult contains the outcome of a dynamic CSV import operation.
type ReadResult struct {
	Events   []*model.Event
	Count    int
	Excluded int
}

// Known Plaso dynamic format field names and their aliases.
// Maps possible header names to our internal field names.
var fieldAliases = map[string]string{
	"datetime":        "datetime",
	"date":            "datetime",
	"timestamp_desc":  "type",
	"type":            "type",
	"source":          "source",
	"source_short":    "source",
	"sourcetype":      "sourcetype",
	"source_long":     "sourcetype",
	"message":         "desc",
	"desc":            "desc",
	"short":           "desc",
	"description":     "desc",
	"parser":          "format",
	"format":          "format",
	"display_name":    "filename",
	"filename":        "filename",
	"host":            "host",
	"hostname":        "host",
	"user":            "user",
	"username":        "user",
	"macb":            "macb",
	"tag":             "tag",
	"inode":           "inode",
	"timezone":        "timezone",
	"zone":            "timezone",
	"tz":              "timezone",
	"notes":           "notes",
	"extra":           "extra",
	"url":             "url",
	"record_number":   "record_number",
	"event_identifier": "event_identifier",
	"event_type":      "event_type",
	"source_name":     "source_name",
	"user_sid":        "user_sid",
	"computer_name":   "computer_name",
}

// ValidateFile checks if a file has a header row with at least one recognized
// Plaso dynamic output field. Returns an error if not recognized.
func ValidateFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	recognized := 0
	for _, col := range header {
		col = strings.TrimSpace(strings.ToLower(col))
		if _, ok := fieldAliases[col]; ok {
			recognized++
		}
	}

	if recognized == 0 {
		return fmt.Errorf("no recognized Plaso fields in header (found: %s)", strings.Join(header, ", "))
	}

	return nil
}

// ReadEvents reads events from a dynamic CSV file.
// The header row determines which fields are present and their mapping.
func ReadEvents(path string, onProgress func(int)) (*ReadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // Allow variable field counts

	// Read and parse header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Build column index to field mapping
	colMap := buildColumnMap(header)
	if len(colMap) == 0 {
		return nil, fmt.Errorf("no recognized fields in header")
	}

	result := &ReadResult{}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows
			result.Excluded++
			continue
		}

		event := rowToEvent(row, colMap, header)
		result.Events = append(result.Events, event)
		result.Count++

		if onProgress != nil && result.Count%10000 == 0 {
			onProgress(result.Count)
		}
	}

	return result, nil
}

// columnMapping maps a column index to an internal field name.
type columnMapping struct {
	index     int
	fieldName string
}

// buildColumnMap creates a mapping from column indices to field names.
func buildColumnMap(header []string) []columnMapping {
	var mappings []columnMapping
	seen := make(map[string]bool)

	for i, col := range header {
		col = strings.TrimSpace(strings.ToLower(col))
		if fieldName, ok := fieldAliases[col]; ok {
			// Avoid duplicate mappings (first one wins)
			if !seen[fieldName] {
				seen[fieldName] = true
				mappings = append(mappings, columnMapping{index: i, fieldName: fieldName})
			}
		}
	}

	return mappings
}

// rowToEvent converts a CSV row to an Event using the column mapping.
// Unmapped columns are collected into the Extra field.
func rowToEvent(row []string, colMap []columnMapping, header []string) *model.Event {
	e := &model.Event{}

	// Track which columns are mapped
	mapped := make(map[int]bool)

	for _, cm := range colMap {
		if cm.index >= len(row) {
			continue
		}
		val := strings.TrimSpace(row[cm.index])
		mapped[cm.index] = true

		switch cm.fieldName {
		case "datetime":
			e.Datetime = normalizeDatetime(val)
		case "type":
			e.Type = val
			if e.MACB == "" {
				e.MACB = mapTimestampDescToMACB(val)
			}
		case "source":
			e.Source = val
		case "sourcetype":
			e.SourceType = val
		case "desc":
			e.Desc = val
		case "format":
			e.Format = val
		case "filename":
			e.Filename = val
		case "host":
			e.Host = val
		case "user":
			e.User = val
		case "macb":
			e.MACB = val
		case "tag":
			e.Tag = val
		case "inode":
			e.Inode = val
		case "timezone":
			e.Timezone = val
		case "notes":
			e.Notes = val
		case "extra":
			e.Extra = val
		case "url":
			e.URL = val
		case "record_number":
			e.RecordNumber = val
		case "event_identifier":
			e.EventID = val
		case "event_type":
			e.EventType = val
		case "source_name":
			e.SourceName = val
		case "user_sid":
			e.UserSID = val
		case "computer_name":
			e.ComputerName = val
		}
	}

	// Set defaults
	if e.Timezone == "" {
		e.Timezone = "UTC"
	}

	// Collect unmapped columns into Extra
	var extras []string
	for i, val := range row {
		if !mapped[i] && strings.TrimSpace(val) != "" && strings.TrimSpace(val) != "-" {
			colName := "unknown"
			if i < len(header) {
				colName = strings.TrimSpace(header[i])
			}
			extras = append(extras, colName+": "+strings.TrimSpace(val))
		}
	}
	if len(extras) > 0 && e.Extra == "" {
		e.Extra = strings.Join(extras, "; ")
	}

	return e
}

// normalizeDatetime converts various datetime formats to "YYYY-MM-DD HH:MM:SS".
func normalizeDatetime(dt string) string {
	if dt == "" || dt == "-" || dt == "0000-00-00T00:00:00+00:00" {
		return ""
	}

	// Already in correct format
	if len(dt) == 19 && dt[4] == '-' && dt[10] == ' ' {
		return dt
	}

	// Replace T separator with space and strip timezone suffix
	if strings.Contains(dt, "T") {
		// Handle formats like 2018-10-09T16:00:00+00:00 or 2018-10-09T16:00:00Z
		dt = strings.Replace(dt, "T", " ", 1)
		// Strip timezone offset
		if idx := strings.Index(dt, "+"); idx > 0 && idx > 10 {
			dt = dt[:idx]
		} else if idx := strings.Index(dt, "Z"); idx > 0 && idx > 10 {
			dt = dt[:idx]
		}
		// Strip fractional seconds
		if idx := strings.Index(dt, "."); idx > 0 && idx > 10 {
			dt = dt[:idx]
		}
	}

	// Trim to 19 chars if longer
	if len(dt) > 19 {
		dt = dt[:19]
	}

	return dt
}

// mapTimestampDescToMACB maps a timestamp description to MACB notation.
func mapTimestampDescToMACB(tsDesc string) string {
	lower := strings.ToLower(tsDesc)
	macb := [4]byte{'.', '.', '.', '.'}

	if strings.Contains(lower, "modification") || strings.Contains(lower, "modified") ||
		strings.Contains(lower, "written") {
		macb[0] = 'M'
	}
	if strings.Contains(lower, "access") {
		macb[1] = 'A'
	}
	if strings.Contains(lower, "change") || strings.Contains(lower, "metadata") ||
		strings.Contains(lower, "entry") || strings.Contains(lower, "mft") {
		macb[2] = 'C'
	}
	if strings.Contains(lower, "creation") || strings.Contains(lower, "birth") ||
		strings.Contains(lower, "created") {
		macb[3] = 'B'
	}

	return string(macb[:])
}
