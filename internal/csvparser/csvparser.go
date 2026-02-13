package csvparser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cdtdelta/4n6time/internal/model"
)

// L2T CSV header as defined in the original log2timeline format.
// Column order matters: the index positions are used for field mapping.
var l2tHeader = []string{
	"date", "time", "timezone", "MACB", "source",
	"sourcetype", "type", "user", "host", "short", "desc", "version",
	"filename", "inode", "notes", "format", "extra",
}

// Export header for writing events back to CSV.
var exportHeader = []string{
	"datetime", "timezone", "MACB", "source", "sourcetype", "type",
	"user", "host", "desc", "filename", "inode", "notes", "format",
	"extra", "reportnotes", "inreport", "tag", "color",
	"offset", "store_number", "store_index", "vss_store_number", "bookmark",
}

// ReadResult contains the outcome of a CSV import operation.
type ReadResult struct {
	Events   []*model.Event
	Count    int
	Excluded int
}

// ValidateHeader checks if a CSV file has a valid L2T header.
// Returns an error describing the mismatch if validation fails.
func ValidateHeader(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(newNullStripper(f))
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	if len(header) < len(l2tHeader) {
		return fmt.Errorf("header too short: got %d columns, expected at least %d", len(header), len(l2tHeader))
	}

	for i, expected := range l2tHeader {
		if header[i] != expected {
			return fmt.Errorf("header mismatch at column %d: expected '%s', got '%s'", i, expected, header[i])
		}
	}

	return nil
}

// ReadEvents reads all events from an L2T CSV file.
// Optionally filters by date range (pass empty strings to skip filtering).
// Optionally limits the number of events (pass 0 for no limit).
// An onProgress callback is called every 10,000 events if non-nil.
func ReadEvents(path string, dateFrom, dateTo string, limit int, onProgress func(count int)) (*ReadResult, error) {
	if err := ValidateHeader(path); err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(newNullStripper(f))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // allow variable field counts

	// Skip header
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	filterByDate := dateFrom != "" && dateTo != ""
	result := &ReadResult{}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row %d: %w", result.Count+result.Excluded+1, err)
		}

		if limit > 0 && result.Count >= limit {
			break
		}

		// Date filtering (compare reformatted date against range)
		if filterByDate {
			date := reformatDate(safeIndex(row, 0))
			if date <= dateFrom || date >= dateTo {
				result.Excluded++
				continue
			}
		}

		event := rowToEvent(row)
		result.Events = append(result.Events, event)
		result.Count++

		if onProgress != nil && result.Count%10000 == 0 {
			onProgress(result.Count)
		}
	}

	return result, nil
}

// WriteEvents writes events to a CSV file in 4n6time export format.
func WriteEvents(path string, events []*model.Event) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Write header
	if err := writer.Write(exportHeader); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	for _, e := range events {
		row := []string{
			e.Datetime,
			e.Timezone,
			e.MACB,
			e.Source,
			e.SourceType,
			e.Type,
			e.User,
			e.Host,
			e.Desc,
			e.Filename,
			e.Inode,
			e.Notes,
			e.Format,
			e.Extra,
			e.ReportNotes,
			e.InReport,
			e.Tag,
			e.Color,
			fmt.Sprintf("%d", e.Offset),
			fmt.Sprintf("%d", e.StoreNumber),
			fmt.Sprintf("%d", e.StoreIndex),
			fmt.Sprintf("%d", e.VSSStoreNumber),
			fmt.Sprintf("%d", e.Bookmark),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("writing row: %w", err)
		}
	}

	return nil
}

// ColorCoding represents a mapping from a value (type or host) to a color name.
type ColorCoding struct {
	Field   string            // "type" or "host"
	Mapping map[string]string // value -> color
}

// ReadColorCoding reads a color coding template CSV.
// Expected format: header row with (type|host, colorcode), then data rows.
func ReadColorCoding(path string) (*ColorCoding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	if len(header) < 2 || (header[0] != "type" && header[0] != "host") || header[1] != "colorcode" {
		return nil, fmt.Errorf("invalid color coding header: expected (type|host, colorcode), got %v", header)
	}

	cc := &ColorCoding{
		Field:   header[0],
		Mapping: make(map[string]string),
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}

		if len(row) < 2 || row[1] == "" {
			cc.Mapping[row[0]] = "WHITE"
		} else {
			cc.Mapping[row[0]] = row[1]
		}
	}

	return cc, nil
}

// WriteColorCoding writes a color coding template to CSV.
func WriteColorCoding(path string, fieldName string, mapping map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if err := writer.Write([]string{fieldName, "colorcode"}); err != nil {
		return err
	}

	for key, color := range mapping {
		if err := writer.Write([]string{key, color}); err != nil {
			return err
		}
	}

	return nil
}

// SavedQueryEntry represents a single row in a saved query CSV file.
type SavedQueryEntry struct {
	Name        string
	SQL         string
	Description string
	EID         string
	OS          string
	IP          string
}

// ReadSavedQueries reads saved queries from a CSV file.
// Expected header: Name, SQL, Description, EID, OS, IP
func ReadSavedQueries(path string) ([]SavedQueryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	if len(header) < 6 ||
		header[0] != "Name" || header[1] != "SQL" || header[2] != "Description" ||
		header[3] != "EID" || header[4] != "OS" || header[5] != "IP" {
		return nil, fmt.Errorf("invalid saved query header: expected [Name, SQL, Description, EID, OS, IP]")
	}

	var queries []SavedQueryEntry
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}

		queries = append(queries, SavedQueryEntry{
			Name:        safeIndex(row, 0),
			SQL:         safeIndex(row, 1),
			Description: safeIndex(row, 2),
			EID:         safeIndex(row, 3),
			OS:          safeIndex(row, 4),
			IP:          safeIndex(row, 5),
		})
	}

	return queries, nil
}

// WriteSavedQueries writes saved queries to a CSV file.
func WriteSavedQueries(path string, queries []SavedQueryEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if err := writer.Write([]string{"Name", "SQL", "Description", "EID", "OS", "IP"}); err != nil {
		return err
	}

	for _, q := range queries {
		if err := writer.Write([]string{q.Name, q.SQL, q.Description, q.EID, q.OS, q.IP}); err != nil {
			return err
		}
	}

	return nil
}

// rowToEvent converts a CSV row (L2T format) into an Event.
// Column mapping matches the original Python csvmanager.py:
//
//	0=date, 1=time, 2=timezone, 3=MACB, 4=source, 5=sourcetype,
//	6=type, 7=user, 8=host, 9=short, 10=desc, 11=version,
//	12=filename, 13=inode, 14=notes, 15=format, 16=extra
func rowToEvent(row []string) *model.Event {
	return &model.Event{
		Datetime:       reformatDate(safeIndex(row, 0)) + " " + reformatTime(safeIndex(row, 1)),
		Timezone:       safeIndex(row, 2),
		MACB:           safeIndex(row, 3),
		Source:         safeIndex(row, 4),
		SourceType:     safeIndex(row, 5),
		Type:           safeIndex(row, 6),
		User:           safeIndex(row, 7),
		Host:           safeIndex(row, 8),
		Desc:           safeIndex(row, 10),
		Filename:       safeIndex(row, 12),
		Inode:          safeIndex(row, 13),
		Notes:          safeIndex(row, 14),
		Format:         safeIndex(row, 15),
		Extra:          safeIndex(row, 16),
		ReportNotes:    "",
		InReport:       "",
		Tag:            "",
		Color:          "",
		Offset:         -1,
		StoreNumber:    -1,
		StoreIndex:     -1,
		VSSStoreNumber: -1,
	}
}

// safeIndex returns the value at index i, or empty string if out of bounds.
func safeIndex(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

// reformatDate converts MM/DD/YYYY to YYYY-MM-DD.
// Returns "0000-00-00" if the format doesn't match.
func reformatDate(dateStr string) string {
	t, err := time.Parse("01/02/2006", dateStr)
	if err != nil {
		// Also try YYYY-MM-DD in case it's already in the right format
		if _, err2 := time.Parse("2006-01-02", dateStr); err2 == nil {
			return dateStr
		}
		return "0000-00-00"
	}
	return t.Format("2006-01-02")
}

// reformatTime validates HH:MM:SS format.
// Returns "00:00:00" if the format doesn't match.
func reformatTime(timeStr string) string {
	_, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		return "00:00:00"
	}
	return timeStr
}

// nullStripper wraps a reader and strips null bytes from the stream.
// The original Python code does this to prevent csv.reader errors.
type nullStripper struct {
	r io.Reader
}

func newNullStripper(r io.Reader) io.Reader {
	return &nullStripper{r: r}
}

func (ns *nullStripper) Read(p []byte) (int, error) {
	n, err := ns.r.Read(p)
	if n > 0 {
		// Replace null bytes in place
		cleaned := strings.ReplaceAll(string(p[:n]), "\x00", "")
		copy(p, cleaned)
		n = len(cleaned)
	}
	return n, err
}
