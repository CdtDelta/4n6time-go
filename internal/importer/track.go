package importer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// Logger receives import lifecycle messages. level is "INFO" or "ERROR".
// A nil Logger discards messages.
type Logger func(level, msg string)

func logAt(logf Logger, level, msg string) {
	if logf != nil {
		logf(level, msg)
	}
}

// stampingStore hands every event it inserts to the wrapped store with
// ImportID set to the l2t_imports row of the import that produced it. Parsers
// insert only through the store passed to Format.Import, so wrapping that
// store is enough to attribute every event.
type stampingStore struct {
	database.Store
	importID int64
}

var _ database.Store = (*stampingStore)(nil)

// InsertEvent stamps e and inserts it.
func (s *stampingStore) InsertEvent(e *model.Event) error {
	if e != nil {
		e.ImportID = s.importID
	}
	return s.Store.InsertEvent(e)
}

// InsertEvents stamps each event and inserts the batch.
func (s *stampingStore) InsertEvents(events []*model.Event, onProgress func(int)) (int, error) {
	for _, e := range events {
		if e != nil {
			e.ImportID = s.importID
		}
	}
	return s.Store.InsertEvents(events, onProgress)
}

// trackedSpec describes the l2t_imports row for one file.
type trackedSpec struct {
	Filename string
	Label    string
	Format   string
}

// TrackedResult reports the outcome of one tracked import. Errors are kept
// apart so callers can tell a file that never reached the database from one
// that failed mid-import.
type TrackedResult struct {
	// ID is the l2t_imports row. Zero means the row could not be created and
	// the file was not imported.
	ID int64
	// Count is the number of events the import reported inserting.
	Count int
	// Unparseable is the parser's count of non-empty timestamp values it
	// skipped (Progress.Unparseable). It is zero for formats that do not
	// report one, and set only by ImportFile.
	Unparseable int
	// ImportErr is the error from the import itself. The row is then marked failed.
	ImportErr error
	// RecordErr is the error from creating or finalizing the row. When ID is
	// zero, the import did not run. When ID is set, the events were already
	// inserted and only the status update failed.
	RecordErr error
}

// runTracked records an l2t_imports row, runs the import with events stamped
// to that row, and finalizes the row as complete or failed. It never returns
// an error directly; the outcome is in the result so recursive import can
// keep going past a single bad file.
func runTracked(store database.Store, spec trackedSpec, logf Logger, run func(database.Store) (int, error)) TrackedResult {
	var res TrackedResult
	rec := model.ImportRecord{
		Filename: spec.Filename,
		Label:    strings.TrimSpace(spec.Label),
		Format:   spec.Format,
	}

	id, err := store.CreateImport(rec)
	if err != nil {
		res.RecordErr = fmt.Errorf("recording import: %w", err)
		logAt(logf, "ERROR", fmt.Sprintf("Could not record import of %s: %v", rec.Filename, err))
		return res
	}
	res.ID = id
	logAt(logf, "INFO", fmt.Sprintf("Import record %d created: file=%s format=%s label=%s",
		id, rec.Filename, rec.Format, rec.Label))

	count, runErr := run(&stampingStore{Store: store, importID: id})
	res.Count = count
	status := model.ImportStatusComplete
	errText := ""
	if runErr != nil {
		res.ImportErr = runErr
		status = model.ImportStatusFailed
		errText = runErr.Error()
		logAt(logf, "ERROR", fmt.Sprintf("Import %d failed: file=%s format=%s: %s",
			id, rec.Filename, rec.Format, errText))
	}

	if err := store.FinalizeImport(id, count, status, errText); err != nil {
		res.RecordErr = fmt.Errorf("finalizing import %d: %w", id, err)
		logAt(logf, "ERROR", res.RecordErr.Error())
		return res
	}
	logAt(logf, "INFO", fmt.Sprintf("Import record %d finalized: file=%s format=%s events=%d status=%s",
		id, rec.Filename, rec.Format, count, status))
	return res
}

// logUnparseableExamples writes the skipped-value examples for one file as a
// single log line, so the examples stay together. It does nothing when the
// parser reported none.
func logUnparseableExamples(logf Logger, name string, p *Progress) {
	if p == nil || len(p.UnparseableExamples) == 0 {
		return
	}
	logAt(logf, "INFO", fmt.Sprintf("%s: unparseable timestamp examples: %s",
		name, strings.Join(p.UnparseableExamples, "; ")))
}

// ImportFile imports one file through format as a tracked import. The
// filename is the file's base name. formatLabel is the label DetectFile returned
// for the file (the per-tool name for EZ Tools, the entry name otherwise), so
// single-file and recursive import record the same format for the same file.
// label is trimmed; an empty label is stored as an empty string.
//
// The returned error is the first of: the row could not be created (nothing
// imported), the import failed (row marked failed, original error returned
// unchanged), or the row could not be finalized after a successful import.
func ImportFile(path string, format *Format, formatLabel, label string, store database.Store, p *Progress, logf Logger) (TrackedResult, error) {
	res := runTracked(store, trackedSpec{
		Filename: filepath.Base(path),
		Label:    label,
		Format:   formatLabel,
	}, logf, func(s database.Store) (int, error) {
		return format.Import(path, s, p)
	})
	if p != nil {
		res.Unparseable = p.Unparseable
	}
	if p != nil && p.Unparseable > 0 {
		logAt(logf, "INFO", fmt.Sprintf("%s: %d unparseable timestamp value(s) skipped",
			filepath.Base(path), p.Unparseable))
	}
	logUnparseableExamples(logf, filepath.Base(path), p)
	switch {
	case res.RecordErr != nil && res.ID == 0:
		return res, res.RecordErr
	case res.ImportErr != nil:
		return res, res.ImportErr
	case res.RecordErr != nil:
		return res, res.RecordErr
	}
	return res, nil
}
