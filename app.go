package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cdtdelta/4n6time/internal/csvparser"
	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/dynamicparser"
	"github.com/cdtdelta/4n6time/internal/jsonlparser"
	"github.com/cdtdelta/4n6time/internal/model"
	"github.com/cdtdelta/4n6time/internal/query"
	"github.com/cdtdelta/4n6time/internal/tlnparser"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct that Wails binds to the frontend.
// All exported methods become callable from JavaScript.
type App struct {
	ctx    context.Context
	store  database.Store
	driver string // "sqlite" or "postgres"

	// Logging
	logFile    *os.File
	logEnabled bool
	logPath    string
	logPersist bool
	logMu      sync.Mutex
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// -- Logging Infrastructure --

// loggingConfig is the persistent logging configuration stored in the user's config directory.
type loggingConfig struct {
	Enabled  bool   `json:"enabled"`
	FilePath string `json:"filePath"`
	Persist  bool   `json:"persist"`
}

// LoggingStatus is returned to the frontend to show current logging state.
type LoggingStatus struct {
	Enabled  bool   `json:"enabled"`
	FilePath string `json:"filePath"`
	Persist  bool   `json:"persist"`
}

func loggingConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "4n6time", "logging.json")
}

func (a *App) logWrite(level, msg string) {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	if !a.logEnabled || a.logFile == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(a.logFile, "%s [%s] %s\n", ts, level, msg)
}

func (a *App) logInfo(msg string)  { a.logWrite("INFO", msg) }
func (a *App) logError(msg string) { a.logWrite("ERROR", msg) }

func (a *App) loadLoggingConfig() {
	path := loggingConfigPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg loggingConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	a.logPersist = cfg.Persist
	if cfg.Persist && cfg.Enabled && cfg.FilePath != "" {
		f, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		a.logFile = f
		a.logEnabled = true
		a.logPath = cfg.FilePath
	}
}

func (a *App) saveLoggingConfig() {
	path := loggingConfigPath()
	if path == "" {
		return
	}
	cfg := loggingConfig{
		Enabled:  a.logEnabled,
		FilePath: a.logPath,
		Persist:  a.logPersist,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, data, 0644)
}

// maskConnStr masks the password in a PostgreSQL connection string for safe logging.
func maskConnStr(connStr string) string {
	if idx := strings.Index(connStr, "://"); idx >= 0 {
		rest := connStr[idx+3:]
		if atIdx := strings.Index(rest, "@"); atIdx >= 0 {
			userPart := rest[:atIdx]
			if colonIdx := strings.Index(userPart, ":"); colonIdx >= 0 {
				return connStr[:idx+3] + userPart[:colonIdx+1] + "****" + rest[atIdx:]
			}
		}
	}
	return connStr
}

// startup is called when the app starts. The context is saved
// so we can call runtime methods (dialogs, events, etc.)
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadLoggingConfig()
	a.logInfo("Application started, version " + Version)
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	a.logInfo("Application shutting down")
	if a.store != nil {
		a.store.Close()
	}
	a.logMu.Lock()
	if a.logFile != nil {
		a.logFile.Close()
		a.logFile = nil
	}
	a.logMu.Unlock()
}

// -- File Operations --

// CloseDatabase closes the current database and returns to the welcome screen.
func (a *App) CloseDatabase() {
	if a.store != nil {
		if a.driver == "postgres" {
			a.logInfo("PostgreSQL disconnected")
		} else {
			a.logInfo("Database closed: " + a.store.Path())
		}
		a.store.Close()
		a.store = nil
		a.driver = ""
	}
}

// OpenDatabase opens a file dialog and loads an existing database.
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

// ImportCSV opens a file dialog for a CSV or JSONL file, creates a new database, and imports events.
func (a *App) ImportCSV() (*DBInfo, error) {
	csvPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import Timeline File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Timeline Files (*.csv, *.jsonl, *.tln, *.l2ttln, *.txt)", Pattern: "*.csv;*.jsonl;*.tln;*.l2ttln;*.txt"},
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "JSONL Files (*.jsonl)", Pattern: "*.jsonl"},
			{DisplayName: "TLN Files (*.tln, *.l2ttln, *.txt)", Pattern: "*.tln;*.l2ttln;*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if csvPath == "" {
		return nil, nil
	}

	ext := strings.ToLower(filepath.Ext(csvPath))
	isJSONL := ext == ".jsonl" || ext == ".json"

	// Detect format: JSONL, TLN/L2TTLN, dynamic CSV, or L2T CSV
	// Try validation in order of specificity
	formatName := ""
	if isJSONL {
		if err := jsonlparser.ValidateFile(csvPath); err != nil {
			return nil, fmt.Errorf("invalid JSONL file: %w", err)
		}
		formatName = "JSONL"
	} else if ext == ".tln" || ext == ".l2ttln" {
		if err := tlnparser.ValidateFile(csvPath); err != nil {
			return nil, fmt.Errorf("invalid TLN file: %w", err)
		}
		formatName = "TLN"
	} else {
		// Try L2T CSV first (fixed 17-column format)
		if err := csvparser.ValidateHeader(csvPath); err == nil {
			formatName = "CSV"
		} else if tlnErr := tlnparser.ValidateFile(csvPath); tlnErr == nil {
			// Could be TLN with .txt or .csv extension
			formatName = "TLN"
		} else if dynErr := dynamicparser.ValidateFile(csvPath); dynErr == nil {
			formatName = "Dynamic CSV"
		} else {
			return nil, fmt.Errorf("unrecognized file format: not a valid L2T CSV, JSONL, TLN, or dynamic CSV file")
		}
	}

	importStart := time.Now()
	a.logInfo("Import started: " + formatName + " from " + csvPath)

	// Determine target store: import into existing database or create new SQLite
	var store database.Store
	importIntoExisting := a.store != nil && (a.driver == "postgres" || a.driver == "sqlite")

	if !importIntoExisting {
		// No database open: prompt for new SQLite file path
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
		if a.store != nil {
			a.store.Close()
			a.store = nil
		}

		var createErr error
		store, createErr = database.CreateStore("sqlite", dbPath, nil)
		if createErr != nil {
			return nil, fmt.Errorf("creating database: %w", createErr)
		}
	} else {
		store = a.store
	}

	// Read the file
	var events []*model.Event

	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "reading", "message": "Reading " + formatName + " file...", "count": 0, "total": 0,
	})

	progressCallback := func(count int) {
		runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
			"phase": "reading", "message": fmt.Sprintf("Read %d events...", count), "count": count, "total": 0,
		})
	}

	// closeOnError closes the store only if we created a new one (not for existing databases)
	closeOnError := func() {
		if !importIntoExisting {
			store.Close()
		}
	}

	switch formatName {
	case "JSONL":
		result, err := jsonlparser.ReadEvents(csvPath, progressCallback)
		if err != nil {
			closeOnError()
			return nil, fmt.Errorf("reading JSONL: %w", err)
		}
		events = result.Events

	case "TLN":
		result, err := tlnparser.ReadEvents(csvPath, progressCallback)
		if err != nil {
			closeOnError()
			return nil, fmt.Errorf("reading TLN: %w", err)
		}
		events = result.Events

	case "Dynamic CSV":
		result, err := dynamicparser.ReadEvents(csvPath, progressCallback)
		if err != nil {
			closeOnError()
			return nil, fmt.Errorf("reading dynamic CSV: %w", err)
		}
		events = result.Events

	case "CSV":
		result, err := csvparser.ReadEvents(csvPath, "", "", 0, progressCallback)
		if err != nil {
			closeOnError()
			return nil, fmt.Errorf("reading CSV: %w", err)
		}
		events = result.Events

	default:
		closeOnError()
		return nil, fmt.Errorf("unknown format: %s", formatName)
	}

	// Insert into database
	total := len(events)
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "inserting", "message": "Inserting into database...", "count": 0, "total": total,
	})
	_, err = store.InsertEvents(events, func(count int) {
		runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
			"phase": "inserting", "message": fmt.Sprintf("Inserted %d of %d events...", count, total), "count": count, "total": total,
		})
	})
	if err != nil {
		closeOnError()
		return nil, fmt.Errorf("inserting events: %w", err)
	}

	// Update metadata tables
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "metadata", "message": "Building metadata and indexes...", "count": 0, "total": 0,
	})
	if err := store.UpdateMetadata(); err != nil {
		closeOnError()
		return nil, fmt.Errorf("updating metadata: %w", err)
	}
	a.logInfo("Metadata update complete")

	if !importIntoExisting {
		a.store = store
		a.driver = "sqlite"
	}
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "done", "message": fmt.Sprintf("Import complete: %d events", total), "count": total, "total": total,
	})
	a.logInfo(fmt.Sprintf("Import complete: %d %s events in %s", total, formatName, time.Since(importStart).Round(time.Millisecond)))

	return a.getDBInfo()
}

// -- Query Operations --

// QueryEventsPage returns a page of events matching the given filters.
type QueryRequest struct {
	Filters      []FilterItem `json:"filters"`
	Logic        string       `json:"logic"`
	OrderBy      string       `json:"orderBy"`
	Page         int          `json:"page"`
	PageSize     int          `json:"pageSize"`
	SearchText   string       `json:"searchText"`
	BookmarkOnly bool         `json:"bookmarkOnly"`
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
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 1000
	}

	q := query.New(pageSize)
	q.SetDialect(a.queryDialect())

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
		// Normalize partial dates for datetime fields
		val := f.Value
		if f.Field == "datetime" {
			val = normalizeDate(val, op == query.LessOrEqual)
		}
		p := query.Simple(f.Field, op, val)
		q.AddPredicate(p)
	}

	// Full-text search across key columns
	if req.SearchText != "" {
		searchFields := []string{
			"desc", "filename", "source", "sourcetype", "type",
			"user", "host", "extra", "tag", "url", "source_name",
			"computer_name", "format", "notes",
		}
		var searchPreds []*query.Predicate
		for _, field := range searchFields {
			p := query.Simple(field, query.Like, req.SearchText)
			if p != nil {
				searchPreds = append(searchPreds, p)
			}
		}
		if len(searchPreds) > 0 {
			combined := query.Combine(searchPreds, query.OR)
			q.AddPredicate(combined)
		}
	}

	// Bookmark filter
	if req.BookmarkOnly {
		p := query.Simple("bookmark", query.Equal, "1")
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
	sqlStr, args := q.Build()
	countSQL, countArgs := q.BuildCount()

	// Get total count (ignore error to preserve existing behavior: count stays 0 on failure)
	totalCount, _ := a.store.ExecuteCountQuery(countSQL, countArgs)

	// Get events
	events, err := a.store.ExecuteQuery(sqlStr, args)
	if err != nil {
		a.logError("Query error: " + err.Error())
		return nil, fmt.Errorf("querying events: %w", err)
	}

	return &QueryResponse{
		Events:     events,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// AdvancedSearch executes a raw WHERE clause query with pagination.
// Returns the same result format as QueryEvents.
func (a *App) AdvancedSearch(whereClause string, page, pageSize int) (*QueryResponse, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}
	if pageSize <= 0 {
		pageSize = 1000
	}
	if page < 1 {
		page = 1
	}

	// On PostgreSQL, auto-quote reserved word column names so users don't have to
	if a.driver == "postgres" {
		whereClause = quotePostgresReservedWords(whereClause)
	}

	rq := query.NewRaw(pageSize, whereClause)
	rq.SetDialect(a.queryDialect())
	rq.SetPage(page)
	rq.OrderBy("datetime")

	sqlStr, args := rq.Build()
	countSQL, countArgs := rq.BuildCount()

	totalCount, err := a.store.ExecuteCountQuery(countSQL, countArgs)
	if err != nil {
		a.logError("Advanced search count error: " + err.Error())
		return nil, fmt.Errorf("count query error: %w", err)
	}

	events, err := a.store.ExecuteQuery(sqlStr, args)
	if err != nil {
		a.logError("Advanced search error: " + err.Error())
		return nil, fmt.Errorf("query error: %w", err)
	}

	return &QueryResponse{
		Events:     events,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// quotePostgresReservedWords replaces standalone occurrences of desc, user, and
// offset with their double-quoted versions ("desc", "user", "offset") so that
// PostgreSQL accepts them as column names. Only text outside single-quoted string
// literals is processed; quoted values are left untouched.
func quotePostgresReservedWords(where string) string {
	// Split on single quotes. Even-indexed segments (0, 2, 4, ...) are outside
	// string literals; odd-indexed segments are inside.
	parts := strings.Split(where, "'")
	re := regexp.MustCompile(`(?i)\b(desc|user|offset)\b`)
	for i := 0; i < len(parts); i += 2 {
		parts[i] = re.ReplaceAllStringFunc(parts[i], func(match string) string {
			return `"` + strings.ToLower(match) + `"`
		})
	}
	return strings.Join(parts, "'")
}

// splitIDs separates a slice of IDs into positive (regular event) and
// negative (examiner note) groups. Negative IDs are negated back to positive
// for use with the examiner_notes table.
func splitIDs(ids []int64) (regular, examiner []int64) {
	for _, id := range ids {
		if id > 0 {
			regular = append(regular, id)
		} else if id < 0 {
			examiner = append(examiner, -id)
		}
	}
	return
}

// BulkUpdateColor applies a color to multiple events. Positive IDs update
// log2timeline; negative IDs update examiner_notes.
func (a *App) BulkUpdateColor(ids []int64, color string) error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	regular, examiner := splitIDs(ids)
	if len(regular) > 0 {
		if err := a.store.BulkUpdateColor(regular, color); err != nil {
			return fmt.Errorf("bulk update color: %w", err)
		}
	}
	if len(examiner) > 0 {
		if err := a.store.BulkUpdateExaminerNoteColor(examiner, color); err != nil {
			return fmt.Errorf("bulk update examiner note color: %w", err)
		}
	}
	a.logInfo(fmt.Sprintf("Bulk color update: %d events, %d examiner notes, color=%s", len(regular), len(examiner), color))
	return nil
}

// BulkAddTag appends a tag to multiple events. Only positive IDs (regular events)
// are updated; examiner note tags are immutable.
func (a *App) BulkAddTag(ids []int64, tag string) error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	regular, _ := splitIDs(ids)
	if len(regular) > 0 {
		if err := a.store.BulkAddTag(regular, tag); err != nil {
			return fmt.Errorf("bulk add tag: %w", err)
		}
	}
	a.logInfo(fmt.Sprintf("Bulk tag add: %d events, tag=%s", len(regular), tag))
	return nil
}

// BulkSetBookmark sets the bookmark value on multiple events. Positive IDs update
// log2timeline; negative IDs update examiner_notes.
func (a *App) BulkSetBookmark(ids []int64, value int64) error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	regular, examiner := splitIDs(ids)
	if len(regular) > 0 {
		if err := a.store.BulkSetBookmark(regular, value); err != nil {
			return fmt.Errorf("bulk set bookmark: %w", err)
		}
	}
	if len(examiner) > 0 {
		if err := a.store.BulkSetExaminerNoteBookmark(examiner, value); err != nil {
			return fmt.Errorf("bulk set examiner note bookmark: %w", err)
		}
	}
	a.logInfo(fmt.Sprintf("Bulk bookmark: %d events, %d examiner notes, value=%d", len(regular), len(examiner), value))
	return nil
}

// ExportCSV exports the current filtered results to a CSV file.
func (a *App) ExportCSV(req QueryRequest) (string, error) {
	if a.store == nil {
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
	q.SetDialect(a.queryDialect())

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
		// Normalize partial dates for datetime fields
		val := f.Value
		if f.Field == "datetime" {
			val = normalizeDate(val, op == query.LessOrEqual)
		}
		q.AddPredicate(query.Simple(f.Field, op, val))
	}

	// Full-text search across key columns
	if req.SearchText != "" {
		searchFields := []string{
			"desc", "filename", "source", "sourcetype", "type",
			"user", "host", "extra", "tag", "url", "source_name",
			"computer_name", "format", "notes",
		}
		var searchPreds []*query.Predicate
		for _, field := range searchFields {
			p := query.Simple(field, query.Like, req.SearchText)
			if p != nil {
				searchPreds = append(searchPreds, p)
			}
		}
		if len(searchPreds) > 0 {
			combined := query.Combine(searchPreds, query.OR)
			q.AddPredicate(combined)
		}
	}

	// Bookmark filter
	if req.BookmarkOnly {
		p := query.Simple("bookmark", query.Equal, "1")
		q.AddPredicate(p)
	}

	orderBy := req.OrderBy
	if orderBy == "" {
		orderBy = "datetime"
	}
	q.OrderBy(orderBy)
	q.SetPage(1)

	sqlStr, args := q.Build()

	runtime.EventsEmit(a.ctx, "export:status", "Querying events...")

	events, err := a.store.ExecuteQuery(sqlStr, args)
	if err != nil {
		return "", fmt.Errorf("querying events: %w", err)
	}

	runtime.EventsEmit(a.ctx, "export:status", fmt.Sprintf("Writing %d events to CSV...", len(events)))

	if err := csvparser.WriteEvents(savePath, events); err != nil {
		return "", fmt.Errorf("writing CSV: %w", err)
	}

	runtime.EventsEmit(a.ctx, "export:status", "Done")

	a.logInfo(fmt.Sprintf("Export CSV: %d events to %s", len(events), savePath))
	return fmt.Sprintf("Exported %d events to %s", len(events), savePath), nil
}

// -- Metadata Operations --

// GetDistinctValues returns distinct values for a given field (for filter dropdowns).
func (a *App) GetDistinctValues(field string) (map[string]int64, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}
	return a.store.GetDistinctValues(field)
}

// GetMinMaxDate returns the date range of events in the database.
func (a *App) GetMinMaxDate() ([]string, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}
	min, max, err := a.store.GetMinMaxDate()
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
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}

	// Build WHERE clause from filters using dialect-aware placeholders and quoting
	d := a.queryDialect()
	var whereParts []string
	var whereArgs []interface{}
	paramIdx := 1

	// Always exclude junk dates (zeroed, pre-epoch, far-future)
	whereParts = append(whereParts, "datetime > '1970-01-01' AND datetime < '2100-01-01'")

	for _, f := range req.Filters {
		switch f.Operator {
		case "=", "!=", "LIKE", "NOT LIKE", ">=", "<=":
			// Normalize partial dates for datetime fields
			val := f.Value
			if f.Field == "datetime" {
				val = normalizeDate(val, f.Operator == "<=")
			}
			whereParts = append(whereParts, fmt.Sprintf("%s %s %s", d.QuoteColumn(f.Field), f.Operator, d.Placeholder(paramIdx)))
			paramIdx++
			whereArgs = append(whereArgs, val)
		}
	}

	// Full-text search for histogram
	if req.SearchText != "" {
		searchFields := []string{
			"desc", "filename", "source", "sourcetype", "type",
			"user", "host", "extra", "tag", "url", "source_name",
			"computer_name", "format", "notes",
		}
		var orParts []string
		for _, field := range searchFields {
			orParts = append(orParts, fmt.Sprintf("%s LIKE %s", d.QuoteColumn(field), d.Placeholder(paramIdx)))
			paramIdx++
			whereArgs = append(whereArgs, "%"+req.SearchText+"%")
		}
		whereParts = append(whereParts, "("+strings.Join(orParts, " OR ")+")")
	}

	// Bookmark filter for histogram
	if req.BookmarkOnly {
		whereParts = append(whereParts, "bookmark = 1")
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

	// Delegate to store for all database operations (date range, bucketing, histogram query)
	dbBuckets, err := a.store.GetTimelineHistogram(whereClause, whereArgs)
	if err != nil {
		return nil, err
	}

	// Convert from database.TimelineBucket to main.TimelineBucket
	buckets := make([]TimelineBucket, len(dbBuckets))
	for i, b := range dbBuckets {
		buckets[i] = TimelineBucket(b)
	}
	return buckets, nil
}

// GetTags returns all distinct tags.
func (a *App) GetTags() ([]string, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}
	return a.store.GetDistinctTags()
}

// -- Event Operations --

// UpdateEventFields updates specific fields on an event.
// Rejects negative IDs since examiner notes use a different update path.
func (a *App) UpdateEventFields(rowid int64, fields map[string]interface{}) error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	if rowid < 0 {
		return fmt.Errorf("cannot update examiner note fields via UpdateEventFields; use dedicated methods")
	}
	return a.store.UpdateEvent(rowid, fields)
}

// ToggleBookmark toggles the bookmark flag on an event or examiner note.
// Negative IDs are routed to the examiner notes table.
func (a *App) ToggleBookmark(rowid int64) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("no database open")
	}
	if rowid < 0 {
		return a.store.ToggleExaminerNoteBookmark(-rowid)
	}
	return a.store.ToggleBookmark(rowid)
}

// AddExaminerNote creates a new examiner note and returns the negated ID.
func (a *App) AddExaminerNote(datetime, description string) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("no database open")
	}
	id, err := a.store.InsertExaminerNote(datetime, description, "", "")
	if err != nil {
		return 0, err
	}
	a.logInfo(fmt.Sprintf("Examiner note added (ID %d)", id))
	return id, nil
}

// DeleteExaminerNote deletes an examiner note. The frontend passes the negated ID;
// this method converts it to the positive internal ID.
func (a *App) DeleteExaminerNote(negatedID int64) error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	if negatedID >= 0 {
		return fmt.Errorf("expected a negative examiner note ID, got %d", negatedID)
	}
	err := a.store.DeleteExaminerNote(-negatedID)
	if err != nil {
		return err
	}
	a.logInfo(fmt.Sprintf("Examiner note deleted (ID %d)", negatedID))
	return nil
}

// UpdateExaminerNoteColor updates the color of an examiner note.
// The frontend passes the negated ID.
func (a *App) UpdateExaminerNoteColor(negatedID int64, color string) error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	if negatedID >= 0 {
		return fmt.Errorf("expected a negative examiner note ID, got %d", negatedID)
	}
	return a.store.UpdateExaminerNoteColor(-negatedID, color)
}

// ToggleExaminerNoteBookmark toggles the bookmark on an examiner note.
// The frontend passes the negated ID.
func (a *App) ToggleExaminerNoteBookmark(negatedID int64) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("no database open")
	}
	if negatedID >= 0 {
		return 0, fmt.Errorf("expected a negative examiner note ID, got %d", negatedID)
	}
	return a.store.ToggleExaminerNoteBookmark(-negatedID)
}

// -- Saved Queries --

// GetSavedQueries returns all saved queries.
func (a *App) GetSavedQueries() ([]database.SavedQuery, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}
	return a.store.GetSavedQueries()
}

// SaveQuery stores a named query.
func (a *App) SaveQuery(name, queryStr string) error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	return a.store.SaveQuery(name, queryStr)
}

// DeleteSavedQuery removes a saved query.
func (a *App) DeleteSavedQuery(name string) error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	return a.store.DeleteQuery(name)
}

// -- PostgreSQL Connection --

// ConnectPostgres connects to an existing 4n6time PostgreSQL database.
func (a *App) ConnectPostgres(host, port, dbName, user, password, sslMode string) (*DBInfo, error) {
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if sslMode == "" {
		sslMode = "disable"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbName, sslMode)

	// Close any existing database
	if a.store != nil {
		a.store.Close()
		a.store = nil
	}

	store, err := database.OpenStore("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("connecting to PostgreSQL: %w", err)
	}

	a.store = store
	a.driver = "postgres"
	a.logInfo("Connected to PostgreSQL: " + maskConnStr(connStr))
	return a.getDBInfo()
}

// CreatePostgresDatabase creates the 4n6time schema on a PostgreSQL database and connects to it.
func (a *App) CreatePostgresDatabase(host, port, dbName, user, password, sslMode string) (*DBInfo, error) {
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if sslMode == "" {
		sslMode = "disable"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbName, sslMode)

	// Close any existing database
	if a.store != nil {
		a.store.Close()
		a.store = nil
	}

	store, err := database.CreateStore("postgres", connStr, nil)
	if err != nil {
		return nil, fmt.Errorf("creating PostgreSQL schema: %w", err)
	}

	a.store = store
	a.driver = "postgres"
	a.logInfo("Created PostgreSQL schema and connected: " + maskConnStr(connStr))
	return a.getDBInfo()
}

// PushToPostgres copies all events from the currently open SQLite database to a new PostgreSQL database.
// The SQLite database remains open after the push completes.
func (a *App) PushToPostgres(host, port, dbName, user, password, sslMode string) (string, error) {
	if a.store == nil || a.driver != "sqlite" {
		return "", fmt.Errorf("no SQLite database is open")
	}

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if sslMode == "" {
		sslMode = "disable"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbName, sslMode)

	pushStart := time.Now()
	a.logInfo("Push to PostgreSQL started: " + maskConnStr(connStr))

	// Verify SQLite has data before creating the PostgreSQL schema
	sourceCount, err := a.store.CountEvents("", nil)
	if err != nil {
		return "", fmt.Errorf("counting SQLite events: %w", err)
	}
	if sourceCount == 0 {
		return "", fmt.Errorf("SQLite database has no events to push")
	}

	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "reading", "message": fmt.Sprintf("SQLite database has %d events, connecting to PostgreSQL...", sourceCount), "count": 0, "total": 0,
	})

	// Create PostgreSQL store with schema
	pgStore, err := database.CreateStore("postgres", connStr, nil)
	if err != nil {
		return "", fmt.Errorf("creating PostgreSQL schema: %w", err)
	}
	defer pgStore.Close()

	// Read all events from SQLite using the same pattern as ExportCSV
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "reading", "message": fmt.Sprintf("Reading %d events from SQLite...", sourceCount), "count": 0, "total": sourceCount,
	})

	q := query.New(999999999) // effectively unlimited, matches ExportCSV pattern
	q.SetDialect(query.DefaultDialect)
	q.OrderBy("datetime")
	q.SetPage(1)
	sqlStr, args := q.Build()

	events, err := a.store.ExecuteQuery(sqlStr, args)
	if err != nil {
		return "", fmt.Errorf("reading events from SQLite: %w", err)
	}

	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "reading", "message": fmt.Sprintf("Read %d events from SQLite", len(events)), "count": len(events), "total": sourceCount,
	})

	if len(events) == 0 {
		return "", fmt.Errorf("query returned 0 events from SQLite (expected %d); the database may be empty or the query failed", sourceCount)
	}

	// Insert into PostgreSQL
	total := len(events)
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "inserting", "message": fmt.Sprintf("Inserting %d events into PostgreSQL...", total), "count": 0, "total": total,
	})

	inserted, err := pgStore.InsertEvents(events, func(count int) {
		runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
			"phase": "inserting", "message": fmt.Sprintf("Inserted %d of %d events into PostgreSQL...", count, total), "count": count, "total": total,
		})
	})
	if err != nil {
		return "", fmt.Errorf("inserting events into PostgreSQL (inserted %d of %d before failure): %w", inserted, total, err)
	}

	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "inserting", "message": fmt.Sprintf("Successfully inserted %d events into PostgreSQL", inserted), "count": inserted, "total": total,
	})

	// Copy examiner notes
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "inserting", "message": "Copying examiner notes...", "count": inserted, "total": total,
	})
	examNotes, err := a.store.GetExaminerNotes()
	if err != nil {
		// Not fatal; log and continue
		a.logError("Failed to read examiner notes for push: " + err.Error())
	}
	notesInserted := 0
	for _, note := range examNotes {
		_, err := pgStore.InsertExaminerNote(note.Datetime, note.Desc, note.Tag, note.Color)
		if err != nil {
			a.logError("Failed to push examiner note: " + err.Error())
			continue
		}
		notesInserted++
	}
	if notesInserted > 0 {
		a.logInfo(fmt.Sprintf("Pushed %d examiner notes to PostgreSQL", notesInserted))
	}

	// Update PostgreSQL metadata
	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "metadata", "message": "Building PostgreSQL metadata and indexes...", "count": 0, "total": 0,
	})
	if err := pgStore.UpdateMetadata(); err != nil {
		return "", fmt.Errorf("updating PostgreSQL metadata: %w", err)
	}
	a.logInfo("Metadata update complete (PostgreSQL)")

	runtime.EventsEmit(a.ctx, "import:progress", map[string]interface{}{
		"phase": "done", "message": fmt.Sprintf("Push complete: %d events transferred to PostgreSQL", inserted), "count": inserted, "total": total,
	})
	a.logInfo(fmt.Sprintf("Push complete: %d events + %d notes in %s", inserted, notesInserted, time.Since(pushStart).Round(time.Millisecond)))

	msg := fmt.Sprintf("Pushed %d events to PostgreSQL", inserted)
	if notesInserted > 0 {
		msg += fmt.Sprintf(" (%d examiner notes)", notesInserted)
	}
	return msg, nil
}

// -- Internal Helpers --

// GetVersion returns the application version string.
func (a *App) GetVersion() string {
	return Version
}

// queryDialect returns the appropriate QueryDialect for the current database driver.
func (a *App) queryDialect() query.QueryDialect {
	if a.driver == "postgres" {
		return &database.PostgresDialect{}
	}
	return query.DefaultDialect
}

// normalizeDate expands partial date strings to full timestamps suitable for
// SQL queries on both SQLite and PostgreSQL. When isEnd is false, the date is
// expanded to the start of the period; when true, to the end of the period.
// Full timestamps (containing a space, i.e. "YYYY-MM-DD HH:MM:SS") pass through unchanged.
func normalizeDate(value string, isEnd bool) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}

	// Already a full timestamp (contains date and time separated by space)
	if strings.Contains(value, " ") {
		return value
	}

	// Year only: "2025"
	if matched, _ := regexp.MatchString(`^\d{4}$`, value); matched {
		if isEnd {
			return value + "-12-31 23:59:59"
		}
		return value + "-01-01 00:00:00"
	}

	// Year-month: "2025-02"
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}$`, value); matched {
		if isEnd {
			// Parse year and month to find last day
			t, err := time.Parse("2006-01", value)
			if err != nil {
				return value
			}
			// Go to first of next month, subtract one day
			lastDay := t.AddDate(0, 1, -1)
			return lastDay.Format("2006-01-02") + " 23:59:59"
		}
		return value + "-01 00:00:00"
	}

	// Date only: "2025-02-15"
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, value); matched {
		if isEnd {
			return value + " 23:59:59"
		}
		return value + " 00:00:00"
	}

	// Unrecognized format, return as-is
	return value
}

// DBInfo contains summary info about the loaded database.
type DBInfo struct {
	Path       string `json:"path"`
	Driver     string `json:"driver"`
	EventCount int64  `json:"eventCount"`
	MinDate    string `json:"minDate"`
	MaxDate    string `json:"maxDate"`
}

func (a *App) loadDatabase(path string) (*DBInfo, error) {
	// Close any existing database
	if a.store != nil {
		a.store.Close()
		a.store = nil
	}

	store, err := database.OpenStore("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	a.store = store
	a.driver = "sqlite"
	a.logInfo("Database opened: " + path)
	return a.getDBInfo()
}

func (a *App) getDBInfo() (*DBInfo, error) {
	count, err := a.store.CountEvents("", nil)
	if err != nil {
		return nil, err
	}

	min, max, err := a.store.GetMinMaxDate()
	if err != nil {
		// Not fatal, just means empty db
		min = ""
		max = ""
	}

	return &DBInfo{
		Path:       a.store.Path(),
		Driver:     a.driver,
		EventCount: count,
		MinDate:    min,
		MaxDate:    max,
	}, nil
}

// -- Logging Controls --

// EnableLogging opens a save file dialog for the user to choose a log file location,
// then begins writing timestamped log entries to that file.
func (a *App) EnableLogging() (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Log File",
		DefaultFilename: "4n6time.log",
		Filters: []runtime.FileFilter{
			{DisplayName: "Log Files (*.log)", Pattern: "*.log"},
			{DisplayName: "Text Files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // user cancelled
	}

	a.logMu.Lock()
	if a.logFile != nil {
		a.logFile.Close()
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		a.logMu.Unlock()
		return "", fmt.Errorf("opening log file: %w", err)
	}
	a.logFile = f
	a.logEnabled = true
	a.logPath = path
	a.logMu.Unlock()

	a.saveLoggingConfig()
	a.logInfo("Logging enabled, writing to " + path)
	return path, nil
}

// DisableLogging closes the log file and stops logging.
func (a *App) DisableLogging() error {
	a.logInfo("Logging disabled")

	a.logMu.Lock()
	if a.logFile != nil {
		a.logFile.Close()
		a.logFile = nil
	}
	a.logEnabled = false
	a.logMu.Unlock()

	a.saveLoggingConfig()
	return nil
}

// GetLoggingStatus returns whether logging is enabled and the current log file path.
func (a *App) GetLoggingStatus() LoggingStatus {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	return LoggingStatus{
		Enabled:  a.logEnabled,
		FilePath: a.logPath,
		Persist:  a.logPersist,
	}
}

// SetLoggingPersist controls whether logging settings persist between sessions.
func (a *App) SetLoggingPersist(persist bool) {
	a.logMu.Lock()
	a.logPersist = persist
	a.logMu.Unlock()
	a.saveLoggingConfig()
	if persist {
		a.logInfo("Logging persistence enabled")
	} else {
		a.logInfo("Logging persistence disabled")
	}
}
