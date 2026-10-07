// Package importer holds the ordered registry of timeline import formats.
// Single-file import and recursive folder import both select a parser by
// walking the same registry, so detection order is defined in one place.
package importer

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cdtdelta/4n6time/internal/csvparser"
	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/dynamicparser"
	"github.com/cdtdelta/4n6time/internal/eztoolparser"
	"github.com/cdtdelta/4n6time/internal/jsonlparser"
	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/tlnparser"
)

// Format display names. These also appear in log lines and progress messages.
const (
	FormatJSONL      = "JSONL"
	FormatTLN        = "TLN"
	FormatEZTools    = "EZ Tools CSV"
	FormatL2TCSV     = "CSV"
	FormatDynamicCSV = "Dynamic CSV"
)

// ErrUnrecognizedFormat is returned by DetectFile when no registry entry
// accepts the file.
var ErrUnrecognizedFormat = errors.New("unrecognized file format: not a valid L2T CSV, JSONL, TLN, EZ Tools CSV, or dynamic CSV file")

// Progress carries optional import callbacks. A nil *Progress, or any nil
// field, disables that callback.
type Progress struct {
	// Reading is called periodically while the parser reads the file.
	Reading func(count int)
	// Status reports a format-specific message after reading completes.
	Status func(message string, count int)
	// InsertStart is called once before database insertion begins.
	InsertStart func(total int)
	// Inserting is called periodically during database insertion.
	Inserting func(count, total int)
}

func (p *Progress) reading() func(int) {
	if p == nil {
		return nil
	}
	return p.Reading
}

func (p *Progress) status(message string, count int) {
	if p != nil && p.Status != nil {
		p.Status(message, count)
	}
}

func (p *Progress) insertStart(total int) {
	if p != nil && p.InsertStart != nil {
		p.InsertStart(total)
	}
}

func (p *Progress) inserting(total int) func(int) {
	if p == nil || p.Inserting == nil {
		return nil
	}
	return func(count int) { p.Inserting(count, total) }
}

// Format describes one importable timeline format.
type Format struct {
	// Name is the display name of the format.
	Name string
	// Extensions lists lowercase extensions (with dot) associated with the
	// format. Recursive import only considers files with these extensions.
	// Single-file import does not filter on them unless Exclusive is set,
	// which preserves the historic behavior of trying CSV-family formats on
	// files with any extension.
	Extensions []string
	// Exclusive means a file whose extension is in Extensions can only be
	// this format; single-file import fails instead of trying later entries
	// when Detect rejects it.
	Exclusive bool
	// Detect reports whether path is this format. label is the per-tool name
	// shown in the recursive import summary. A non-nil err means detection
	// itself failed (unreadable file, malformed header, invalid content).
	Detect func(path string) (label string, ok bool, err error)
	// HasTimeline reports whether a detected label produces timeline events.
	// Recursive import skips labels for which it returns false. nil means
	// every detected label has a timeline.
	HasTimeline func(label string) bool
	// Import reads path and inserts its events into store, returning the
	// number of events inserted.
	Import func(path string, store database.Store, p *Progress) (int, error)
	// Recursive reports whether recursive folder import may use this entry.
	Recursive bool
}

func (f *Format) hasExtension(ext string) bool {
	for _, e := range f.Extensions {
		if e == ext {
			return true
		}
	}
	return false
}

// Registry is the ordered list of import formats. Order matters: the first
// entry whose Detect accepts a file wins. Dynamic CSV accepts almost any CSV,
// so it must stay last and must never be Recursive, otherwise every
// unrecognized CSV in a folder walk would import instead of being skipped.
var Registry = []Format{
	{
		Name:       FormatJSONL,
		Extensions: []string{".jsonl", ".json"},
		Exclusive:  true,
		Detect:     detectJSONL,
		Import:     importJSONL,
	},
	{
		Name:       FormatTLN,
		Extensions: []string{".tln", ".l2ttln"},
		Exclusive:  true,
		Detect:     detectTLN,
		Import:     importTLN,
	},
	{
		Name:        FormatEZTools,
		Extensions:  []string{".csv"},
		Detect:      detectEZTools,
		HasTimeline: ezToolsHasTimeline,
		Import:      importEZTools,
		Recursive:   true,
	},
	{
		Name:       FormatL2TCSV,
		Extensions: []string{".csv"},
		Detect:     detectL2TCSV,
		Import:     importL2TCSV,
	},
	{
		Name:       FormatDynamicCSV,
		Extensions: []string{".csv"},
		Detect:     detectDynamicCSV,
		Import:     importDynamicCSV,
	},
}

// DetectFile walks the registry in order and returns the first format that
// accepts path, along with its detection label.
func DetectFile(path string) (*Format, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	for i := range Registry {
		f := &Registry[i]
		label, ok, err := f.Detect(path)
		if ok {
			return f, label, nil
		}
		if f.Exclusive && f.hasExtension(ext) {
			if err == nil {
				err = errors.New("format not detected")
			}
			return nil, "", fmt.Errorf("invalid %s file: %w", f.Name, err)
		}
	}
	return nil, "", ErrUnrecognizedFormat
}

// insertEvents hands parsed events to the store and reports progress.
func insertEvents(store database.Store, events []*model.Event, p *Progress) (int, error) {
	total := len(events)
	p.insertStart(total)
	if total == 0 {
		return 0, nil
	}
	inserted, err := store.InsertEvents(events, p.inserting(total))
	if err != nil {
		return 0, fmt.Errorf("inserting events: %w", err)
	}
	return inserted, nil
}

// --- JSONL ---

func detectJSONL(path string) (string, bool, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".jsonl" && ext != ".json" {
		return "", false, nil
	}
	if err := jsonlparser.ValidateFile(path); err != nil {
		return "", false, err
	}
	return FormatJSONL, true, nil
}

func importJSONL(path string, store database.Store, p *Progress) (int, error) {
	result, err := jsonlparser.ReadEvents(path, p.reading())
	if err != nil {
		return 0, fmt.Errorf("reading JSONL: %w", err)
	}
	return insertEvents(store, result.Events, p)
}

// --- TLN / L2TTLN ---

func detectTLN(path string) (string, bool, error) {
	if err := tlnparser.ValidateFile(path); err != nil {
		return "", false, err
	}
	return FormatTLN, true, nil
}

func importTLN(path string, store database.Store, p *Progress) (int, error) {
	result, err := tlnparser.ReadEvents(path, p.reading())
	if err != nil {
		return 0, fmt.Errorf("reading TLN: %w", err)
	}
	return insertEvents(store, result.Events, p)
}

// --- EZ Tools CSV ---

func detectEZTools(path string) (string, bool, error) {
	tool, err := eztoolparser.DetectTool(path)
	if err != nil {
		return "", false, err
	}
	if tool == "" {
		return "", false, nil
	}
	return tool, true, nil
}

func ezToolsHasTimeline(tool string) bool {
	_, noTimestamp := eztoolparser.NoTimestampFormats[tool]
	return !noTimestamp
}

func importEZTools(path string, store database.Store, p *Progress) (int, error) {
	result, err := eztoolparser.ReadEvents(path, p.reading())
	if err != nil {
		return 0, fmt.Errorf("reading EZ Tools CSV: %w", err)
	}
	p.status(fmt.Sprintf("Importing %s data...", result.Tool), result.Count)
	return insertEvents(store, result.Events, p)
}

// --- L2T CSV ---

func detectL2TCSV(path string) (string, bool, error) {
	if err := csvparser.ValidateHeader(path); err != nil {
		return "", false, err
	}
	return FormatL2TCSV, true, nil
}

func importL2TCSV(path string, store database.Store, p *Progress) (int, error) {
	result, err := csvparser.ReadEvents(path, "", "", 0, p.reading())
	if err != nil {
		return 0, fmt.Errorf("reading CSV: %w", err)
	}
	return insertEvents(store, result.Events, p)
}

// --- Dynamic CSV ---

func detectDynamicCSV(path string) (string, bool, error) {
	if err := dynamicparser.ValidateFile(path); err != nil {
		return "", false, err
	}
	return FormatDynamicCSV, true, nil
}

func importDynamicCSV(path string, store database.Store, p *Progress) (int, error) {
	result, err := dynamicparser.ReadEvents(path, p.reading())
	if err != nil {
		return 0, fmt.Errorf("reading dynamic CSV: %w", err)
	}
	return insertEvents(store, result.Events, p)
}
