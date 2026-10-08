package importer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cdtdelta/4n6time/internal/unifiedlogparser"
)

// markUnifiedLogDuplicates finds UnifiedLog files in entries that hold the
// same log data and marks all but one of each group as skipped. Collections
// often include both the CSV and JSONL output of one unifiedlog_iterator run,
// and importing both would double every event.
//
// Two rules group duplicates, in order:
//  1. A .csv and a .jsonl in the same directory with the same base name.
//  2. Among files still being imported after rule 1, identical first-record
//     fingerprints (boot UUID, timestamp to the millisecond, process UUID).
//     Boot UUID alone is not enough, since separate collections from one
//     boot share it.
//
// Each group keeps its first JSONL file in walk order (JSONL carries every
// CSV field plus message flags and evidence), or its first file when none is
// JSONL. A file whose fingerprint cannot be read is left alone and imports
// normally.
func markUnifiedLogDuplicates(entries []walkEntry) {
	var candidates []int
	for i := range entries {
		if isUnifiedLogEntry(entries[i]) {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) < 2 {
		return
	}

	// Rule 1: same directory and base name, one CSV and one JSONL.
	byName, nameOrder := groupBy(candidates, func(i int) (string, bool) {
		path := entries[i].path
		stem := strings.TrimSuffix(path, filepath.Ext(path))
		return stem, true
	})
	for _, key := range nameOrder {
		group := byName[key]
		if hasBothFormats(entries, group) {
			keepOne(entries, group)
		}
	}

	// Rule 2: matching first-record fingerprints among the survivors.
	byFingerprint, fpOrder := groupBy(candidates, func(i int) (string, bool) {
		if !isUnifiedLogEntry(entries[i]) {
			return "", false // already marked a duplicate by rule 1
		}
		fp, err := unifiedLogFingerprint(entries[i])
		if err != nil {
			return "", false
		}
		return fp, true
	})
	for _, key := range fpOrder {
		if group := byFingerprint[key]; len(group) > 1 {
			keepOne(entries, group)
		}
	}
}

// isUnifiedLogEntry reports whether e is a UnifiedLog file still set to import.
func isUnifiedLogEntry(e walkEntry) bool {
	return e.skipReason == "" && e.format != nil && e.format.Family == FamilyUnifiedLog
}

func isUnifiedLogJSONL(e walkEntry) bool {
	return e.format != nil && e.format.Name == FormatUnifiedLogJSONL
}

func unifiedLogFingerprint(e walkEntry) (string, error) {
	if isUnifiedLogJSONL(e) {
		return unifiedlogparser.FingerprintJSONL(e.path)
	}
	return unifiedlogparser.FingerprintCSV(e.path)
}

// groupBy buckets indices by key, preserving walk order both within each
// group and across groups. key returns false to leave an index out.
func groupBy(indices []int, key func(int) (string, bool)) (map[string][]int, []string) {
	groups := make(map[string][]int)
	var order []string
	for _, i := range indices {
		k, ok := key(i)
		if !ok {
			continue
		}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}
	return groups, order
}

func hasBothFormats(entries []walkEntry, group []int) bool {
	var jsonl, csv bool
	for _, i := range group {
		if isUnifiedLogJSONL(entries[i]) {
			jsonl = true
		} else {
			csv = true
		}
	}
	return jsonl && csv
}

// keepOne marks every file in group except the keeper as a duplicate. group
// is in walk order.
func keepOne(entries []walkEntry, group []int) {
	keep := group[0]
	for _, i := range group {
		if isUnifiedLogJSONL(entries[i]) {
			keep = i
			break
		}
	}
	for _, i := range group {
		if i == keep {
			continue
		}
		entries[i].skipReason = fmt.Sprintf("duplicate of %s (same UnifiedLog data)", entries[keep].rel)
		entries[i].format = nil
		entries[i].label = ""
	}
}
