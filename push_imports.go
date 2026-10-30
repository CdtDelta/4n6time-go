package main

import (
	"fmt"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/importer"
	"github.com/cdtdelta/4n6time/internal/model"
)

// copyImportRecords copies every import record from src to dst and returns the
// map from source ID to destination ID. Filename, label, format, imported_at,
// event count, status, and a failed import's error_message carry over.
// FinalizeImport accepts only final statuses, so an in_progress record stays
// in_progress on the destination. A destination l2t_imports table without
// error_message gets the column from migrateImportProvenance when
// CreateStore opens it, before this copy runs.
//
// The push is not one transaction: records, events (one transaction), notes,
// and metadata are separate steps, so a failure part way leaves the earlier
// steps in the destination.
func copyImportRecords(src, dst database.Store, logf importer.Logger) (map[int64]int64, error) {
	records, err := src.GetImports()
	if err != nil {
		return nil, fmt.Errorf("reading import records: %w", err)
	}

	idMap := make(map[int64]int64, len(records))
	for _, rec := range records {
		newID, err := dst.CreateImport(model.ImportRecord{
			Filename:   rec.Filename,
			Label:      rec.Label,
			Format:     rec.Format,
			ImportedAt: rec.ImportedAt,
		})
		if err != nil {
			return nil, fmt.Errorf("creating import record for %s: %w", rec.Filename, err)
		}
		idMap[rec.ID] = newID

		switch rec.Status {
		case model.ImportStatusComplete, model.ImportStatusFailed:
			if err := dst.FinalizeImport(newID, int(rec.EventCount), rec.Status, rec.ErrorMessage); err != nil {
				return nil, fmt.Errorf("finalizing import record for %s: %w", rec.Filename, err)
			}
		case model.ImportStatusInProgress:
			if rec.EventCount != 0 {
				logAt(logf, "ERROR", fmt.Sprintf("Import record %d (%s) is in_progress with %d events; the count cannot be copied",
					rec.ID, rec.Filename, rec.EventCount))
			}
		default:
			return nil, fmt.Errorf("import record %d (%s) has unknown status %q", rec.ID, rec.Filename, rec.Status)
		}
	}

	logAt(logf, "INFO", fmt.Sprintf("Copied %d import records to PostgreSQL", len(records)))
	return idMap, nil
}

// remapImportIDs rewrites each event's ImportID from its source ID to the
// destination ID in idMap. Zero stays zero. An ImportID with no mapping is a
// dangling reference in the source; it is cleared so the destination never
// points at a row that does not exist, and the count of cleared events is
// returned.
func remapImportIDs(events []*model.Event, idMap map[int64]int64) (cleared int) {
	for _, e := range events {
		if e.ImportID == 0 {
			continue
		}
		if newID, ok := idMap[e.ImportID]; ok {
			e.ImportID = newID
			continue
		}
		e.ImportID = 0
		cleared++
	}
	return cleared
}

// logAt sends msg to logf when logf is set. The app passes its log writer;
// tests pass nil.
func logAt(logf importer.Logger, level, msg string) {
	if logf != nil {
		logf(level, msg)
	}
}
