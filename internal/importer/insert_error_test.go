package importer

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

// TestRecursiveInsertErrorSkipReason checks that a file that parses but whose
// InsertEvents fails is skipped as an insert error, not a parse error, and
// that its record is failed with the error text.
func TestRecursiveInsertErrorSkipReason(t *testing.T) {
	root := t.TempDir()
	writeTmpCSV(t, root, "a_LECmd.csv", minimalLECmdCSV)

	store := &mockStore{insertErr: errors.New(`inserting event 1: column "offset" value 5747310864: failed to encode args[18]: boom`)}
	var logged []string
	logf := func(level, msg string) { logged = append(logged, level+" "+msg) }
	summary, err := ImportFolderRecursive(root, store, "", logf, nil)
	if err != nil {
		t.Fatalf("ImportFolderRecursive: %v", err)
	}
	if summary.TotalFilesProcessed != 0 || summary.TotalEvents != 0 {
		t.Errorf("processed %d files, %d events; want 0 and 0", summary.TotalFilesProcessed, summary.TotalEvents)
	}
	if len(summary.SkippedFiles) != 1 {
		t.Fatalf("SkippedFiles = %+v, want one", summary.SkippedFiles)
	}
	want := SkipReasonInsertError + `inserting event 1: column "offset" value 5747310864: failed to encode args[18]: boom`
	if got := summary.SkippedFiles[0].Reason; got != want {
		t.Errorf("Reason = %q, want %q", got, want)
	}

	if len(store.imports) != 1 {
		t.Fatalf("import records = %+v, want one", store.imports)
	}
	rec := store.imports[0]
	if rec.Status != model.ImportStatusFailed || !strings.Contains(rec.ErrorMessage, "args[18]") {
		t.Errorf("record = %+v, want failed with the insert error text", rec)
	}

	found := false
	for _, line := range logged {
		if strings.HasPrefix(line, "ERROR ") && strings.Contains(line, "a_LECmd.csv") && strings.Contains(line, "args[18]") {
			found = true
		}
	}
	if !found {
		t.Errorf("log %q has no ERROR line naming the file and the error", logged)
	}
}

func TestImportSkipReason(t *testing.T) {
	storeErr := errors.New("failed to encode args[18]: boom")
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"insert", &InsertError{Err: storeErr}, SkipReasonInsertError + "failed to encode args[18]: boom"},
		{"wrapped insert", fmt.Errorf("context: %w", &InsertError{Err: storeErr}), SkipReasonInsertError + "failed to encode args[18]: boom"},
		{"parse", fmt.Errorf("reading EZ Tools CSV: %w", errors.New("bad row")), SkipReasonParseError + "bad row"},
		{"bare", errors.New("bad row"), SkipReasonParseError + "bad row"},
	}
	for _, c := range cases {
		if got := importSkipReason(c.err); got != c.want {
			t.Errorf("%s: importSkipReason = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestInsertErrorText keeps the error text an import returns unchanged from
// before InsertError existed, since it is stored in error_message and shown.
func TestInsertErrorText(t *testing.T) {
	inner := errors.New("boom")
	err := &InsertError{Err: inner}
	if got := err.Error(); got != "inserting events: boom" {
		t.Errorf("Error() = %q, want %q", got, "inserting events: boom")
	}
	if !errors.Is(err, inner) {
		t.Errorf("errors.Is(InsertError, inner) = false")
	}
}
