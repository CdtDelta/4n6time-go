package main

import (
	"fmt"
	"time"

	"github.com/cdtdelta/4n6time/internal/database"
)

// cachedEvidenceCount returns the number of evidence rows in log2timeline
// from l2t_stats, without counting the table. When the row is missing it
// counts live once and stores the result with InsertStatIfMissing, so a value
// written meanwhile by another connection is not overwritten. A failure to
// read or store the cached value is passed to warn and the live count is
// used; on read-only storage the count still works, just without the cache.
//
// Only unfiltered evidence counts may use this: getDBInfo, the grid count
// when the query has no WHERE (QueryEvents), and Push to PostgreSQL's source
// count. Examiner notes are never included.
func cachedEvidenceCount(store database.Store, warn func(string)) (int64, error) {
	n, ok, err := store.GetStat(database.StatEventCount)
	if err == nil && ok {
		return n, nil
	}
	if err != nil {
		warn("Reading cached event count failed, counting live: " + err.Error())
	}
	n, err = store.CountEvents("", nil)
	if err != nil {
		return 0, err
	}
	if err := store.InsertStatIfMissing(database.StatEventCount, n); err != nil {
		warn("Storing cached event count failed, using the live count: " + err.Error())
	}
	return n, nil
}

// evidenceCount is cachedEvidenceCount for the open store, warning to the log.
func (a *App) evidenceCount() (int64, error) {
	return cachedEvidenceCount(a.store, a.logWarn)
}

// RecountResult is the outcome of Tools > Recount Events.
type RecountResult struct {
	// Previous is the cached value before the recount; HadPrevious is false
	// when there was none.
	Previous    int64 `json:"previous"`
	HadPrevious bool  `json:"hadPrevious"`
	// Count is the live count now stored.
	Count int64 `json:"count"`
}

// RecountEvents counts the evidence rows live and overwrites the cached
// event count with the result. It corrects a cached value missed by the
// rebuild window described at database.StatEventCount. The old and new values
// are logged at INFO.
func (a *App) RecountEvents() (*RecountResult, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}
	start := time.Now()
	prev, had, err := a.store.GetStat(database.StatEventCount)
	if err != nil {
		a.logWarn("Reading cached event count before recount failed: " + err.Error())
		had = false
	}
	n, err := a.store.CountEvents("", nil)
	if err != nil {
		a.logError("Recount events failed: " + err.Error())
		return nil, fmt.Errorf("counting events: %w", err)
	}
	if err := a.store.SetStat(database.StatEventCount, n); err != nil {
		a.logError("Storing recounted event count failed: " + err.Error())
		return nil, fmt.Errorf("storing event count: %w", err)
	}
	old := "none"
	if had {
		old = fmt.Sprintf("%d", prev)
	}
	a.logInfo(fmt.Sprintf("Recount events: cached count %s, live count %d (%s)",
		old, n, time.Since(start).Round(time.Millisecond)))
	return &RecountResult{Previous: prev, HadPrevious: had, Count: n}, nil
}
