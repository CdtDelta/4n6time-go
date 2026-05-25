package eztoolparser

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cdtdelta/4n6time/internal/database"
)

// Skip reason constants for unrecognized or unprocessable CSV files.
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
}

// ImportFolderRecursive walks root up to 3 directory levels deep (root = depth 0),
// detects and imports all recognized EZ Tool CSV files, and returns a summary.
// Symlinks are skipped without error. Non-.csv files are silently ignored.
// onProgress is called after each successfully imported file with the relative
// path and the number of events inserted; it may be nil.
func ImportFolderRecursive(root string, store database.Store, onProgress func(relPath string, eventsInserted int)) (*ImportSummary, error) {
	summary := &ImportSummary{
		PerTool: make(map[string]ToolStats),
	}

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

		// Filter to .csv files only (case-insensitive).
		if strings.ToLower(filepath.Ext(path)) != ".csv" {
			return nil
		}

		// Empty files produce no useful data.
		info, err := d.Info()
		if err != nil || info.Size() == 0 {
			summary.SkippedFiles = append(summary.SkippedFiles, SkippedFile{
				RelativePath: rel,
				Reason:       SkipReasonEmptyFile,
			})
			return nil
		}

		toolName, detectErr := DetectTool(path)
		if detectErr != nil {
			summary.SkippedFiles = append(summary.SkippedFiles, SkippedFile{
				RelativePath: rel,
				Reason:       SkipReasonParseError + detectErr.Error(),
			})
			return nil
		}
		if toolName == "" {
			summary.SkippedFiles = append(summary.SkippedFiles, SkippedFile{
				RelativePath: rel,
				Reason:       SkipReasonUnrecognizedFormat,
			})
			return nil
		}
		if _, isNoTimestamp := NoTimestampFormats[toolName]; isNoTimestamp {
			summary.SkippedFiles = append(summary.SkippedFiles, SkippedFile{
				RelativePath: rel,
				Reason:       fmt.Sprintf("no timestamp columns (recognized as %s): no timeline data to import", toolName),
			})
			return nil
		}

		result, parseErr := ReadEvents(path, nil)
		if parseErr != nil {
			summary.SkippedFiles = append(summary.SkippedFiles, SkippedFile{
				RelativePath: rel,
				Reason:       SkipReasonParseError + parseErr.Error(),
			})
			return nil
		}

		// A recognized file may expand to zero events (all timestamps empty).
		if len(result.Events) == 0 {
			stats := summary.PerTool[result.Tool]
			stats.FileCount++
			summary.PerTool[result.Tool] = stats
			summary.TotalFilesProcessed++
			if depth > summary.MaxDepthReached {
				summary.MaxDepthReached = depth
			}
			if onProgress != nil {
				onProgress(rel, 0)
			}
			return nil
		}

		inserted, insertErr := store.InsertEvents(result.Events, nil)
		if insertErr != nil {
			summary.SkippedFiles = append(summary.SkippedFiles, SkippedFile{
				RelativePath: rel,
				Reason:       fmt.Sprintf("%s%s", SkipReasonParseError, insertErr.Error()),
			})
			return nil
		}

		stats := summary.PerTool[result.Tool]
		stats.FileCount++
		stats.EventCount += inserted
		summary.PerTool[result.Tool] = stats

		summary.TotalEvents += inserted
		summary.TotalFilesProcessed++
		if depth > summary.MaxDepthReached {
			summary.MaxDepthReached = depth
		}

		if onProgress != nil {
			onProgress(rel, inserted)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return summary, nil
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

