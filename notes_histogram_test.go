package main

import (
	"fmt"
	"sort"
	"strings"
)

// Histogram expectations. The histogram applies the case's WHERE, ANDed with
// the junk-date guard, to evidence and to notes separately, and buckets each
// matching row by its datetime. These rules are read from runHistogram and
// histogramBucketFormat (internal/database/histogram.go):
//
//   - Junk-date guard: datetime > '1970-01-01' AND datetime < '2100-01-01'.
//   - Range: in simple mode with both a date from and a date to, the user's
//     range, with a date-only from at 00:00:00 and a date-only to at 23:59:59.
//     Otherwise the minimum and maximum datetime of the matching evidence, as
//     "YYYY-MM-DD HH:MM:SS" text; when no evidence matches, the matching
//     notes' minimum and maximum instead. No matching rows: no buckets.
//   - Format: hourly ("YYYY-MM-DD HH:00:00") when min and max share a day;
//     daily ("YYYY-MM-DD") when they share a month, or differ in month within
//     one year; monthly ("YYYY-MM") when the years differ.
//   - A row's bucket is its datetime cut to that format. Evidence and notes are
//     counted separately per bucket and never summed.
//   - Window: a note counts only when its bucket lies between the buckets of
//     the range's min and max, inclusive. Notes outside are dropped, so the
//     histogram's notes total can be lower than the grid's note count.

// mxHistBucket is one bucket's evidence and notes counts.
type mxHistBucket struct {
	evidence, notes int64
}

// mxRowDatetime maps every fixture key to its stored datetime text.
var mxRowDatetime = func() map[string]string {
	m := map[string]string{}
	for _, e := range mxEvidenceRows {
		m[e.key] = e.datetime
	}
	for _, n := range mxNoteRows {
		m[n.key] = n.datetime
	}
	return m
}()

// oraHistGuard is the junk-date guard over the fixed-width datetime text.
func oraHistGuard(dt string) bool {
	return dt > "1970-01-01" && dt < "2100-01-01"
}

// oraHistFormatLen returns the prefix length of a datetime that names its
// bucket, and the suffix the bucket label adds: 13 and ":00:00" for hourly,
// 10 for daily, 7 for monthly.
func oraHistFormatLen(minDate, maxDate string) (int, string) {
	switch {
	case minDate[:10] == maxDate[:10]:
		return 13, ":00:00"
	case minDate[:7] == maxDate[:7]:
		return 10, ""
	case minDate[:4] != maxDate[:4]:
		return 7, ""
	default:
		return 10, ""
	}
}

// oraHistRange returns the case's explicit histogram range: the simple-mode
// date range with date-only bounds widened to whole days. ok is false when the
// case has none.
func oraHistRange(c mxCase) (from, to string, ok bool) {
	if c.mode != "simple" || c.dateFrom == "" || c.dateTo == "" {
		return "", "", false
	}
	widen := func(v, clock string) string {
		if len(v) == len("2006-01-02") {
			return v + " " + clock
		}
		return v
	}
	return widen(c.dateFrom, "00:00:00"), widen(c.dateTo, "23:59:59"), true
}

// oraHistogram returns the expected buckets for the case's matching evidence
// and note keys. An empty map means no bucket.
func oraHistogram(c mxCase, evidence, notes []string) map[string]mxHistBucket {
	guarded := func(keys []string) []string {
		var dts []string
		for _, k := range keys {
			if dt := mxRowDatetime[k]; oraHistGuard(dt) {
				dts = append(dts, dt)
			}
		}
		sort.Strings(dts)
		return dts
	}
	out := map[string]mxHistBucket{}
	evDts, noteDts := guarded(evidence), guarded(notes)
	if len(evDts) == 0 && len(noteDts) == 0 {
		return out
	}
	lo, hi, explicit := oraHistRange(c)
	if !explicit {
		dts := evDts
		if len(dts) == 0 {
			dts = noteDts
		}
		lo, hi = dts[0], dts[len(dts)-1]
	}
	n, suffix := oraHistFormatLen(lo, hi)
	first, last := lo[:n]+suffix, hi[:n]+suffix
	add := func(keys []string, note bool) {
		for _, k := range keys {
			dt := mxRowDatetime[k]
			if !oraHistGuard(dt) {
				continue
			}
			label := dt[:n] + suffix
			if note && (label < first || label > last) {
				continue
			}
			b := out[label]
			if note {
				b.notes++
			} else {
				b.evidence++
			}
			out[label] = b
		}
	}
	add(evidence, false)
	add(notes, true)
	return out
}

// mxHistTotals sums a histogram's evidence and notes counts.
func mxHistTotals(h map[string]mxHistBucket) (evidence, notes int64) {
	for _, b := range h {
		evidence += b.evidence
		notes += b.notes
	}
	return evidence, notes
}

// mxRunHistogram calls GetTimelineHistogram with the request the frontend's
// TimelineChart sends for the case (the export request: simple fields, or the
// advanced clause with the base fields).
func mxRunHistogram(app *App, c mxCase) (map[string]mxHistBucket, error) {
	buckets, err := app.GetTimelineHistogram(mxExportRequest(c))
	if err != nil {
		return nil, err
	}
	out := map[string]mxHistBucket{}
	for _, b := range buckets {
		if _, dup := out[b.Timestamp]; dup {
			return nil, fmt.Errorf("duplicate bucket %q", b.Timestamp)
		}
		out[b.Timestamp] = mxHistBucket{evidence: b.Count, notes: b.NotesCount}
	}
	return out, nil
}

// mxCompareHistogram lists histogram issues: totals always, and per-bucket
// counts when buckets is true (golden cases).
func mxCompareHistogram(c mxCase, want oraResult, got map[string]mxHistBucket, gotErr error, buckets bool) (issues, detail []string) {
	if gotErr != nil {
		return []string{"histogram error"}, []string{"histogram error: " + gotErr.Error()}
	}
	exp := oraHistogram(c, want.evidence, want.notes)
	expEv, expNotes := mxHistTotals(exp)
	gotEv, gotNotes := mxHistTotals(got)
	if gotEv != expEv {
		issues = append(issues, "histogram evidence differ")
		detail = append(detail, fmt.Sprintf("histogram evidence total %d, want %d", gotEv, expEv))
	}
	if gotNotes != expNotes {
		issues = append(issues, "histogram notes differ")
		detail = append(detail, fmt.Sprintf("histogram notes total %d, want %d", gotNotes, expNotes))
	}
	if buckets && mxRenderHist(exp) != mxRenderHist(got) {
		issues = append(issues, "histogram buckets differ")
		detail = append(detail, fmt.Sprintf("histogram buckets %s, want %s", mxRenderHist(got), mxRenderHist(exp)))
	}
	return issues, detail
}

// mxRenderHist writes buckets as "label:evidence/notes" in label order.
func mxRenderHist(h map[string]mxHistBucket) string {
	labels := make([]string, 0, len(h))
	for k := range h {
		labels = append(labels, k)
	}
	sort.Strings(labels)
	parts := make([]string, len(labels))
	for i, k := range labels {
		parts[i] = fmt.Sprintf("%s:%d/%d", k, h[k].evidence, h[k].notes)
	}
	return "[" + strings.Join(parts, " ") + "]"
}
