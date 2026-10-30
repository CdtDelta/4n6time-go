package database

import "github.com/cdtdelta/4n6time/internal/model"

// TimelineBucket is one histogram bucket: its timestamp label, the evidence
// count, and the examiner notes count. The two counts are never summed.
type TimelineBucket struct {
	Timestamp  string `json:"timestamp"`
	Count      int64  `json:"count"`
	NotesCount int64  `json:"notesCount"`
}

// HistogramWindow is an explicit histogram range, as "YYYY-MM-DD HH:MM:SS"
// text. When set, it fixes the bucket format and the window that notes must
// fall in, instead of the matching evidence's range.
type HistogramWindow struct {
	From, To string
}

// Store defines the interface for all database operations.
// Every method that the application needs is captured here so that
// app.go depends on the interface, not on a concrete database type.
type Store interface {
	// Event CRUD
	InsertEvent(e *model.Event) error
	InsertEvents(events []*model.Event, onProgress func(int)) (int, error)
	QueryEvents(where string, args []interface{}, orderBy string, limit, offset int) ([]*model.Event, error)
	CountEvents(where string, args []interface{}) (int64, error)
	UpdateEvent(id int64, fields map[string]interface{}) error
	ToggleBookmark(id int64) (int64, error)

	// Query execution for pre-built SQL from the query package, run as given.
	// The scan order matches model.Fields: rowid, datetime, timezone, MACB, ...
	// Whether examiner notes appear is decided by the SQL alone: Build reads
	// log2timeline only, and BuildCombined reads evidence UNION ALL notes.
	ExecuteQuery(sql string, args []interface{}) ([]*model.Event, error)
	ExecuteCountQuery(sql string, args []interface{}) (int64, error)

	// Metadata and filters
	GetDistinctValues(field string) (map[string]int64, error)
	GetDistinctValuesFiltered(field, whereClause string, whereArgs []interface{}) (map[string]int64, error)
	GetDistinctTags() ([]string, error)
	// GetFilterValues returns the unscoped filter panel values for each field
	// from the l2t_* metadata tables and l2t_imports, falling back to the live
	// GetDistinctValues query only when the metadata was never built. A field
	// that fails is omitted and its error joined into the returned error.
	GetFilterValues(fields []string) (map[string][]string, error)
	GetMinMaxDate() (string, string, error)
	GetMinMaxDateFiltered(whereClause string, whereArgs []interface{}) (string, string, error)
	GetTimelineHistogram(whereClause string, whereArgs []interface{}, window *HistogramWindow) ([]TimelineBucket, error)

	// Saved queries
	GetSavedQueries() ([]SavedQuery, error)
	SaveQuery(name, query string) error
	DeleteQuery(name string) error

	// Examiner notes
	InsertExaminerNote(datetime, description, tag, color string) (int64, error)
	DeleteExaminerNote(id int64) error
	UpdateExaminerNoteColor(id int64, color string) error
	ToggleExaminerNoteBookmark(id int64) (int64, error)
	GetExaminerNotes() ([]*model.Event, error)

	// Bulk operations
	BulkUpdateColor(ids []int64, color string) error
	BulkAddTag(ids []int64, tag string) error
	BulkSetBookmark(ids []int64, bookmark int64) error
	BulkUpdateExaminerNoteColor(ids []int64, color string) error
	BulkSetExaminerNoteBookmark(ids []int64, bookmark int64) error

	// Tab session persistence
	SaveTabSession(data string) error
	LoadTabSession() (string, error)

	// Import provenance. CreateImport records a run as in_progress and returns its ID;
	// FinalizeImport sets the event count, a final status (complete or failed),
	// and, for a failed import, its error text (stored in error_message; NULL
	// otherwise). GetImport returns nil, nil when the ID does not exist.
	CreateImport(rec model.ImportRecord) (int64, error)
	FinalizeImport(id int64, eventCount int, status, errorMessage string) error
	GetImports() ([]model.ImportRecord, error)
	GetImport(id int64) (*model.ImportRecord, error)

	// Schema and maintenance
	UpdateMetadata() error
	RebuildIndexes(fields []string) error
	Migrate() error

	// OffsetNeedsUpgrade reports whether log2timeline.offset is a 32-bit INT
	// (PostgreSQL databases created before v0.18.1). UpgradeOffsetColumn widens
	// it to BIGINT. Both are no-ops on SQLite.
	OffsetNeedsUpgrade() (bool, error)
	UpgradeOffsetColumn() error

	// Lifecycle
	Close() error
	Path() string
}
