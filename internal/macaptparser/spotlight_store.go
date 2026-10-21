package macaptparser

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/cdtdelta/4n6time/internal/model"
)

// Spotlight store CSVs have a column set that varies by store, so they cannot
// match an exact header. They are recognized by file name and key columns, and
// each file's header sets its timestamp and extra columns.
const (
	spotlightStoreLabel      = "mac_apt Spotlight store"
	spotlightStoreSource     = "SPOTLIGHT"
	spotlightStoreFilePrefix = "Spotlight-"
	spotlightStoreTypePrefix = "mac_apt Spotlight "
	spotlightRecordUpdated   = "Date_Updated"
	spotlightRecordType      = "Spotlight record updated"
	spotlightExpiration      = "_kMDItemExpirationDate"
	spotlightBundleID        = "_kMDItemBundleID"
	spotlightContentURL      = "kMDItemContentURL"
	spotlightRecordID        = "ID"
)

// spotlightKeyColumns are the first six columns of every store CSV. With the
// file name prefix, they identify a store.
var spotlightKeyColumns = []string{"ID", "ID_hex", "Flags", "Parent_ID", "Parent_ID_hex", "Date_Updated"}

// spotlightDescColumns are the columns a store item's description can come
// from, in order of preference.
var spotlightDescColumns = []string{"kMDItemDisplayName", "_kMDItemFileName", "kMDItemTitle", spotlightContentURL}

// stripNULNames removes NUL bytes from header names, the same way the CSV
// reader's NUL stripping does for the rows.
func stripNULNames(header []string) []string {
	out := make([]string, len(header))
	for i, h := range header {
		out[i] = stripNUL(h)
	}
	return out
}

// isSpotlightStore reports whether a CSV named name with header is a Spotlight
// store. Both the file name prefix and the key columns must match.
func isSpotlightStore(name string, header []string) bool {
	if !strings.HasPrefix(name, spotlightStoreFilePrefix) {
		return false
	}
	if len(header) < len(spotlightKeyColumns) {
		return false
	}
	for i, c := range spotlightKeyColumns {
		if header[i] != c {
			return false
		}
	}
	return true
}

// outputForFile returns the output a CSV file belongs to. Exact header matches
// come first. The Spotlight store rule runs only after they all fail, so the
// fixed Spotlight outputs are never mistaken for a store.
func outputForFile(path string, header []string) *output {
	if o := outputForHeader(header); o != nil {
		return o
	}
	clean := stripNULNames(header)
	if isSpotlightStore(filepath.Base(path), clean) {
		return spotlightStoreOutput(filepath.Base(path), clean)
	}
	return nil
}

// spotlightStoreOutput builds the output for one store file. Its columns are
// the file's own header, so the exact-header check in eachCSVRow passes.
func spotlightStoreOutput(name string, header []string) *output {
	s := newSpotlightStore(name, header)
	return &output{
		label:    spotlightStoreLabel,
		columns:  header,
		mapRow:   s.mapRow,
		stripNUL: true,
		cells:    s.cells,
	}
}

// spotlightStore holds what one store file's rows are mapped with.
type spotlightStore struct {
	header     []string
	sourceType string
	stamps     []string
	stampSet   map[string]bool
}

// newSpotlightStore reads the timestamp columns out of header. A column is a
// timestamp under isSpotlightTimestamp.
func newSpotlightStore(name string, header []string) *spotlightStore {
	s := &spotlightStore{
		header:     header,
		sourceType: spotlightStoreTypePrefix + storeName(name),
		stampSet:   make(map[string]bool),
	}
	for _, col := range header {
		if isSpotlightTimestamp(col) {
			s.stamps = append(s.stamps, col)
			s.stampSet[col] = true
		}
	}
	return s
}

// storeName returns the store part of "Spotlight-<store>.csv".
func storeName(name string) string {
	n := strings.TrimPrefix(name, spotlightStoreFilePrefix)
	if strings.EqualFold(filepath.Ext(n), ".csv") {
		n = strings.TrimSuffix(n, filepath.Ext(n))
	}
	return n
}

// isSpotlightTimestamp reports whether a header column holds event times:
// Date_Updated, or a column ending in Date, Dates, or DateAdded. The expiration
// date and ranking scores are never event times, even when their names match.
func isSpotlightTimestamp(col string) bool {
	if col == spotlightRecordUpdated {
		return true
	}
	if col == spotlightExpiration || strings.HasSuffix(col, "_Ranking") {
		return false
	}
	return strings.HasSuffix(col, "Date") ||
		strings.HasSuffix(col, "Dates") ||
		strings.HasSuffix(col, "DateAdded")
}

// cells lists a row's non-empty timestamp values. A column ending in Dates is
// a comma-separated list, so each value in it is its own cell.
func (s *spotlightStore) cells(row map[string]string) []timeCell {
	var cells []timeCell
	for _, col := range s.stamps {
		var values []string
		if strings.HasSuffix(col, "Dates") {
			values = strings.Split(row[col], ",")
		} else {
			values = []string{row[col]}
		}
		for _, v := range values {
			if v = strings.TrimSpace(v); v != "" {
				cells = append(cells, timeCell{column: col, value: v})
			}
		}
	}
	return cells
}

// describe returns the item's description and the column it came from. An item
// with none of the description columns is named by its ID.
func (s *spotlightStore) describe(row map[string]string) (string, string) {
	for _, c := range spotlightDescColumns {
		if v := strings.TrimSpace(row[c]); v != "" {
			return v, c
		}
	}
	return joinNonEmpty(" ", "Spotlight item", row[spotlightRecordID]), ""
}

// skipExtra reports whether a column stays out of extra. Mapped columns,
// timestamps, embeddings, ranking scores, and derived date fragments such as
// _kMDItemStartDateHour stay out.
func (s *spotlightStore) skipExtra(col, descCol string) bool {
	switch {
	case s.stampSet[col]:
		return true
	case col == spotlightRecordID, col == spotlightBundleID, col == spotlightContentURL:
		return true
	case col == descCol:
		return true
	case col == spotlightExpiration:
		return true
	case strings.Contains(col, "Embedding"):
		return true
	case strings.HasSuffix(col, "_Ranking"):
		return true
	case strings.Contains(col, "Date"):
		return true
	}
	return false
}

// extraFields lists the columns that go in extra, in header order. Empty
// values are dropped later by renderExtra.
func (s *spotlightStore) extraFields(row map[string]string, descCol string) []field {
	var out []field
	for _, col := range s.header {
		if s.skipExtra(col, descCol) {
			continue
		}
		out = append(out, field{col, row[col]})
	}
	return out
}

// mapRow maps one store row to one event per timestamp value that parses. Each
// event's extra leads with its own timestamp value, as the other outputs do.
// Store CSVs have no Source column, so the filename stays empty.
func (s *spotlightStore) mapRow(row map[string]string) ([]*model.Event, int) {
	desc, descCol := s.describe(row)
	var events []*model.Event
	skipped := 0
	for _, c := range s.cells(row) {
		when, ok, bad := parseColumnTimestamp(c.value)
		skipped += bad
		if !ok {
			continue
		}
		e := newEvent(spotlightStoreSource, s.sourceType, when, "")
		e.Type = c.column
		if c.column == spotlightRecordUpdated {
			e.Type = spotlightRecordType
		}
		e.Desc = desc
		e.RecordNumber = row[spotlightRecordID]
		e.EventType = row[spotlightBundleID]
		e.URL = row[spotlightContentURL]
		e.Extra = renderExtra(append([]field{{c.column, c.value}}, s.extraFields(row, descCol)...)...)
		events = append(events, e)
	}
	return events, skipped
}

// nulStrippingReader drops NUL bytes as they stream past, before the CSV
// parser sees them, so a NUL inside a quoted field cannot change quoting.
type nulStrippingReader struct {
	r io.Reader
}

func (n nulStrippingReader) Read(p []byte) (int, error) {
	for {
		k, err := n.r.Read(p)
		w := 0
		for _, b := range p[:k] {
			if b != 0 {
				p[w] = b
				w++
			}
		}
		if w > 0 || err != nil || k == 0 {
			return w, err
		}
	}
}
