package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/cdtdelta/4n6time/internal/model"

	_ "modernc.org/sqlite"
)

// Default fields to index when creating a new database.
var DefaultIndexFields = []string{"host", "user", "source", "sourcetype", "type", "datetime", "color"}

// Metadata table names that track distinct values and their frequencies.
// These map to l2t_<name>s tables in the database (e.g. l2t_sources, l2t_sourcetypes).
var metadataFields = []string{"sourcetype", "source", "user", "host", "MACB", "color", "type", "record_number"}

// SQLiteStore manages all SQLite operations for a 4n6time database.
// It implements the Store interface.
type SQLiteStore struct {
	path    string
	conn    *sql.DB
	dialect Dialect
}

// OpenSQLite opens an existing 4n6time SQLite database.
func OpenSQLite(path string) (*SQLiteStore, error) {
	d := &SQLiteDialect{}

	conn, err := sql.Open(d.DriverName(), d.DSN(path))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Verify the connection works
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	db := &SQLiteStore{path: path, conn: conn, dialect: d}

	// Migrate: add bookmark column if it doesn't exist (for pre-0.8.0 databases)
	db.migrate()

	return db, nil
}

// migrate applies schema migrations for backward compatibility.
func (db *SQLiteStore) migrate() {
	// Add bookmark column if missing
	var count int
	err := db.conn.QueryRow(
		db.dialect.SchemaCheckColumnSQL("log2timeline", "bookmark"),
	).Scan(&count)
	if err == nil && count == 0 {
		db.conn.Exec("ALTER TABLE log2timeline ADD COLUMN bookmark INT DEFAULT 0")
	}
}

// ToggleBookmark toggles the bookmark flag on an event and returns the new value.
func (db *SQLiteStore) ToggleBookmark(rowid int64) (int64, error) {
	idCol := db.dialect.IDColumn()
	_, err := db.conn.Exec(
		"UPDATE log2timeline SET bookmark = CASE WHEN bookmark = 1 THEN 0 ELSE 1 END WHERE "+idCol+" = "+db.dialect.Placeholder(1),
		rowid,
	)
	if err != nil {
		return 0, err
	}

	var val int64
	err = db.conn.QueryRow(
		"SELECT bookmark FROM log2timeline WHERE "+idCol+" = "+db.dialect.Placeholder(1),
		rowid,
	).Scan(&val)
	return val, err
}

// CreateSQLite creates a new 4n6time SQLite database with the full schema.
// indexFields specifies which columns to index. Pass nil to use DefaultIndexFields.
func CreateSQLite(path string, indexFields []string) (*SQLiteStore, error) {
	d := &SQLiteDialect{}

	conn, err := sql.Open(d.DriverName(), d.DSN(path))
	if err != nil {
		return nil, fmt.Errorf("creating database: %w", err)
	}

	db := &SQLiteStore{path: path, conn: conn, dialect: d}

	if err := db.createSchema(indexFields); err != nil {
		conn.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *SQLiteStore) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Path returns the file path of the database.
func (db *SQLiteStore) Path() string {
	return db.path
}

// Conn returns the underlying *sql.DB connection for advanced query usage.
func (db *SQLiteStore) Conn() *sql.DB {
	return db.conn
}

// createSchema builds all tables and indexes for a new database.
func (db *SQLiteStore) createSchema(indexFields []string) error {
	if indexFields == nil {
		indexFields = DefaultIndexFields
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Main event table
	_, err = tx.Exec(db.dialect.CreateTableSQL())
	if err != nil {
		return fmt.Errorf("creating log2timeline table: %w", err)
	}

	// Metadata tables for filter dropdowns (distinct values + frequency)
	for _, f := range metadataFields {
		tableName := "l2t_" + f + "s"
		_, err = tx.Exec(db.dialect.CreateMetadataTableSQL(tableName, f))
		if err != nil {
			return fmt.Errorf("creating metadata table %s: %w", tableName, err)
		}
	}

	// Tags table
	_, err = tx.Exec(db.dialect.CreateTagsTableSQL())
	if err != nil {
		return fmt.Errorf("creating l2t_tags table: %w", err)
	}

	// Saved queries table
	_, err = tx.Exec(db.dialect.CreateSavedQueryTableSQL())
	if err != nil {
		return fmt.Errorf("creating l2t_saved_query table: %w", err)
	}

	// Disk image config table
	_, err = tx.Exec(db.dialect.CreateDiskTableSQL())
	if err != nil {
		return fmt.Errorf("creating l2t_disk table: %w", err)
	}

	// Insert default disk config row
	_, err = tx.Exec(db.dialect.InsertDefaultDiskSQL())
	if err != nil {
		return fmt.Errorf("inserting default disk config: %w", err)
	}

	// Create indexes
	for _, field := range indexFields {
		_, err = tx.Exec(db.dialect.CreateIndexSQL(field+"_idx", "log2timeline", field))
		if err != nil {
			return fmt.Errorf("creating index on %s: %w", field, err)
		}
	}

	return tx.Commit()
}

// InsertEvent inserts a single event into the database.
func (db *SQLiteStore) InsertEvent(e *model.Event) error {
	_, err := db.conn.Exec(db.dialect.InsertEventSQL(),
		e.Timezone, e.MACB, e.Source, e.SourceType, e.Type,
		e.User, e.Host, e.Desc, e.Filename, e.Inode,
		e.Notes, e.Format, e.Extra, e.Datetime, e.ReportNotes,
		e.InReport, e.Tag, e.Color, e.Offset, e.StoreNumber,
		e.StoreIndex, e.VSSStoreNumber, e.URL, e.RecordNumber,
		e.EventID, e.EventType, e.SourceName, e.UserSID, e.ComputerName,
		e.Bookmark,
	)
	return err
}

// InsertEvents inserts a batch of events inside a single transaction.
// The onProgress callback is called every 10,000 events with the current count.
// Pass nil for onProgress if you don't need progress updates.
func (db *SQLiteStore) InsertEvents(events []*model.Event, onProgress func(count int)) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(db.dialect.InsertEventSQL())
	if err != nil {
		return 0, fmt.Errorf("preparing insert statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, e := range events {
		_, err := stmt.Exec(
			e.Timezone, e.MACB, e.Source, e.SourceType, e.Type,
			e.User, e.Host, e.Desc, e.Filename, e.Inode,
			e.Notes, e.Format, e.Extra, e.Datetime, e.ReportNotes,
			e.InReport, e.Tag, e.Color, e.Offset, e.StoreNumber,
			e.StoreIndex, e.VSSStoreNumber, e.URL, e.RecordNumber,
			e.EventID, e.EventType, e.SourceName, e.UserSID, e.ComputerName,
			e.Bookmark,
		)
		if err != nil {
			return inserted, fmt.Errorf("inserting event %d: %w", inserted+1, err)
		}
		inserted++
		if onProgress != nil && inserted%10000 == 0 {
			onProgress(inserted)
		}
	}

	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("committing transaction: %w", err)
	}

	return inserted, nil
}

// QueryEvents runs a SQL query and returns the matching events.
// The query should be a full SELECT statement or a WHERE clause.
// If whereClause is provided, it's wrapped in a full SELECT from log2timeline.
func (db *SQLiteStore) QueryEvents(whereClause string, args []interface{}, orderBy string, limit, offset int) ([]*model.Event, error) {
	idCol := db.dialect.IDColumn()
	query := "SELECT " + idCol + ", timezone, MACB, source, sourcetype, type, user, host, " +
		"desc, filename, inode, notes, format, extra, datetime, reportnotes, " +
		"inreport, tag, color, offset, store_number, store_index, vss_store_number, " +
		"URL, record_number, event_identifier, event_type, source_name, user_sid, " +
		"computer_name, bookmark FROM log2timeline"

	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
		if offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", offset)
		}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// CountEvents returns the total number of events, optionally filtered by a WHERE clause.
func (db *SQLiteStore) CountEvents(whereClause string, args []interface{}) (int64, error) {
	idCol := db.dialect.IDColumn()
	query := "SELECT COUNT(" + idCol + ") FROM log2timeline"
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	var count int64
	err := db.conn.QueryRow(query, args...).Scan(&count)
	return count, err
}

// GetMinMaxDate returns the earliest and latest datetime values in the database,
// excluding the sentinel value '0000-00-00 00:00:00'.
func (db *SQLiteStore) GetMinMaxDate() (minDate, maxDate string, err error) {
	err = db.conn.QueryRow(
		"SELECT COALESCE(min(datetime), ''), COALESCE(max(datetime), '') FROM log2timeline WHERE datetime > '1970-01-01' AND datetime < '2100-01-01'",
	).Scan(&minDate, &maxDate)
	return
}

// GetDistinctValues returns a map of distinct values and their counts for a given column.
func (db *SQLiteStore) GetDistinctValues(fieldName string) (map[string]int64, error) {
	// Validate field name against known fields to prevent injection
	if !isValidField(fieldName) {
		return nil, fmt.Errorf("invalid field name: %s", fieldName)
	}

	query := fmt.Sprintf(
		"SELECT %s, COUNT(%s) FROM log2timeline GROUP BY %s", fieldName, fieldName, fieldName)

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var value string
		var count int64
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		if value != "" {
			result[value] = count
		}
	}
	return result, rows.Err()
}

// GetDistinctTags returns all unique tags from the events table.
// Tags can be comma-separated within a single field, so this splits them.
func (db *SQLiteStore) GetDistinctTags() ([]string, error) {
	rows, err := db.conn.Query("SELECT DISTINCT tag FROM log2timeline")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var tags []string

	for rows.Next() {
		var tagStr string
		if err := rows.Scan(&tagStr); err != nil {
			return nil, err
		}
		if tagStr == "" {
			continue
		}
		for _, t := range strings.Split(tagStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" && !seen[t] {
				seen[t] = true
				tags = append(tags, t)
			}
		}
	}
	return tags, rows.Err()
}

// UpdateEvent updates specific fields of an event identified by rowid.
func (db *SQLiteStore) UpdateEvent(rowid int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	// Validate all field names
	setClauses := make([]string, 0, len(fields))
	args := make([]interface{}, 0, len(fields)+1)

	paramIdx := 1
	for field, value := range fields {
		if !isValidField(field) {
			return fmt.Errorf("invalid field name: %s", field)
		}
		setClauses = append(setClauses, field+" = "+db.dialect.Placeholder(paramIdx))
		paramIdx++
		args = append(args, value)
	}
	args = append(args, rowid)

	idCol := db.dialect.IDColumn()
	query := fmt.Sprintf("UPDATE log2timeline SET %s WHERE %s = %s",
		strings.Join(setClauses, ", "), idCol, db.dialect.Placeholder(paramIdx))

	_, err := db.conn.Exec(query, args...)
	return err
}

// UpdateMetadata refreshes all metadata tables (l2t_sources, l2t_hosts, etc.)
// with current distinct values from the main table.
func (db *SQLiteStore) UpdateMetadata() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, f := range metadataFields {
		tableName := "l2t_" + f + "s"

		// Clear existing metadata
		_, err = tx.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
		if err != nil {
			return fmt.Errorf("clearing %s: %w", tableName, err)
		}

		// Repopulate with current values
		_, err = tx.Exec(fmt.Sprintf(
			"INSERT INTO %s (%s, frequency) SELECT %s, COUNT(%s) FROM log2timeline WHERE %s <> '' GROUP BY %s",
			tableName, f, f, f, f, f))
		if err != nil {
			return fmt.Errorf("populating %s: %w", tableName, err)
		}
	}

	// Update tags table
	_, err = tx.Exec("DELETE FROM l2t_tags")
	if err != nil {
		return fmt.Errorf("clearing l2t_tags: %w", err)
	}

	// Get distinct tags (need to split comma-separated values in Go)
	rows, err := tx.Query("SELECT DISTINCT tag FROM log2timeline WHERE tag <> ''")
	if err != nil {
		return fmt.Errorf("querying tags: %w", err)
	}

	seen := make(map[string]bool)
	tagStmt, err := tx.Prepare("INSERT INTO l2t_tags (tag) VALUES (" + db.dialect.Placeholder(1) + ")")
	if err != nil {
		rows.Close()
		return fmt.Errorf("preparing tag insert: %w", err)
	}

	for rows.Next() {
		var tagStr string
		if err := rows.Scan(&tagStr); err != nil {
			rows.Close()
			tagStmt.Close()
			return err
		}
		for _, t := range strings.Split(tagStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" && !seen[t] {
				seen[t] = true
				_, err = tagStmt.Exec(t)
				if err != nil {
					rows.Close()
					tagStmt.Close()
					return fmt.Errorf("inserting tag: %w", err)
				}
			}
		}
	}
	rows.Close()
	tagStmt.Close()

	if err := rows.Err(); err != nil {
		return err
	}

	return tx.Commit()
}

// RebuildIndexes drops all existing indexes and creates new ones for the given fields.
func (db *SQLiteStore) RebuildIndexes(indexFields []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Drop all existing indexes
	for _, f := range model.Fields {
		_, err = tx.Exec(db.dialect.DropIndexSQL(f + "_idx"))
		if err != nil {
			return fmt.Errorf("dropping index %s_idx: %w", f, err)
		}
	}

	// Create new indexes
	for _, f := range indexFields {
		_, err = tx.Exec(db.dialect.CreateIndexSQL(f+"_idx", "log2timeline", f))
		if err != nil {
			return fmt.Errorf("creating index %s_idx: %w", f, err)
		}
	}

	return tx.Commit()
}

// SavedQuery represents a named query stored in the database.
type SavedQuery struct {
	Name  string
	Query string
}

// GetSavedQueries returns all saved queries from the database.
func (db *SQLiteStore) GetSavedQueries() ([]SavedQuery, error) {
	rows, err := db.conn.Query("SELECT name, query FROM l2t_saved_query")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queries []SavedQuery
	for rows.Next() {
		var q SavedQuery
		if err := rows.Scan(&q.Name, &q.Query); err != nil {
			return nil, err
		}
		queries = append(queries, q)
	}
	return queries, rows.Err()
}

// SaveQuery stores a named query in the database.
func (db *SQLiteStore) SaveQuery(name, query string) error {
	_, err := db.conn.Exec(
		"INSERT INTO l2t_saved_query (name, query) VALUES ("+db.dialect.Placeholder(1)+", "+db.dialect.Placeholder(2)+")",
		name, query,
	)
	return err
}

// DeleteQuery removes a saved query by name.
func (db *SQLiteStore) DeleteQuery(name string) error {
	_, err := db.conn.Exec(
		"DELETE FROM l2t_saved_query WHERE name = "+db.dialect.Placeholder(1),
		name,
	)
	return err
}

// Migrate applies any pending schema migrations.
func (db *SQLiteStore) Migrate() error {
	db.migrate()
	return nil
}

// ExecuteQuery runs a pre-built SQL SELECT and scans results using model.Fields
// column order. This is used for queries built by query.go's Build() method,
// where the SELECT list is: rowid, datetime, timezone, MACB, source, ...
// (Pattern B scan order, distinct from QueryEvents' Pattern A order.)
func (db *SQLiteStore) ExecuteQuery(sqlStr string, args []interface{}) ([]*model.Event, error) {
	rows, err := db.conn.Query(sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()
	return scanFieldsOrderEvents(rows)
}

// ExecuteCountQuery runs a pre-built COUNT query and returns the result.
func (db *SQLiteStore) ExecuteCountQuery(sqlStr string, args []interface{}) (int64, error) {
	var count int64
	err := db.conn.QueryRow(sqlStr, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("executing count query: %w", err)
	}
	return count, nil
}

// GetTimelineHistogram returns event counts bucketed by time interval.
// whereClause should include the "WHERE" keyword if non-empty (e.g. "WHERE datetime > '1970-01-01'").
// whereArgs are the parameter values for any placeholders in the clause.
// The bucket size is automatically chosen based on the filtered date range.
func (db *SQLiteStore) GetTimelineHistogram(whereClause string, whereArgs []interface{}) ([]TimelineBucket, error) {
	// Get date range to determine bucket size
	rangeSQL := "SELECT MIN(datetime), MAX(datetime) FROM log2timeline"
	if whereClause != "" {
		rangeSQL += " " + whereClause
	}

	var minDate, maxDate string
	if err := db.conn.QueryRow(rangeSQL, whereArgs...).Scan(&minDate, &maxDate); err != nil {
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

	// Build and run histogram query using dialect-specific date formatting
	bucketExpr := db.dialect.DateFormatSQL("datetime", bucketFormat)
	histSQL := "SELECT " + bucketExpr + " as bucket, COUNT(*) as cnt FROM log2timeline"
	if whereClause != "" {
		histSQL += " " + whereClause
	}
	histSQL += " GROUP BY bucket ORDER BY bucket"

	rows, err := db.conn.Query(histSQL, whereArgs...)
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

// scanFieldsOrderEvents scans rows using model.Fields column order.
// This matches the SELECT generated by query.go's Build() method:
//
//	rowid, datetime, timezone, MACB, source, sourcetype, type, user, host, desc,
//	filename, inode, notes, format, extra, reportnotes, inreport, tag, color,
//	offset, store_number, store_index, vss_store_number, URL, record_number,
//	event_identifier, event_type, source_name, user_sid, computer_name, bookmark
//
// Note: datetime is at position 2 (right after rowid), NOT at position 15.
// This is Pattern B, distinct from scanEvents which uses Pattern A.
func scanFieldsOrderEvents(rows *sql.Rows) ([]*model.Event, error) {
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
			&e.Bookmark,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// scanEvents converts sql.Rows into a slice of Event pointers.
func scanEvents(rows *sql.Rows) ([]*model.Event, error) {
	var events []*model.Event
	for rows.Next() {
		e := &model.Event{}
		err := rows.Scan(
			&e.ID, &e.Timezone, &e.MACB, &e.Source, &e.SourceType,
			&e.Type, &e.User, &e.Host, &e.Desc, &e.Filename,
			&e.Inode, &e.Notes, &e.Format, &e.Extra, &e.Datetime,
			&e.ReportNotes, &e.InReport, &e.Tag, &e.Color, &e.Offset,
			&e.StoreNumber, &e.StoreIndex, &e.VSSStoreNumber, &e.URL,
			&e.RecordNumber, &e.EventID, &e.EventType, &e.SourceName,
			&e.UserSID, &e.ComputerName, &e.Bookmark,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// isValidField checks that a field name is one of the known log2timeline columns.
// This prevents SQL injection when field names are interpolated into queries.
func isValidField(name string) bool {
	for _, f := range model.Fields {
		if f == name {
			return true
		}
	}
	return false
}
