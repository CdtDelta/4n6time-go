package importer

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cdtdelta/4n6time/internal/database"
)

// Skip reason constants for unrecognized or unprocessable files.
const (
	SkipReasonUnrecognizedFormat = "unrecognized format"
	SkipReasonEmptyFile          = "empty file"
	SkipReasonParseError         = "parse error: "
)

// ToolStats holds per-tool import metrics.
type ToolStats struct {
	FileCount  int `json:"fileCount"`
	EventCount int `json:"eventCount"`
}

// SkippedFile records a file that was not imported and the reason why.
type SkippedFile struct {
	RelativePath string `json:"relativePath"`
	Reason       string `json:"reason"`
}

// ImportSummary summarizes the outcome of a recursive folder import.
type ImportSummary struct {
	PerTool             map[string]ToolStats `json:"perTool"`
	SkippedFiles        []SkippedFile        `json:"skippedFiles"`
	TotalEvents         int                  `json:"totalEvents"`
	TotalFilesProcessed int                  `json:"totalFilesProcessed"`
	DirectoriesWalked   int                  `json:"directoriesWalked"`
	MaxDepthReached     int                  `json:"maxDepthReached"`
	// ToolFamilies maps each PerTool label to its tool family (for example
	// "LECmd" to "EZ Tools") so the summary can group tools. Labels from an
	// entry with no family are absent.
	ToolFamilies map[string]string `json:"toolFamilies"`
}

// walkEntry is one file the folder walk considered, in walk order. Either
// skipReason is set, or format and label identify how to import it.
type walkEntry struct {
	rel        string
	path       string
	depth      int
	format     *Format
	label      string
	skipReason string
}

// recursiveFormats returns the registry entries recursive import may use,
// in registry order.
func recursiveFormats() []*Format {
	var out []*Format
	for i := range Registry {
		if Registry[i].Recursive {
			out = append(out, &Registry[i])
		}
	}
	return out
}

// recursiveExtensions returns the set of file extensions recursive import
// considers, derived from the recursive registry entries.
func recursiveExtensions(formats []*Format) map[string]struct{} {
	exts := make(map[string]struct{})
	for _, f := range formats {
		for _, e := range f.Extensions {
			exts[e] = struct{}{}
		}
	}
	return exts
}

// ImportFolderRecursive walks root up to 3 directory levels deep (root = depth 0),
// detects and imports all files recognized by a Recursive registry entry, and
// returns a summary. Symlinks are skipped without error. Files whose extension
// no recursive entry claims are silently ignored. Duplicate UnifiedLog exports
// of the same data are imported once; see markUnifiedLogDuplicates.
// onProgress is called after each successfully imported file with the relative
// path and the number of events inserted; it may be nil.
func ImportFolderRecursive(root string, store database.Store, onProgress func(relPath string, eventsInserted int)) (*ImportSummary, error) {
	summary := &ImportSummary{
		PerTool:      make(map[string]ToolStats),
		ToolFamilies: make(map[string]string),
	}

	// Detection runs over the whole walk before anything is imported, because
	// duplicate resolution needs to see every UnifiedLog file first.
	entries, err := collectWalkEntries(root, summary)
	if err != nil {
		return nil, err
	}
	markUnifiedLogDuplicates(entries)

	skip := func(rel, reason string) {
		summary.SkippedFiles = append(summary.SkippedFiles, SkippedFile{
			RelativePath: rel,
			Reason:       reason,
		})
	}

	for _, e := range entries {
		if e.skipReason != "" {
			skip(e.rel, e.skipReason)
			continue
		}

		inserted, importErr := e.format.Import(e.path, store, nil)
		if importErr != nil {
			skip(e.rel, SkipReasonParseError+innerError(importErr).Error())
			continue
		}

		stats := summary.PerTool[e.label]
		stats.FileCount++
		stats.EventCount += inserted
		summary.PerTool[e.label] = stats
		if e.format.Family != "" {
			summary.ToolFamilies[e.label] = e.format.Family
		}

		summary.TotalEvents += inserted
		summary.TotalFilesProcessed++
		if e.depth > summary.MaxDepthReached {
			summary.MaxDepthReached = e.depth
		}

		if onProgress != nil {
			onProgress(e.rel, inserted)
		}
	}

	return summary, nil
}

// collectWalkEntries walks root and detects every candidate file, returning
// the files in walk order with either a detected format or a skip reason.
// It counts walked directories into summary.
func collectWalkEntries(root string, summary *ImportSummary) ([]walkEntry, error) {
	formats := recursiveFormats()
	allowedExts := recursiveExtensions(formats)

	var entries []walkEntry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip inaccessible entries
		}

		rel, _ := filepath.Rel(root, path)
		depth := depthOf(rel)

		// Skip symlinks regardless of whether they point to a file or directory.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			if depth == 0 {
				return nil // root itself; not counted
			}
			if depth >= 4 {
				return fs.SkipDir
			}
			summary.DirectoriesWalked++
			return nil
		}

		// Regular file beyond the depth limit is silently skipped.
		if depth > 3 {
			return nil
		}

		// Filter to extensions claimed by recursive formats (case-insensitive).
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := allowedExts[ext]; !ok {
			return nil
		}

		entry := walkEntry{rel: rel, path: path, depth: depth}

		// Empty files produce no useful data.
		info, err := d.Info()
		if err != nil || info.Size() == 0 {
			entry.skipReason = SkipReasonEmptyFile
			entries = append(entries, entry)
			return nil
		}

		format, label, detectErr := detectRecursive(formats, path, ext)
		switch {
		case format == nil && detectErr != nil:
			entry.skipReason = SkipReasonParseError + detectErr.Error()
		case format == nil:
			entry.skipReason = SkipReasonUnrecognizedFormat
		case format.HasTimeline != nil && !format.HasTimeline(label):
			entry.skipReason = fmt.Sprintf("no timestamp columns (recognized as %s): no timeline data to import", label)
		default:
			entry.format = format
			entry.label = label
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}
	return entries, nil
}

// detectRecursive tries each recursive format that claims ext, in registry
// order. It returns the first match, or nil with the first detection error
// seen (nil if every format simply declined the file).
func detectRecursive(formats []*Format, path, ext string) (*Format, string, error) {
	var firstErr error
	for _, f := range formats {
		if !f.hasExtension(ext) {
			continue
		}
		label, ok, err := f.Detect(path)
		if ok {
			return f, label, nil
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return nil, "", firstErr
}

// innerError strips the single "reading X:" or "inserting events:" wrapper
// that Format.Import adds, so skip reasons show the underlying parser or
// store error exactly as they did before the registry existed.
func innerError(err error) error {
	if inner := errors.Unwrap(err); inner != nil {
		return inner
	}
	return err
}

// depthOf returns the depth of a path relative to the walk root.
// The root itself ("." from filepath.Rel) returns 0.
// Direct children return 1, grandchildren return 2, and so on.
func depthOf(rel string) int {
	if rel == "." {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}
