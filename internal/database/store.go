package database

import "github.com/cdtdelta/4n6time/internal/model"

// TimelineBucket represents a single histogram bucket with a timestamp label and event count.
type TimelineBucket struct {
	Timestamp string `json:"timestamp"`
	Count     int64  `json:"count"`
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

	// Query execution for pre-built SQL (from query.go Build).
	// The scan order matches model.Fields: rowid, datetime, timezone, MACB, ...
	ExecuteQuery(sql string, args []interface{}) ([]*model.Event, error)
	ExecuteCountQuery(sql string, args []interface{}) (int64, error)

	// Metadata and filters
	GetDistinctValues(field string) (map[string]int64, error)
	GetDistinctTags() ([]string, error)
	GetMinMaxDate() (string, string, error)
	GetTimelineHistogram(whereClause string, whereArgs []interface{}) ([]TimelineBucket, error)

	// Saved queries
	GetSavedQueries() ([]SavedQuery, error)
	SaveQuery(name, query string) error
	DeleteQuery(name string) error

	// Schema and maintenance
	UpdateMetadata() error
	RebuildIndexes(fields []string) error
	Migrate() error

	// Lifecycle
	Close() error
	Path() string
}
