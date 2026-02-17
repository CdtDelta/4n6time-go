package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

func tempDBPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.db")
}

func createTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	db, err := CreateSQLite(tempDBPath(t), nil)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func sampleEvent() *model.Event {
	return &model.Event{
		Timezone:       "UTC",
		MACB:           "MACB",
		Source:         "FILE",
		SourceType:     "OS:NTFS:MFT",
		Type:           "Last Written",
		User:           "admin",
		Host:           "WORKSTATION1",
		Desc:           "test file entry",
		Filename:       "/Users/admin/test.txt",
		Inode:          "12345",
		Notes:          "",
		Format:         "mft",
		Extra:          "",
		Datetime:       "2025-01-15 10:30:00",
		ReportNotes:    "",
		InReport:       "",
		Tag:            "",
		Color:          "",
		Offset:         0,
		StoreNumber:    -1,
		StoreIndex:     -1,
		VSSStoreNumber: -1,
		URL:            "",
		RecordNumber:   "1001",
		EventID:        "",
		EventType:      "",
		SourceName:     "",
		UserSID:        "S-1-5-21-123456",
		ComputerName:   "WORKSTATION1",
	}
}

func TestCreateAndOpen(t *testing.T) {
	path := tempDBPath(t)

	// Create a new database
	db, err := CreateSQLite(path, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	db.Close()

	// Verify the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	// Reopen it
	db2, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db2.Close()

	// Verify we can query it
	count, err := db2.CountEvents("", nil)
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 events, got %d", count)
	}
}

func TestInsertAndQueryEvent(t *testing.T) {
	db := createTestDB(t)
	e := sampleEvent()

	// Insert one event
	err := db.InsertEvent(e)
	if err != nil {
		t.Fatalf("InsertEvent failed: %v", err)
	}

	// Count should be 1
	count, err := db.CountEvents("", nil)
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}

	// Query it back
	events, err := db.QueryEvents("", nil, "", 0, 0)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.Source != "FILE" {
		t.Errorf("expected Source 'FILE', got '%s'", got.Source)
	}
	if got.Host != "WORKSTATION1" {
		t.Errorf("expected Host 'WORKSTATION1', got '%s'", got.Host)
	}
	if got.Datetime != "2025-01-15 10:30:00" && got.Datetime != "2025-01-15T10:30:00Z" {
		t.Errorf("expected Datetime '2025-01-15 10:30:00', got '%s'", got.Datetime)
	}
	if got.ID == 0 {
		t.Error("expected non-zero rowid")
	}
}

func TestInsertBatch(t *testing.T) {
	db := createTestDB(t)

	events := make([]*model.Event, 100)
	for i := range events {
		e := sampleEvent()
		e.Host = "HOST" + string(rune('A'+i%26))
		events[i] = e
	}

	var progressCalls int
	inserted, err := db.InsertEvents(events, func(count int) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("InsertEvents failed: %v", err)
	}
	if inserted != 100 {
		t.Errorf("expected 100 inserted, got %d", inserted)
	}

	count, err := db.CountEvents("", nil)
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}
	if count != 100 {
		t.Errorf("expected 100 events, got %d", count)
	}
}

func TestQueryWithFilter(t *testing.T) {
	db := createTestDB(t)

	// Insert events with different sources
	for _, src := range []string{"FILE", "REG", "EVT", "FILE", "FILE"} {
		e := sampleEvent()
		e.Source = src
		if err := db.InsertEvent(e); err != nil {
			t.Fatalf("InsertEvent failed: %v", err)
		}
	}

	// Filter by source
	events, err := db.QueryEvents("source = ?", []interface{}{"FILE"}, "", 0, 0)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 FILE events, got %d", len(events))
	}

	// Count with filter
	count, err := db.CountEvents("source = ?", []interface{}{"REG"})
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 REG event, got %d", count)
	}
}

func TestQueryPagination(t *testing.T) {
	db := createTestDB(t)

	// Insert 25 events
	for i := 0; i < 25; i++ {
		e := sampleEvent()
		if err := db.InsertEvent(e); err != nil {
			t.Fatalf("InsertEvent failed: %v", err)
		}
	}

	// Get first page of 10
	page1, err := db.QueryEvents("", nil, "rowid", 10, 0)
	if err != nil {
		t.Fatalf("page 1 query failed: %v", err)
	}
	if len(page1) != 10 {
		t.Errorf("expected 10 events on page 1, got %d", len(page1))
	}

	// Get second page of 10
	page2, err := db.QueryEvents("", nil, "rowid", 10, 10)
	if err != nil {
		t.Fatalf("page 2 query failed: %v", err)
	}
	if len(page2) != 10 {
		t.Errorf("expected 10 events on page 2, got %d", len(page2))
	}

	// Get third page (should have 5)
	page3, err := db.QueryEvents("", nil, "rowid", 10, 20)
	if err != nil {
		t.Fatalf("page 3 query failed: %v", err)
	}
	if len(page3) != 5 {
		t.Errorf("expected 5 events on page 3, got %d", len(page3))
	}

	// Verify pages don't overlap
	if page1[0].ID == page2[0].ID {
		t.Error("page 1 and page 2 returned the same first event")
	}
}

func TestGetMinMaxDate(t *testing.T) {
	db := createTestDB(t)

	e1 := sampleEvent()
	e1.Datetime = "2025-01-01 00:00:00"
	e2 := sampleEvent()
	e2.Datetime = "2025-06-15 12:00:00"
	e3 := sampleEvent()
	e3.Datetime = "0000-00-00 00:00:00" // sentinel, should be excluded

	for _, e := range []*model.Event{e1, e2, e3} {
		if err := db.InsertEvent(e); err != nil {
			t.Fatalf("InsertEvent failed: %v", err)
		}
	}

	minDate, maxDate, err := db.GetMinMaxDate()
	if err != nil {
		t.Fatalf("GetMinMaxDate failed: %v", err)
	}
	if minDate != "2025-01-01 00:00:00" {
		t.Errorf("expected min '2025-01-01 00:00:00', got '%s'", minDate)
	}
	if maxDate != "2025-06-15 12:00:00" {
		t.Errorf("expected max '2025-06-15 12:00:00', got '%s'", maxDate)
	}
}

func TestGetDistinctValues(t *testing.T) {
	db := createTestDB(t)

	for _, src := range []string{"FILE", "REG", "EVT", "FILE", "FILE", "REG"} {
		e := sampleEvent()
		e.Source = src
		if err := db.InsertEvent(e); err != nil {
			t.Fatalf("InsertEvent failed: %v", err)
		}
	}

	vals, err := db.GetDistinctValues("source")
	if err != nil {
		t.Fatalf("GetDistinctValues failed: %v", err)
	}

	if vals["FILE"] != 3 {
		t.Errorf("expected FILE count 3, got %d", vals["FILE"])
	}
	if vals["REG"] != 2 {
		t.Errorf("expected REG count 2, got %d", vals["REG"])
	}
	if vals["EVT"] != 1 {
		t.Errorf("expected EVT count 1, got %d", vals["EVT"])
	}
}

func TestGetDistinctValuesInvalidField(t *testing.T) {
	db := createTestDB(t)

	_, err := db.GetDistinctValues("DROP TABLE log2timeline;--")
	if err == nil {
		t.Fatal("expected error for invalid field name, got nil")
	}
}

func TestGetDistinctTags(t *testing.T) {
	db := createTestDB(t)

	events := []struct {
		tag string
	}{
		{"malware"},
		{"suspicious,lateral_movement"},
		{"malware"},
		{""},
		{"lateral_movement,persistence"},
	}

	for _, tc := range events {
		e := sampleEvent()
		e.Tag = tc.tag
		if err := db.InsertEvent(e); err != nil {
			t.Fatalf("InsertEvent failed: %v", err)
		}
	}

	tags, err := db.GetDistinctTags()
	if err != nil {
		t.Fatalf("GetDistinctTags failed: %v", err)
	}

	expected := map[string]bool{"malware": true, "suspicious": true, "lateral_movement": true, "persistence": true}
	if len(tags) != len(expected) {
		t.Errorf("expected %d tags, got %d: %v", len(expected), len(tags), tags)
	}
	for _, tag := range tags {
		if !expected[tag] {
			t.Errorf("unexpected tag: %s", tag)
		}
	}
}

func TestUpdateEvent(t *testing.T) {
	db := createTestDB(t)

	e := sampleEvent()
	if err := db.InsertEvent(e); err != nil {
		t.Fatalf("InsertEvent failed: %v", err)
	}

	events, _ := db.QueryEvents("", nil, "", 0, 0)
	rowid := events[0].ID

	// Update color and tag
	err := db.UpdateEvent(rowid, map[string]interface{}{
		"color": "RED",
		"tag":   "suspicious",
	})
	if err != nil {
		t.Fatalf("UpdateEvent failed: %v", err)
	}

	// Query it back
	updated, _ := db.QueryEvents("rowid = ?", []interface{}{rowid}, "", 0, 0)
	if updated[0].Color != "RED" {
		t.Errorf("expected color 'RED', got '%s'", updated[0].Color)
	}
	if updated[0].Tag != "suspicious" {
		t.Errorf("expected tag 'suspicious', got '%s'", updated[0].Tag)
	}
}

func TestUpdateEventInvalidField(t *testing.T) {
	db := createTestDB(t)

	err := db.UpdateEvent(1, map[string]interface{}{
		"DROP TABLE log2timeline;--": "gotcha",
	})
	if err == nil {
		t.Fatal("expected error for invalid field name, got nil")
	}
}

func TestUpdateMetadata(t *testing.T) {
	db := createTestDB(t)

	for _, src := range []string{"FILE", "REG", "FILE"} {
		e := sampleEvent()
		e.Source = src
		if err := db.InsertEvent(e); err != nil {
			t.Fatalf("InsertEvent failed: %v", err)
		}
	}

	err := db.UpdateMetadata()
	if err != nil {
		t.Fatalf("UpdateMetadata failed: %v", err)
	}

	// Verify l2t_sources was populated
	rows, err := db.conn.Query("SELECT source, frequency FROM l2t_sources ORDER BY source")
	if err != nil {
		t.Fatalf("querying l2t_sources failed: %v", err)
	}
	defer rows.Close()

	results := make(map[string]int64)
	for rows.Next() {
		var name string
		var freq int64
		rows.Scan(&name, &freq)
		results[name] = freq
	}

	if results["FILE"] != 2 {
		t.Errorf("expected FILE frequency 2, got %d", results["FILE"])
	}
	if results["REG"] != 1 {
		t.Errorf("expected REG frequency 1, got %d", results["REG"])
	}
}

func TestSavedQueries(t *testing.T) {
	db := createTestDB(t)

	// Save a query
	err := db.SaveQuery("Find malware", "tag LIKE '%malware%'")
	if err != nil {
		t.Fatalf("SaveQuery failed: %v", err)
	}

	// Save another
	err = db.SaveQuery("Recent events", "datetime > '2025-01-01'")
	if err != nil {
		t.Fatalf("SaveQuery failed: %v", err)
	}

	// Retrieve all
	queries, err := db.GetSavedQueries()
	if err != nil {
		t.Fatalf("GetSavedQueries failed: %v", err)
	}
	if len(queries) != 2 {
		t.Fatalf("expected 2 saved queries, got %d", len(queries))
	}

	// Delete one
	err = db.DeleteQuery("Find malware")
	if err != nil {
		t.Fatalf("DeleteQuery failed: %v", err)
	}

	queries, _ = db.GetSavedQueries()
	if len(queries) != 1 {
		t.Errorf("expected 1 saved query after delete, got %d", len(queries))
	}
}

func TestRebuildIndexes(t *testing.T) {
	db := createTestDB(t)

	// Rebuild with a different set of fields
	err := db.RebuildIndexes([]string{"source", "datetime", "tag"})
	if err != nil {
		t.Fatalf("RebuildIndexes failed: %v", err)
	}

	// Verify indexes exist by checking sqlite_master
	rows, err := db.conn.Query("SELECT name FROM sqlite_master WHERE type='index' AND name LIKE '%_idx'")
	if err != nil {
		t.Fatalf("querying indexes failed: %v", err)
	}
	defer rows.Close()

	indexes := make(map[string]bool)
	for rows.Next() {
		var name string
		rows.Scan(&name)
		indexes[name] = true
	}

	if !indexes["source_idx"] {
		t.Error("expected source_idx to exist")
	}
	if !indexes["datetime_idx"] {
		t.Error("expected datetime_idx to exist")
	}
	if !indexes["tag_idx"] {
		t.Error("expected tag_idx to exist")
	}
	if indexes["host_idx"] {
		t.Error("expected host_idx to be dropped")
	}
}
