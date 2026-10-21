package database

import (
	"reflect"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

func TestHistogramBucketFormat(t *testing.T) {
	cases := []struct{ min, max, want string }{
		{"2025-01-10 10:00:00", "2025-01-10 23:00:00", "%Y-%m-%d %H:00:00"},
		{"2025-01-10 10:00:00", "2025-01-20 10:00:00", "%Y-%m-%d"},
		{"2025-01-10 10:00:00", "2025-06-20 10:00:00", "%Y-%m-%d"},
		{"2025-01-10 10:00:00", "2026-01-05 10:00:00", "%Y-%m"},
		{"short", "2026-01-05 10:00:00", "%Y-%m-%d %H:00:00"},
	}
	for _, c := range cases {
		if got := histogramBucketFormat(c.min, c.max); got != c.want {
			t.Errorf("histogramBucketFormat(%q, %q) = %q, want %q", c.min, c.max, got, c.want)
		}
	}
}

func TestMergeHistogramBuckets(t *testing.T) {
	got := mergeHistogramBuckets(
		map[string]int64{"2025-01": 3, "2025-03": 1},
		map[string]int64{"2025-02": 2, "2025-03": 4},
	)
	want := []TimelineBucket{
		{Timestamp: "2025-01", Count: 3},
		{Timestamp: "2025-02", NotesCount: 2},
		{Timestamp: "2025-03", Count: 1, NotesCount: 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merge = %+v, want %+v", got, want)
	}
	if got := mergeHistogramBuckets(nil, nil); len(got) != 0 {
		t.Errorf("empty merge = %+v, want none", got)
	}
}

func TestHistogramBucketKey(t *testing.T) {
	cases := []struct{ dt, format, want string }{
		{"2025-01-10 10:42:07", "%Y-%m-%d %H:00:00", "2025-01-10 10:00:00"},
		{"2025-01-10T10:42:07Z", "%Y-%m-%d %H:00:00", "2025-01-10 10:00:00"},
		{"2025-01-10 10:42:07", "%Y-%m-%d", "2025-01-10"},
		{"2025-01-10 10:42:07", "%Y-%m", "2025-01"},
		{"2025", "%Y-%m", "2025"},
	}
	for _, c := range cases {
		if got := histogramBucketKey(c.dt, c.format); got != c.want {
			t.Errorf("histogramBucketKey(%q, %q) = %q, want %q", c.dt, c.format, got, c.want)
		}
	}
}

func TestWindowBuckets(t *testing.T) {
	got := windowBuckets(map[string]int64{"2025-01-09": 1, "2025-01-10": 2, "2025-01-12": 3, "2025-01-13": 4}, "2025-01-10", "2025-01-12")
	want := map[string]int64{"2025-01-10": 2, "2025-01-12": 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("windowBuckets = %v, want %v", got, want)
	}
}

const histTestGuard = "WHERE datetime > '1970-01-01' AND datetime < '2100-01-01'"

// histTestDB holds two RED events on 2025-01-10 and 2025-01-12, a RED note
// later on the last event's day, a RED note outside the evidence range, and a
// note that matches no evidence.
func histTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	db := createTestDB(t)
	events := []*model.Event{
		{Source: "FILE", Datetime: "2025-01-10 08:00:00", Color: "RED"},
		{Source: "FILE", Datetime: "2025-01-12 09:00:00", Color: "RED"},
		{Source: "FILE", Datetime: "2025-01-12 09:30:00", Color: "BLUE"},
	}
	if _, err := db.InsertEvents(events, nil); err != nil {
		t.Fatalf("InsertEvents failed: %v", err)
	}
	for _, n := range []struct{ dt, desc, color string }{
		{"2025-01-12 22:00:00", "same day as the last event", "RED"},
		{"2025-01-20 10:00:00", "after the evidence", "RED"},
		{"2025-03-05 14:10:00", "lone green note", "GREEN"},
	} {
		if _, err := db.InsertExaminerNote(n.dt, n.desc, "", n.color); err != nil {
			t.Fatalf("InsertExaminerNote failed: %v", err)
		}
	}
	return db
}

// The evidence sets the window. A note later on the last evidence day shares
// its daily bucket and counts; a note past the window is dropped, and does not
// coarsen the format.
func TestGetTimelineHistogramEvidenceWindow(t *testing.T) {
	db := histTestDB(t)
	buckets, err := db.GetTimelineHistogram(histTestGuard+" AND (color = ?)", []interface{}{"RED"}, nil)
	if err != nil {
		t.Fatalf("GetTimelineHistogram failed: %v", err)
	}
	want := []TimelineBucket{
		{Timestamp: "2025-01-10", Count: 1},
		{Timestamp: "2025-01-12", Count: 1, NotesCount: 1},
	}
	if !reflect.DeepEqual(buckets, want) {
		t.Errorf("buckets = %+v, want %+v", buckets, want)
	}
}

// An explicit window sets the format and the notes window, so a note inside
// it but past the matching evidence counts. An explicit window with no
// matching rows is still empty.
func TestGetTimelineHistogramExplicitWindow(t *testing.T) {
	db := histTestDB(t)
	window := &HistogramWindow{From: "2025-01-01 00:00:00", To: "2025-01-31 23:59:59"}
	buckets, err := db.GetTimelineHistogram(histTestGuard+" AND (color = ?)", []interface{}{"RED"}, window)
	if err != nil {
		t.Fatalf("GetTimelineHistogram failed: %v", err)
	}
	want := []TimelineBucket{
		{Timestamp: "2025-01-10", Count: 1},
		{Timestamp: "2025-01-12", Count: 1, NotesCount: 1},
		{Timestamp: "2025-01-20", NotesCount: 1},
	}
	if !reflect.DeepEqual(buckets, want) {
		t.Errorf("buckets = %+v, want %+v", buckets, want)
	}

	none, err := db.GetTimelineHistogram(histTestGuard+" AND (color = ?)", []interface{}{"PURPLE"}, window)
	if err != nil {
		t.Fatalf("GetTimelineHistogram failed: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("no match: buckets = %+v, want none", none)
	}
}

// With no matching evidence, the notes' own range is the window.
func TestGetTimelineHistogramNotesFallback(t *testing.T) {
	db := histTestDB(t)
	buckets, err := db.GetTimelineHistogram(histTestGuard+" AND (color = ?)", []interface{}{"GREEN"}, nil)
	if err != nil {
		t.Fatalf("GetTimelineHistogram failed: %v", err)
	}
	want := []TimelineBucket{{Timestamp: "2025-03-05 14:00:00", NotesCount: 1}}
	if !reflect.DeepEqual(buckets, want) {
		t.Errorf("buckets = %+v, want %+v", buckets, want)
	}

	none, err := db.GetTimelineHistogram(histTestGuard+" AND (color = ?)", []interface{}{"PURPLE"}, nil)
	if err != nil {
		t.Fatalf("GetTimelineHistogram failed: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("no match: buckets = %+v, want none", none)
	}
}
