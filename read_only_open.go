package main

// ReadOnlyOpenFailure describes a database that could not be opened because
// it was created by an older version of 4n6time, needs a one-time schema
// upgrade, and is on read-only storage. The store is closed and the file is
// unchanged. The frontend reads it with GetReadOnlyOpenFailure after
// OpenDatabase returns an error, and shows ReadOnlyDatabaseDialog.
type ReadOnlyOpenFailure struct {
	Path  string `json:"path"`  // full path of the database file
	File  string `json:"file"`  // base name of the database file
	Error string `json:"error"` // the underlying error text, for the log and details
}

// GetReadOnlyOpenFailure returns the read-only failure from the most recent
// OpenDatabase call, or nil when that call succeeded or failed for another
// reason.
func (a *App) GetReadOnlyOpenFailure() *ReadOnlyOpenFailure {
	a.importFailMu.Lock()
	defer a.importFailMu.Unlock()
	return a.lastReadOnlyOpen
}

// setReadOnlyOpenFailure stores f (nil clears it). It shares importFailMu
// with the import failure, since both are small last-failure records.
func (a *App) setReadOnlyOpenFailure(f *ReadOnlyOpenFailure) {
	a.importFailMu.Lock()
	a.lastReadOnlyOpen = f
	a.importFailMu.Unlock()
}
