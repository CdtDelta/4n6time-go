package database

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestDisplayDatetime(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2025-01-10 10:00:00", "2025-01-10T10:00:00Z"},
		{"2025-01-10T10:00:00", "2025-01-10T10:00:00Z"},
		{"2025-01-10T10:00:00Z", "2025-01-10T10:00:00Z"},
		{"2025-01-10 10:00:00.123456", "2025-01-10T10:00:00.123456Z"},
		{"2025-01-10T10:00:00.5Z", "2025-01-10T10:00:00.5Z"},
		{"2025-01-10 10:00", "2025-01-10T10:00:00Z"},
		{"2025-01-10", "2025-01-10T00:00:00Z"},
		{"2025-01-10T10:00:00+05:00", "2025-01-10T10:00:00+05:00"},
		{"2025-01-10 10:00:00 +0000 UTC", "2025-01-10T10:00:00Z"},
		{time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), "2025-01-10T10:00:00Z"},
		// Passthrough: not a datetime, so returned exactly as read.
		{"Not a time", "Not a time"},
		{"", ""},
		{" 2025-01-10 10:00:00", " 2025-01-10 10:00:00"},
		{"2025-02-31 10:00:00", "2025-02-31 10:00:00"},
		{"0", "0"},
		{"garbage Z", "garbage Z"},
	}
	for _, c := range cases {
		if got := displayDatetime(c.in); got != c.want {
			t.Errorf("displayDatetime(%q) = %q, want %q", c.in, got, c.want)
		}
		if got := displayDatetime(c.want); got != c.want {
			t.Errorf("displayDatetime not idempotent on %q: got %q", c.want, got)
		}
	}
}

// The pre-3c read path: a DATETIME column scanned into a string, so modernc
// parses the text and database/sql formats the time. displayDatetime over the
// raw text must match it byte for byte.
func TestDisplayDatetimeMatchesDriver(t *testing.T) {
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "display.db"))
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, dt DATETIME)"); err != nil {
		t.Fatalf("creating table: %v", err)
	}
	stored := []string{
		"2025-01-10 10:00:00",
		"2025-01-10T10:00:00",
		"2025-01-10T10:00:00Z",
		"Not a time",
		"",
		"2025-01-10 10:00:00.123456",
		"2025-01-10T10:00:00.5Z",
		"2025-01-10",
		"2025-01-10T10:00:00+05:00",
	}
	for i, s := range stored {
		if _, err := conn.Exec("INSERT INTO t (id, dt) VALUES (?, ?)", i, s); err != nil {
			t.Fatalf("inserting %q: %v", s, err)
		}
	}
	for i, s := range stored {
		var viaDriver, raw string
		if err := conn.QueryRow("SELECT dt, CAST(dt AS TEXT) FROM t WHERE id = ?", i).Scan(&viaDriver, &raw); err != nil {
			t.Fatalf("reading %q: %v", s, err)
		}
		if raw != s {
			t.Fatalf("stored text %q read back as %q", s, raw)
		}
		if got := displayDatetime(raw); got != viaDriver {
			t.Errorf("stored %q: displayDatetime = %q, driver = %q", s, got, viaDriver)
		}
	}
}
