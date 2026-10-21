package database

import (
	"strings"
	"time"
)

// storedDatetimeLayouts are modernc.org/sqlite v1.48.0's parseTimeFormats, the
// layouts it tried on a DATETIME column's text before returning a time.Time.
// They are copied so a datetime displays the same whether or not the driver
// parsed it.
var storedDatetimeLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// timeStringLayout is time.Time.String's form, which modernc also accepted.
const timeStringLayout = "2006-01-02 15:04:05.999999999 -0700 MST"

// displayDatetime formats a datetime read from either store the way every
// release through v0.17.0 showed it: modernc parsed the DATETIME text and
// database/sql wrote the time.Time into the string as RFC 3339 (RFC3339Nano,
// so fractional seconds and offsets are kept). A value neither would parse,
// such as Plaso's "Not a time" or an empty string, is returned unchanged.
//
// SQLite stops parsing when a query's datetime column has no declared type, as
// the notes projection's normalized datetime causes, so every event scan calls
// this instead of relying on the driver. It is idempotent: RFC 3339 output, and
// PostgreSQL's TIMESTAMP values, come back unchanged.
func displayDatetime(s string) string {
	t, ok := parseStoredDatetime(s)
	if !ok {
		return s
	}
	return t.Format(time.RFC3339Nano)
}

// parseStoredDatetime follows modernc's parseTime with no _loc DSN parameter,
// which this app never sets: the time.String form first, then each layout in
// UTC with one trailing Z removed.
func parseStoredDatetime(s string) (time.Time, bool) {
	ts := s
	if x := strings.Index(ts, "m="); x > 0 {
		ts = ts[:x]
	}
	if t, err := time.Parse(timeStringLayout, strings.TrimSpace(ts)); err == nil {
		return t, true
	}
	ts, _ = strings.CutSuffix(s, "Z")
	for _, layout := range storedDatetimeLayouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
