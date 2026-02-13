package tlnparser

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cdtdelta/4n6time/internal/model"
)

// ReadResult contains the outcome of a TLN import operation.
type ReadResult struct {
	Events   []*model.Event
	Count    int
	Excluded int
	Format   string // "TLN" or "L2TTLN"
}

// ValidateFile checks if a file is a valid TLN or L2TTLN file.
// Returns an error if the file cannot be parsed.
func ValidateFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return fmt.Errorf("empty file")
	}

	header := scanner.Text()
	header = strings.TrimSpace(header)

	if header == "Time|Source|Host|User|Description|TZ|Notes" {
		return nil // L2TTLN
	}
	if header == "Time|Source|Host|User|Description" {
		return nil // TLN
	}

	// Check if first line looks like data (no header)
	parts := strings.Split(header, "|")
	if len(parts) == 5 || len(parts) == 7 {
		// First field should be a numeric timestamp
		if _, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			return nil
		}
	}

	return fmt.Errorf("not a valid TLN/L2TTLN file: expected 5 or 7 pipe-delimited fields, got %d", len(parts))
}

// ReadEvents reads events from a TLN or L2TTLN file.
// Auto-detects the format based on header or field count.
func ReadEvents(path string, onProgress func(int)) (*ReadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Increase buffer for potentially long description lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	result := &ReadResult{}
	lineNum := 0
	fieldCount := 0
	hasHeader := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++

		if line == "" {
			continue
		}

		// Detect format from first line
		if fieldCount == 0 {
			if line == "Time|Source|Host|User|Description|TZ|Notes" {
				result.Format = "L2TTLN"
				fieldCount = 7
				hasHeader = true
				continue
			}
			if line == "Time|Source|Host|User|Description" {
				result.Format = "TLN"
				fieldCount = 5
				hasHeader = true
				continue
			}

			// No header, detect from field count
			parts := strings.Split(line, "|")
			if len(parts) == 7 {
				result.Format = "L2TTLN"
				fieldCount = 7
			} else if len(parts) == 5 {
				result.Format = "TLN"
				fieldCount = 5
			} else {
				return nil, fmt.Errorf("line %d: expected 5 or 7 pipe-delimited fields, got %d", lineNum, len(parts))
			}
			// Fall through to parse this line as data if no header
		}

		// Skip header if we already processed it
		if hasHeader && lineNum == 1 {
			continue
		}

		parts := strings.SplitN(line, "|", fieldCount)
		if len(parts) < fieldCount {
			// Tolerate short lines by padding
			for len(parts) < fieldCount {
				parts = append(parts, "")
			}
		}

		event, err := parseTLNLine(parts, fieldCount)
		if err != nil {
			result.Excluded++
			continue
		}

		result.Events = append(result.Events, event)
		result.Count++

		if onProgress != nil && result.Count%10000 == 0 {
			onProgress(result.Count)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return result, nil
}

// parseTLNLine parses a single TLN or L2TTLN line into an Event.
// TLN fields:    Time|Source|Host|User|Description
// L2TTLN fields: Time|Source|Host|User|Description|TZ|Notes
func parseTLNLine(parts []string, fieldCount int) (*model.Event, error) {
	e := &model.Event{}

	// Time: Unix epoch seconds
	epoch, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %s", parts[0])
	}

	if epoch > 0 {
		t := time.Unix(epoch, 0).UTC()
		e.Datetime = t.Format("2006-01-02 15:04:05")
	} else {
		e.Datetime = "Not a time"
	}

	// Source
	e.Source = strings.TrimSpace(parts[1])
	e.Format = e.Source

	// Host
	e.Host = strings.TrimSpace(parts[2])

	// User
	e.User = strings.TrimSpace(parts[3])

	// Description: in TLN format this is a composite "datetime; timestamp_desc; message"
	desc := strings.TrimSpace(parts[4])
	e.Desc = desc

	// Try to extract MACB/type from description if it contains semicolons
	// TLN description format: "datetime; timestamp_desc; message"
	descParts := strings.SplitN(desc, ";", 3)
	if len(descParts) >= 2 {
		tsDesc := strings.TrimSpace(descParts[1])
		e.Type = tsDesc
		e.MACB = mapTimestampDescToMACB(tsDesc)
		if len(descParts) == 3 {
			e.Desc = strings.TrimSpace(descParts[2])
		}
	}

	// Timezone
	e.Timezone = "UTC"

	// L2TTLN extra fields
	if fieldCount == 7 {
		tz := strings.TrimSpace(parts[5])
		if tz != "" && tz != "-" {
			e.Timezone = tz
		}

		notes := strings.TrimSpace(parts[6])
		if notes != "" && notes != "-" {
			e.Notes = notes
			// Extract filename from "File: /path/to/file inode: 12345"
			if strings.HasPrefix(notes, "File: ") {
				filePart := strings.TrimPrefix(notes, "File: ")
				if idx := strings.Index(filePart, " inode: "); idx >= 0 {
					e.Filename = filePart[:idx]
					e.Inode = filePart[idx+8:]
				} else {
					e.Filename = filePart
				}
			}
		}
	}

	return e, nil
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
