package main

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// errNoteDatetime is shown in the add note dialog. Its wording names the one
// form to type and the pasted grid form that also works.
var errNoteDatetime = errors.New("Enter the time in UTC as YYYY-MM-DD HH:MM:SS (a pasted 2009-07-14T04:58:36Z also works). Time zone offsets are not accepted.")

// noteDatetimeRe matches the three accepted forms: a space separator, or a T
// separator with an optional Z. Offsets, fractional seconds, and date-only
// values do not match.
var noteDatetimeRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})(?: |T)(\d{2}:\d{2}:\d{2})$|^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})Z$`)

const noteDatetimeLayout = "2006-01-02 15:04:05"

// parseNoteDatetime reads an examiner's note time, always UTC, and returns it
// as "YYYY-MM-DD HH:MM:SS". The grid shows RFC 3339, so a value copied from it
// must be accepted; anything that would need a conversion, truncation, or an
// assumed time is rejected instead. Surrounding whitespace is ignored.
func parseNoteDatetime(s string) (string, error) {
	m := noteDatetimeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", errNoteDatetime
	}
	date, clock := m[1], m[2]
	if date == "" {
		date, clock = m[3], m[4]
	}
	// The regexp checks shape only; time.Parse rejects 2025-02-31 and 25:00:00.
	t, err := time.Parse(noteDatetimeLayout, date+" "+clock)
	if err != nil {
		return "", errNoteDatetime
	}
	return t.Format(noteDatetimeLayout), nil
}
