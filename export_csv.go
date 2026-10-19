package main

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/cdtdelta/4n6time/internal/csvparser"
	"github.com/cdtdelta/4n6time/internal/model"
)

// importFileColumn is the export column that names each event's source file.
const importFileColumn = "import_file"

// writeExportCSV writes events in export format: the csvparser columns, then
// import_file. importFiles maps an import ID to the source filename; events with
// ImportID 0, or with an ID missing from the map, get an empty import_file. The
// raw import ID is not exported.
func writeExportCSV(path string, events []*model.Event, importFiles map[int64]string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	if err := writer.Write(append(csvparser.ExportHeader(), importFileColumn)); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	for _, e := range events {
		row := append(csvparser.ExportRow(e), importFileFor(e, importFiles))
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("writing row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flushing CSV: %w", err)
	}
	return nil
}

// importFileFor returns the source filename for e, or "" when e has no import.
func importFileFor(e *model.Event, importFiles map[int64]string) string {
	if e.ImportID == 0 {
		return ""
	}
	return importFiles[e.ImportID]
}
