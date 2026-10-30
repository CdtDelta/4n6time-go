package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cdtdelta/4n6time/internal/model"
)

// mxEvidence is one evidence row of the matrix fixture. importID 0 is stored
// as SQL NULL, and every other empty text field is stored as an empty string.
type mxEvidence struct {
	key, datetime, source, sourcetype, host, user, desc, tag, color string
	bookmark, importID                                              int64
}

// mxNote is one examiner note. storeTag marks a note whose tag goes in through
// Store.InsertExaminerNote, because App.AddExaminerNote always passes an empty
// tag. Every other note goes through AddExaminerNote.
type mxNote struct {
	key, datetime, desc, tag, color string
	bookmark                        int64
	storeTag                        bool
}

var mxEvidenceRows = []mxEvidence{
	{"E01", "2025-01-10 10:00:00", "FILE", "NTFS", "WS1", "alice", "malware.exe", "", "RED", 0, 1},
	{"E02", "2025-02-20 10:00:00", "FILE", "NTFS", "WS1", "", "beacon log", "malware,lateral", "BLUE", 1, 0},
	{"E03", "2025-03-01 09:00:00", "REG", "Registry", "WS1", "", "run key", "", "", 0, 2},
	{"E04", "2025-03-15 10:00:00", "EVT", "WinEVTX", "WS2", "bob", "logon event", "lateral", "RED", 0, 0},
	{"E05", "2025-03-02 00:00:00", "UNIFIEDLOG", "macOS Unified Log", "WS2", "", "BEACON out", "", "BLUE", 0, 2},
	{"E06", "2025-06-30 12:00:00", "FILE", "NTFS", "WS2", "", "sql order by test", "lateral", "", 1, 3},
	{"E07", "2025-06-30 23:59:59", "REG", "Registry", "WS1", "", "50% done", "", "RED", 0, 0},
	{"E08", "2025-05-20 10:00:00", "FILE", "NTFS", "WS1", "alice", "file_name.txt", "malware,lateral", "", 0, 1},
	{"E09", "2025-07-04 10:00:00", "REG", "Registry", "", "", "beacon to c2", "", "BLUE", 1, 0},
	{"E10", "2025-11-02 10:00:00", "EVT", "WinEVTX", "WS2", "", "service install", "", "", 0, 3},
	{"E11", "2026-01-05 10:00:00", "UNIFIEDLOG", "macOS Unified Log", "", "", "cleanup key", "", "RED", 0, 0},
	{"E12", "2025-09-30 08:30:00", "FILE", "NTFS", "WS1", "", "Beacon C2 check", "", "BLUE", 0, 1},
	{"E13", "2025-06-15 12:00:00", "EVT", "WinEVTX", "WS2", "", "logon type 3", "", "", 1, 0},
	{"E14", "2025-06-30 10:00:00", "EXAMINER", "Examiner Note", "", "", "re-imported note", "", "", 0, 0},
}

// mxEvidenceURLs holds the URL column for the rows that have one. Every other
// evidence row stores an empty URL. E10's URL is the only place "payload"
// appears, so keyword search on payload exercises the URL column.
var mxEvidenceURLs = map[string]string{
	"E10": "https://update.example.com/payload",
}

var mxNoteRows = []mxNote{
	{"N1", "2025-03-01 09:00:00", "beacon to c2", "lateral", "RED", 1, true},
	{"N2", "2025-06-15 12:00:00", "user login", "", "BLUE", 0, false},
	{"N3", "2025-09-30 08:30:00", "malware dropped", "", "", 1, false},
	{"N4", "2026-01-10 10:00:00", "cleanup", "", "RED", 0, false},
	{"N5", "2025-03-02 00:00:00", "Beacon renewal", "", "BLUE", 0, false},
	{"N6", "2025-06-30 23:59:59", "phone call with HR", "", "", 0, false},
	{"N7", "2025-06-30 18:00:00", "pasted from grid", "", "", 0, false},
}

// mxNoteStoredDatetimes holds the datetime text written for notes that go in
// straight through Store.InsertExaminerNote, bypassing AddExaminerNote's
// validation, as notes saved by older versions were. The note's datetime in
// mxNoteRows is the normalized value, which the oracle uses. N7 is a grid
// timestamp pasted as typed.
var mxNoteStoredDatetimes = map[string]string{
	"N7": "2025-06-30T18:00:00Z",
}

// mxIDs maps the IDs the grid returns to fixture keys: positive for evidence,
// negative for notes.
type mxIDs struct {
	evidence map[int64]string
	notes    map[int64]string
}

// mxBuildFixture loads the evidence and notes through the app and store paths,
// then reads everything back and fails the test on any setup mismatch.
func mxBuildFixture(t *testing.T, app *App) mxIDs {
	t.Helper()
	events := make([]*model.Event, 0, len(mxEvidenceRows))
	for _, e := range mxEvidenceRows {
		events = append(events, &model.Event{
			Datetime:     e.datetime,
			Source:       e.source,
			SourceType:   e.sourcetype,
			Host:         e.host,
			User:         e.user,
			Desc:         e.desc,
			Tag:          e.tag,
			Color:        e.color,
			Bookmark:     e.bookmark,
			ImportID:     e.importID,
			RecordNumber: e.key,
			URL:          mxEvidenceURLs[e.key],
		})
	}
	if _, err := app.store.InsertEvents(events, nil); err != nil {
		t.Fatalf("fixture: InsertEvents: %v", err)
	}
	for _, n := range mxNoteRows {
		var id int64
		var err error
		if stored, ok := mxNoteStoredDatetimes[n.key]; ok {
			id, err = app.store.InsertExaminerNote(stored, n.desc, n.tag, "")
		} else if n.storeTag {
			id, err = app.store.InsertExaminerNote(n.datetime, n.desc, n.tag, "")
		} else {
			id, err = app.AddExaminerNote(n.datetime, n.desc)
		}
		if err != nil {
			t.Fatalf("fixture: adding note %s: %v", n.key, err)
		}
		// The store returns note IDs already negated (InsertExaminerNote returns -id),
		// so id is passed to the bulk calls unchanged.
		if n.color != "" {
			if _, err := app.BulkUpdateColor([]int64{id}, n.color); err != nil {
				t.Fatalf("fixture: coloring note %s: %v", n.key, err)
			}
		}
		if n.bookmark == 1 {
			if err := app.BulkSetBookmark([]int64{id}, 1); err != nil {
				t.Fatalf("fixture: bookmarking note %s: %v", n.key, err)
			}
		}
	}
	return mxValidateFixture(t, app)
}

// mxValidateFixture reads every row back through QueryEvents and compares it
// with the definitions. Any difference is a setup error, so the test fails.
func mxValidateFixture(t *testing.T, app *App) mxIDs {
	t.Helper()
	res, err := app.QueryEvents(QueryRequest{OrderBy: "datetime", Page: 1, PageSize: 1000})
	if err != nil {
		t.Fatalf("fixture readback: %v", err)
	}
	ids := mxIDs{evidence: map[int64]string{}, notes: map[int64]string{}}
	evByKey := map[string]*model.Event{}
	noteByDesc := map[string]*model.Event{}
	for _, e := range res.Events {
		if e.ID > 0 {
			evByKey[e.RecordNumber] = e
		} else {
			noteByDesc[e.Desc] = e
		}
	}

	var problems []string
	check := func(key, field string, got, want interface{}) {
		if fmt.Sprint(got) != fmt.Sprint(want) {
			problems = append(problems, fmt.Sprintf("%s %s: got %q, want %q", key, field, fmt.Sprint(got), fmt.Sprint(want)))
		}
	}
	for _, d := range mxEvidenceRows {
		e, ok := evByKey[d.key]
		if !ok {
			problems = append(problems, "evidence "+d.key+" missing from readback")
			continue
		}
		ids.evidence[e.ID] = d.key
		check(d.key, "datetime", mxCanonicalTime(e.Datetime), d.datetime)
		check(d.key, "source", e.Source, d.source)
		check(d.key, "sourcetype", e.SourceType, d.sourcetype)
		check(d.key, "host", e.Host, d.host)
		check(d.key, "user", e.User, d.user)
		check(d.key, "desc", e.Desc, d.desc)
		check(d.key, "tag", e.Tag, d.tag)
		check(d.key, "color", e.Color, d.color)
		check(d.key, "bookmark", e.Bookmark, d.bookmark)
		check(d.key, "import_id", e.ImportID, d.importID)
		check(d.key, "URL", e.URL, mxEvidenceURLs[d.key])
	}
	for _, n := range mxNoteRows {
		e, ok := noteByDesc[n.desc]
		if !ok {
			problems = append(problems, "note "+n.key+" missing from readback")
			continue
		}
		ids.notes[e.ID] = n.key
		check(n.key, "datetime", mxCanonicalTime(e.Datetime), n.datetime)
		check(n.key, "tag", e.Tag, n.tag)
		check(n.key, "color", e.Color, n.color)
		check(n.key, "bookmark", e.Bookmark, n.bookmark)
	}
	if len(evByKey) != len(mxEvidenceRows) || len(noteByDesc) != len(mxNoteRows) {
		problems = append(problems, fmt.Sprintf("readback counts: %d evidence, %d notes", len(evByKey), len(noteByDesc)))
	}
	if len(problems) > 0 {
		t.Fatalf("fixture readback does not match the definitions (setup error):\n  %s", strings.Join(problems, "\n  "))
	}
	return ids
}

// mxOracleRows builds the spec's view of the fixture: evidence rows as stored,
// and notes projected with source EXAMINER, sourcetype Examiner Note, and every
// other column NULL (absent from cells).
func mxOracleRows() []oraRow {
	var rows []oraRow
	textCols := append([]string{"datetime", "source", "sourcetype", "host", "user", "desc", "tag", "color", "filename", "type"}, oraKeywordCols...)
	for _, e := range mxEvidenceRows {
		cells := map[string]interface{}{}
		for _, col := range textCols {
			cells[col] = ""
		}
		cells["datetime"] = e.datetime
		cells["source"] = e.source
		cells["sourcetype"] = e.sourcetype
		cells["host"] = e.host
		cells["user"] = e.user
		cells["desc"] = e.desc
		cells["tag"] = e.tag
		cells["color"] = e.color
		cells["bookmark"] = e.bookmark
		cells["url"] = mxEvidenceURLs[e.key]
		if e.importID != 0 {
			cells["import_id"] = e.importID
		}
		rows = append(rows, oraRow{key: e.key, cells: cells})
	}
	for _, n := range mxNoteRows {
		rows = append(rows, oraRow{key: n.key, note: true, cells: map[string]interface{}{
			"datetime":   n.datetime,
			"source":     "EXAMINER",
			"sourcetype": "Examiner Note",
			"desc":       n.desc,
			"tag":        n.tag,
			"color":      n.color,
			"bookmark":   n.bookmark,
		}})
	}
	return rows
}

// mxCanonicalTime returns a datetime in the stored "YYYY-MM-DD HH:MM:SS" form.
// PostgreSQL returns TIMESTAMP values as RFC 3339 text through database/sql, so
// the harness compares canonical forms and counts the raw differences separately.
func mxCanonicalTime(s string) string {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	return s
}
