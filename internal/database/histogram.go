package database

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/cdtdelta/4n6time/internal/query"
)

// runHistogram buckets the evidence and the projected examiner notes that
// match whereClause, with the same clause and arguments on each half, and
// merges the two by bucket. The counts stay separate: Count is evidence only
// and NotesCount is notes only.
//
// whereClause includes the WHERE keyword when non-empty. rangeExpr is the
// dialect's SELECT list for the minimum and maximum datetime as
// "YYYY-MM-DD HH:MM:SS" text.
//
// An explicit window (the user's simple-mode date range) is the window as
// given, so a note inside the user's range shows even when the matching
// evidence spans less of it. Otherwise the window is the matching evidence's
// range, so notes never stretch or coarsen the evidence chart, and only when
// no evidence matches does the notes' range stand in. A note counts only when
// its bucket falls inside the window's first and last bucket; notes outside
// are dropped, not clamped to an edge.
func runHistogram(conn *sql.DB, d Dialect, rangeExpr, whereClause string, whereArgs []interface{}, window *HistogramWindow) ([]TimelineBucket, error) {
	sources := []string{"log2timeline", query.NotesSource(d)}

	var minDate, maxDate string
	if window != nil {
		minDate, maxDate = window.From, window.To
	}
	for _, src := range sources {
		if minDate != "" && maxDate != "" {
			break
		}
		var lo, hi sql.NullString
		rangeSQL := "SELECT " + rangeExpr + " FROM " + src
		if whereClause != "" {
			rangeSQL += " " + whereClause
		}
		if err := conn.QueryRow(rangeSQL, whereArgs...).Scan(&lo, &hi); err != nil {
			return nil, fmt.Errorf("getting date range: %w", err)
		}
		if lo.String != "" && hi.String != "" {
			minDate, maxDate = lo.String, hi.String
			break
		}
	}
	if minDate == "" || maxDate == "" {
		return []TimelineBucket{}, nil
	}

	format := histogramBucketFormat(minDate, maxDate)
	bucketExpr := d.DateFormatSQL("datetime", format)
	counts := make([]map[string]int64, len(sources))
	for i, src := range sources {
		histSQL := "SELECT " + bucketExpr + " as bucket, COUNT(*) as cnt FROM " + src
		if whereClause != "" {
			histSQL += " " + whereClause
		}
		histSQL += " GROUP BY bucket ORDER BY bucket"
		m, err := histogramCounts(conn, histSQL, whereArgs)
		if err != nil {
			return nil, err
		}
		counts[i] = m
	}
	notes := windowBuckets(counts[1], histogramBucketKey(minDate, format), histogramBucketKey(maxDate, format))
	// An explicit window skips the range queries, so a range with no matching
	// rows lands here and stays empty, as it does without a window.
	return mergeHistogramBuckets(counts[0], notes), nil
}

// windowBuckets keeps the buckets whose key lies between first and last
// inclusive. Keys share one fixed-width format, so text order is time order.
func windowBuckets(counts map[string]int64, first, last string) map[string]int64 {
	out := make(map[string]int64, len(counts))
	for k, n := range counts {
		if k >= first && k <= last {
			out[k] = n
		}
	}
	return out
}

// histogramBucketKey cuts a "YYYY-MM-DD HH:MM:SS" datetime to the bucket label
// that format produces, matching what DateFormatSQL returns for the row. A
// datetime too short to cut is returned whole. An ISO "T" separator is read as
// a space, because strftime and to_char always write a space.
func histogramBucketKey(dt, format string) string {
	if len(dt) > 10 && dt[10] == 'T' {
		dt = dt[:10] + " " + dt[11:]
	}
	switch format {
	case "%Y-%m-%d %H:00:00":
		if len(dt) >= 13 {
			return dt[:13] + ":00:00"
		}
	case "%Y-%m-%d":
		if len(dt) >= 10 {
			return dt[:10]
		}
	case "%Y-%m":
		if len(dt) >= 7 {
			return dt[:7]
		}
	}
	return dt
}

// histogramCounts runs one grouped bucket query and returns count by bucket.
func histogramCounts(conn *sql.DB, histSQL string, args []interface{}) (map[string]int64, error) {
	rows, err := conn.Query(histSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("histogram query: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var bucket string
		var n int64
		if err := rows.Scan(&bucket, &n); err != nil {
			return nil, fmt.Errorf("scanning bucket: %w", err)
		}
		out[bucket] = n
	}
	return out, rows.Err()
}

// mergeHistogramBuckets joins evidence and notes counts by bucket, sorted by
// bucket. A bucket with only notes has Count 0.
func mergeHistogramBuckets(evidence, notes map[string]int64) []TimelineBucket {
	keys := make([]string, 0, len(evidence)+len(notes))
	for k := range evidence {
		keys = append(keys, k)
	}
	for k := range notes {
		if _, ok := evidence[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	buckets := make([]TimelineBucket, 0, len(keys))
	for _, k := range keys {
		buckets = append(buckets, TimelineBucket{Timestamp: k, Count: evidence[k], NotesCount: notes[k]})
	}
	return buckets
}

// histogramBucketFormat chooses the strftime-style bucket format from the
// range's "YYYY-MM-DD HH:MM:SS" endpoints: hourly within one day, daily within
// one month or one year, monthly across years.
func histogramBucketFormat(minDate, maxDate string) string {
	if len(minDate) < 10 || len(maxDate) < 10 {
		return "%Y-%m-%d %H:00:00"
	}
	switch {
	case minDate[:10] == maxDate[:10]:
		return "%Y-%m-%d %H:00:00"
	case minDate[:7] == maxDate[:7]:
		return "%Y-%m-%d"
	case minDate[:4] != maxDate[:4]:
		return "%Y-%m"
	default:
		return "%Y-%m-%d"
	}
}
