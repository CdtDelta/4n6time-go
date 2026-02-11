package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cdtdelta/4n6time/internal/csvparser"
	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/query"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct that Wails binds to the frontend.
// All exported methods become callable from JavaScript.
type App struct {
	ctx context.Context
	db  *database.DB
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call runtime methods (dialogs, events, etc.)
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

// -- File Operations --

// CloseDatabase closes the current database and returns to the welcome screen.
func (a *App) CloseDatabase() {
	if a.db != nil {
		a.db.Close()
		a.db = nil
	}
}

// OpenDatabase opens a file dialog and loads an existing SQLite database.
func (a *App) OpenDatabase() (*DBInfo, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open 4n6time Database",
		Filters: []runtime.FileFilter{
			{DisplayName: "SQLite Database (*.db)", Pattern: "*.db"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // user cancelled
	}

	return a.loadDatabase(path)
}

// ImportCSV opens a file dialog for a CSV, creates a new database, and imports events.
func (a *App) ImportCSV() (*DBInfo, error) {
	csvPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import L2T CSV File",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if csvPath == "" {
		return nil, nil
	}

	// Validate the CSV header before doing anything
	if err := csvparser.ValidateHeader(csvPath); err != nil {
		return nil, fmt.Errorf("invalid CSV file: %w", err)
	}

	// Ask where to save the database
	dbPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Database As",
		DefaultFilename: strings.TrimSuffix(filepath.Base(csvPath), filepath.Ext(csvPath)) + ".db",
		Filters: []runtime.FileFilter{
			{DisplayName: "SQLite Database (*.db)", Pattern: "*.db"},
		},
	})
	if err != nil {
		return nil, err
	}
	if dbPath == "" {
		return nil, nil
	}

	// Close any existing database
	if a.db != nil {
		a.db.Close()
		a.db = nil
	}

	// Create the database
	db, err := database.Create(dbPath, nil)
	if err != nil {
		return nil, fmt.Errorf("creating database: %w", err)
	}

	// Read the CSV
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "reading", "message": "Reading CSV file...", "count": 0, "total": 0,
	})
	result, err := csvparser.ReadEvents(csvPath, "", "", 0, func(count int) {
		runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
			"phase": "reading", "message": fmt.Sprintf("Read %d events...", count), "count": count, "total": 0,
		})
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("reading CSV: %w", err)
	}

	// Insert into database
	total := len(result.Events)
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "inserting", "message": "Inserting into database...", "count": 0, "total": total,
	})
	_, err = db.InsertEvents(result.Events, func(count int) {
		runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
			"phase": "inserting", "message": fmt.Sprintf("Inserted %d of %d events...", count, total), "count": count, "total": total,
		})
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("inserting events: %w", err)
	}

	// Update metadata tables
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "metadata", "message": "Building metadata and indexes...", "count": 0, "total": 0,
	})
	if err := db.UpdateMetadata(); err != nil {
		db.Close()
		return nil, fmt.Errorf("updating metadata: %w", err)
	}

	a.db = db
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "done", "message": fmt.Sprintf("Import complete: %d events", total), "count": total, "total": total,
	})

	return a.getDBInfo()
}

// -- Query Operations --

// QueryEventsPage returns a page of events matching the given filters.
type QueryRequest struct {
	Filters  []FilterItem `json:"filters"`
	Logic    string       `json:"logic"`
	OrderBy  string       `json:"orderBy"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

type FilterItem struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type QueryResponse struct {
	Events     []*model.Event `json:"events"`
	TotalCount int64          `json:"totalCount"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
}

func (a *App) QueryEvents(req QueryRequest) (*QueryResponse, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no database open")
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 1000
	}

	q := query.New(pageSize)

	// Set logic
	if req.Logic == "OR" {
		q.SetLogic(query.OR)
	}

	// Add filters
	for _, f := range req.Filters {
		// Handle date range comparisons directly via SQL
		// since the query builder's Simple predicate handles these
		var op query.Operator
		switch f.Operator {
		case "=":
			op = query.Equal
		case "!=":
			op = query.NotEqual
		case "LIKE":
			op = query.Like
		case "NOT LIKE":
			op = query.NotLike
		case ">=":
			op = query.GreaterOrEqual
		case "<=":
			op = query.LessOrEqual
		default:
			continue
		}
		p := query.Simple(f.Field, op, f.Value)
		q.AddPredicate(p)
	}

	// Order by
	if req.OrderBy != "" {
		q.OrderBy(req.OrderBy)
	}

	// Page
	page := req.Page
	if page < 1 {
		page = 1
	}
	q.SetPage(page)

	// Build and execute
	sql, args := q.Build()

	// We need the WHERE clause for counting, so build it from the query
	countSQL, countArgs := q.BuildCount()

	// Get total count
	var totalCount int64
	row := a.db.Conn().QueryRow(countSQL, countArgs...)
	row.Scan(&totalCount)

	// Get events
	rows, err := a.db.Conn().Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []*model.Event
	for rows.Next() {
		e := &model.Event{}
		err := rows.Scan(
			&e.ID, &e.Datetime, &e.Timezone, &e.MACB, &e.Source, &e.SourceType,
			&e.Type, &e.User, &e.Host, &e.Desc, &e.Filename,
			&e.Inode, &e.Notes, &e.Format, &e.Extra, &e.ReportNotes,
			&e.InReport, &e.Tag, &e.Color, &e.Offset, &e.StoreNumber,
			&e.StoreIndex, &e.VSSStoreNumber, &e.URL, &e.RecordNumber,
			&e.EventID, &e.EventType, &e.SourceName, &e.UserSID, &e.ComputerName,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		events = append(events, e)
	}

	return &QueryResponse{
		Events:     events,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// ExportCSV exports the current filtered results to a CSV file.
func (a *App) ExportCSV(req QueryRequest) (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("no database open")
	}

	// Ask where to save
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export to CSV",
		DefaultFilename: "export.csv",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return "", err
	}
	if savePath == "" {
		return "", nil // user cancelled
	}

	// Build query without pagination to get all matching events
	q := query.New(999999999) // effectively unlimited

	if req.Logic == "OR" {
		q.SetLogic(query.OR)
	}

	for _, f := range req.Filters {
		var op query.Operator
		switch f.Operator {
		case "=":
			op = query.Equal
		case "!=":
			op = query.NotEqual
		case "LIKE":
			op = query.Like
		case "NOT LIKE":
			op = query.NotLike
		case ">=":
			op = query.GreaterOrEqual
		case "<=":
			op = query.LessOrEqual
		default:
			continue
		}
		q.AddPredicate(query.Simple(f.Field, op, f.Value))
	}

	orderBy := req.OrderBy
	if orderBy == "" {
		orderBy = "datetime"
	}
	q.OrderBy(orderBy)
	q.SetPage(1)

	sqlStr, args := q.Build()

	runtime.EventsEmit(a.ctx, "export:status", "Querying events...")

	rows, err := a.db.Conn().Query(sqlStr, args...)
	if err != nil {
		return "", fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []*model.Event
	for rows.Next() {
		e := &model.Event{}
		err := rows.Scan(
			&e.ID, &e.Datetime, &e.Timezone, &e.MACB, &e.Source, &e.SourceType,
			&e.Type, &e.User, &e.Host, &e.Desc, &e.Filename,
			&e.Inode, &e.Notes, &e.Format, &e.Extra, &e.ReportNotes,
			&e.InReport, &e.Tag, &e.Color, &e.Offset, &e.StoreNumber,
			&e.StoreIndex, &e.VSSStoreNumber, &e.URL, &e.RecordNumber,
			&e.EventID, &e.EventType, &e.SourceName, &e.UserSID, &e.ComputerName,
		)
		if err != nil {
			return "", fmt.Errorf("scanning row: %w", err)
		}
		events = append(events, e)
	}

	runtime.EventsEmit(a.ctx, "export:status", fmt.Sprintf("Writing %d events to CSV...", len(events)))

	if err := csvparser.WriteEvents(savePath, events); err != nil {
		return "", fmt.Errorf("writing CSV: %w", err)
	}

	runtime.EventsEmit(a.ctx, "export:status", "Done")

	return fmt.Sprintf("Exported %d events to %s", len(events), savePath), nil
}

// -- Metadata Operations --

// GetDistinctValues returns distinct values for a given field (for filter dropdowns).
func (a *App) GetDistinctValues(field string) (map[string]int64, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no database open")
	}
	return a.db.GetDistinctValues(field)
}

// GetMinMaxDate returns the date range of events in the database.
func (a *App) GetMinMaxDate() ([]string, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no database open")
	}
	min, max, err := a.db.GetMinMaxDate()
	if err != nil {
		return nil, err
	}
	return []string{min, max}, nil
}

// TimelineBucket represents a single histogram bucket.
type TimelineBucket struct {
	Timestamp string `json:"timestamp"`
	Count     int64  `json:"count"`
}

// GetTimelineHistogram returns event counts bucketed by time interval.
// The bucket size is automatically chosen based on the date range.
func (a *App) GetTimelineHistogram(req QueryRequest) ([]TimelineBucket, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no database open")
	}

	// Build WHERE clause from filters
	var whereParts []string
	var whereArgs []interface{}

	// Always exclude junk dates (zeroed, pre-epoch, far-future)
	whereParts = append(whereParts, "datetime > '1970-01-01' AND datetime < '2100-01-01'")

	for _, f := range req.Filters {
		switch f.Operator {
		case "=", "!=", "LIKE", "NOT LIKE", ">=", "<=":
			whereParts = append(whereParts, fmt.Sprintf("%s %s ?", f.Field, f.Operator))
			whereArgs = append(whereArgs, f.Value)
		}
	}

	logic := "AND"
	if req.Logic == "OR" {
		logic = "OR"
	}

	whereClause := ""
	// The first part (junk date filter) is always AND
	// User filters get their own logic
	if len(whereParts) == 1 {
		// Only the junk date filter, no user filters
		whereClause = "WHERE " + whereParts[0]
	} else {
		// Junk date filter AND (user filters with their logic)
		userParts := whereParts[1:]
		whereClause = "WHERE " + whereParts[0] + " AND (" + strings.Join(userParts, " "+logic+" ") + ")"
	}

	// Get date range to determine bucket size
	rangeSQL := "SELECT MIN(datetime), MAX(datetime) FROM log2timeline " + whereClause
	var minDate, maxDate string
	row := a.db.Conn().QueryRow(rangeSQL, whereArgs...)
	if err := row.Scan(&minDate, &maxDate); err != nil {
		return nil, fmt.Errorf("getting date range: %w", err)
	}

	if minDate == "" || maxDate == "" {
		return []TimelineBucket{}, nil
	}

	// Choose bucket format based on date range span
	bucketFormat := "%Y-%m-%d %H:00:00" // default: hourly

	if len(minDate) >= 10 && len(maxDate) >= 10 {
		minDay := minDate[:10]
		maxDay := maxDate[:10]

		if minDay == maxDay {
			bucketFormat = "%Y-%m-%d %H:00:00"
		} else {
			minYM := minDate[:7]
			maxYM := maxDate[:7]
			if minYM == maxYM {
				bucketFormat = "%Y-%m-%d"
			} else {
				minYear := minDate[:4]
				maxYear := maxDate[:4]
				if minYear != maxYear {
					bucketFormat = "%Y-%m"
				} else {
					bucketFormat = "%Y-%m-%d"
				}
			}
		}
	}

	// Build and run histogram query
	histSQL := "SELECT strftime('" + bucketFormat + "', datetime) as bucket, COUNT(*) as cnt FROM log2timeline " + whereClause + " GROUP BY bucket ORDER BY bucket"

	rows, err := a.db.Conn().Query(histSQL, whereArgs...)
	if err != nil {
		return nil, fmt.Errorf("histogram query: %w", err)
	}
	defer rows.Close()

	var buckets []TimelineBucket
	for rows.Next() {
		var b TimelineBucket
		if err := rows.Scan(&b.Timestamp, &b.Count); err != nil {
			return nil, fmt.Errorf("scanning bucket: %w", err)
		}
		buckets = append(buckets, b)
	}

	return buckets, rows.Err()
}

// GetTags returns all distinct tags.
func (a *App) GetTags() ([]string, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no database open")
	}
	return a.db.GetDistinctTags()
}

// -- Event Operations --

// UpdateEventFields updates specific fields on an event.
func (a *App) UpdateEventFields(rowid int64, fields map[string]interface{}) error {
	if a.db == nil {
		return fmt.Errorf("no database open")
	}
	return a.db.UpdateEvent(rowid, fields)
}

// -- Saved Queries --

// GetSavedQueries returns all saved queries.
func (a *App) GetSavedQueries() ([]database.SavedQuery, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no database open")
	}
	return a.db.GetSavedQueries()
}

// SaveQuery stores a named query.
func (a *App) SaveQuery(name, queryStr string) error {
	if a.db == nil {
		return fmt.Errorf("no database open")
	}
	return a.db.SaveQuery(name, queryStr)
}

// DeleteSavedQuery removes a saved query.
func (a *App) DeleteSavedQuery(name string) error {
	if a.db == nil {
		return fmt.Errorf("no database open")
	}
	return a.db.DeleteQuery(name)
}

// -- Internal Helpers --

// GetVersion returns the application version string.
func (a *App) GetVersion() string {
	return Version
}

// DBInfo contains summary info about the loaded database.
type DBInfo struct {
	Path       string `json:"path"`
	EventCount int64  `json:"eventCount"`
	MinDate    string `json:"minDate"`
	MaxDate    string `json:"maxDate"`
}

func (a *App) loadDatabase(path string) (*DBInfo, error) {
	// Close any existing database
	if a.db != nil {
		a.db.Close()
		a.db = nil
	}

	db, err := database.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	a.db = db
	return a.getDBInfo()
}

func (a *App) getDBInfo() (*DBInfo, error) {
	count, err := a.db.CountEvents("", nil)
	if err != nil {
		return nil, err
	}

	min, max, err := a.db.GetMinMaxDate()
	if err != nil {
		// Not fatal, just means empty db
		min = ""
		max = ""
	}

	return &DBInfo{
		Path:       a.db.Path(),
		EventCount: count,
		MinDate:    min,
		MaxDate:    max,
	}, nil
}
