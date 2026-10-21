package main

import (
	"database/sql"
	"testing"
)

func TestParseNoteDatetime(t *testing.T) {
	accepted := []struct{ in, want string }{
		{"2009-07-14 04:58:36", "2009-07-14 04:58:36"},
		{"2009-07-14T04:58:36", "2009-07-14 04:58:36"},
		{"2009-07-14T04:58:36Z", "2009-07-14 04:58:36"},
		{"  2009-07-14T04:58:36Z\n", "2009-07-14 04:58:36"},
		{"2024-02-29 23:59:59", "2024-02-29 23:59:59"},
	}
	for _, c := range accepted {
		got, err := parseNoteDatetime(c.in)
		if err != nil || got != c.want {
			t.Errorf("parseNoteDatetime(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}

	rejected := []struct{ in, why string }{
		{"2009-07-14T04:58:36+05:00", "offset"},
		{"2009-07-14T04:58:36-0600", "offset without colon"},
		{"2009-07-14 04:58:36+00:00", "zero offset"},
		{"2009-07-14 04:58:36Z", "Z after a space separator"},
		{"2009-07-14T04:58:36.123Z", "fractional seconds"},
		{"2009-07-14 04:58:36.5", "fractional seconds"},
		{"2009-07-14", "date only"},
		{"2009-07-14 04:58", "no seconds"},
		{"2025-02-31 10:00:00", "invalid day"},
		{"2025-13-01 10:00:00", "invalid month"},
		{"2025-06-30 24:00:00", "invalid hour"},
		{"2025-06-30 10:60:00", "invalid minute"},
		{"x2009-07-14 04:58:36", "leading junk"},
		{"2009-07-14 04:58:36 UTC", "trailing junk"},
		{"2009-07-14T04:58:36Zz", "trailing junk after Z"},
		{"", "empty"},
		{"07/14/2009 04:58:36", "other layout"},
	}
	for _, c := range rejected {
		got, err := parseNoteDatetime(c.in)
		if err != errNoteDatetime {
			t.Errorf("parseNoteDatetime(%q) (%s) = %q, %v; want errNoteDatetime", c.in, c.why, got, err)
		}
	}
}

// AddExaminerNote stores a pasted grid timestamp in the canonical form and
// stores nothing for a rejected one. The text is read with CAST, because the
// SQLite driver returns a DATETIME column as a time value, which
// GetExaminerNotes would render as RFC 3339 whatever the stored form.
func TestAddExaminerNoteCanonicalDatetime(t *testing.T) {
	app, store := newTestSQLiteApp(t)
	if _, err := app.AddExaminerNote("2009-07-14T04:58:36Z", "pasted"); err != nil {
		t.Fatalf("AddExaminerNote: %v", err)
	}
	if _, err := app.AddExaminerNote("2009-07-14T04:58:36+05:00", "offset"); err != errNoteDatetime {
		t.Fatalf("AddExaminerNote with offset: err = %v, want errNoteDatetime", err)
	}
	conn, err := sql.Open("sqlite", store.Path())
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer conn.Close()
	rows, err := conn.Query("SELECT CAST(datetime AS TEXT) FROM examiner_notes")
	if err != nil {
		t.Fatalf("reading notes: %v", err)
	}
	defer rows.Close()
	var stored []string
	for rows.Next() {
		var dt string
		if err := rows.Scan(&dt); err != nil {
			t.Fatalf("scanning note: %v", err)
		}
		stored = append(stored, dt)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading notes: %v", err)
	}
	if len(stored) != 1 || stored[0] != "2009-07-14 04:58:36" {
		t.Fatalf("stored note datetimes = %q, want [2009-07-14 04:58:36]", stored)
	}
}
