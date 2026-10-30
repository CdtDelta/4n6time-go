package main

import (
	"fmt"
	"path/filepath"
)

// Import failure stages, in the order ImportCSV reaches them. The stage
// decides what the failure left in the database.
const (
	importStageDetect   = "detect"   // the file was not recognized
	importStageCreateDB = "create"   // the new SQLite database could not be created
	importStageRecord   = "record"   // the l2t_imports row could not be created
	importStageImport   = "import"   // parsing or inserting failed
	importStageFinalize = "finalize" // events committed; the row could not be finalized
	importStageMetadata = "metadata" // events committed; UpdateMetadata failed
	importStageReload   = "reload"   // events committed and metadata built; reading DBInfo failed

	// importStageImportUnrecorded is importStageImport when the record could
	// not be marked failed either, so it may still show as in progress.
	importStageImportUnrecorded = "import_unrecorded"
)

// ImportFailure describes a failed single-file import for the failure dialog.
// ImportCSV keeps the last one; the frontend reads it with GetLastImportFailure
// after ImportCSV returns an error.
type ImportFailure struct {
	File  string `json:"file"`  // base name of the imported file
	Path  string `json:"path"`  // full path of the imported file
	Error string `json:"error"` // the error text ImportCSV returned
	Stage string `json:"stage"` // one of the importStage constants
	// ImportID is the l2t_imports row, or 0 when none was created.
	ImportID int64 `json:"importId"`
	// EventsAdded is the number of events left in the database.
	EventsAdded int `json:"eventsAdded"`
	// Backend is "sqlite" or "postgres".
	Backend string `json:"backend"`
	// NewDatabase is the path of a SQLite database this import created and
	// then closed because of the failure. Empty when importing into an open
	// database.
	NewDatabase string `json:"newDatabase"`
	// Outcome says, in plain words, what the failure left in the database.
	Outcome string `json:"outcome"`
	// InImportsList reports whether the open database's Imports list shows
	// this import's record.
	InImportsList bool `json:"inImportsList"`
}

// GetLastImportFailure returns the failure from the most recent ImportCSV
// call, or nil when that call succeeded or was cancelled.
func (a *App) GetLastImportFailure() *ImportFailure {
	a.importFailMu.Lock()
	defer a.importFailMu.Unlock()
	return a.lastImportFailure
}

// setLastImportFailure stores f (nil clears it).
func (a *App) setLastImportFailure(f *ImportFailure) {
	a.importFailMu.Lock()
	a.lastImportFailure = f
	a.importFailMu.Unlock()
}

// importFailed logs a failed single-file import at ERROR, records it for
// GetLastImportFailure, and returns err unchanged so ImportCSV's error text
// stays what it was.
func (a *App) importFailed(f ImportFailure, err error) error {
	f.Error = err.Error()
	if f.File == "" {
		f.File = filepath.Base(f.Path)
	}
	f.Outcome = importFailureOutcome(f)
	f.InImportsList = f.ImportID != 0 && f.NewDatabase == ""
	a.logError(fmt.Sprintf("Import failed: file=%s stage=%s import=%d: %s", f.Path, f.Stage, f.ImportID, f.Error))
	a.setLastImportFailure(&f)
	return err
}

// importFailureOutcome says what a failed import left behind. Every parser
// reads the whole file before inserting, and both backends insert a file's
// events in one transaction (InsertEvents), so a parse or insert failure adds
// no events on either backend. Failures after the insert committed leave
// every event in place.
func importFailureOutcome(f ImportFailure) string {
	var msg string
	switch f.Stage {
	case importStageDetect:
		msg = "The file was not imported. Nothing was added to the database."
	case importStageCreateDB:
		msg = "The new database could not be created. Nothing was imported."
	case importStageRecord:
		msg = "No import record was created and no events from this file were added."
	case importStageImport:
		msg = fmt.Sprintf("No events from this file were added. The import is recorded as failed (import %d).", f.ImportID)
	case importStageImportUnrecorded:
		msg = fmt.Sprintf("No events from this file were added. The import failed, and its record (import %d) could not be updated, so it may still show as in progress in the Imports list.", f.ImportID)
	case importStageFinalize:
		msg = fmt.Sprintf("All %d events from this file were added, but the import record (import %d) could not be marked complete and still shows in progress. The filter panel values were not updated either; they will catch up after the next successful import.", f.EventsAdded, f.ImportID)
	case importStageMetadata:
		msg = fmt.Sprintf("All %d events from this file were added (import %d), but the filter panel values could not be updated. They will catch up after the next successful import.", f.EventsAdded, f.ImportID)
	case importStageReload:
		msg = fmt.Sprintf("All %d events from this file were added (import %d), but the database details could not be reloaded. Reopen the database to see them.", f.EventsAdded, f.ImportID)
	default:
		msg = "The import did not complete."
	}
	if f.NewDatabase != "" && f.Stage != importStageCreateDB {
		msg += fmt.Sprintf(" The new database file %s was created but not opened.", f.NewDatabase)
	}
	return msg
}
