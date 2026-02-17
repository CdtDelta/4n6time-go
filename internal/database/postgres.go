package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/cdtdelta/4n6time/internal/model"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// validTimestampRe matches datetime strings in YYYY-MM-DD HH:MM:SS or
// YYYY-MM-DDTHH:MM:SS format (with optional fractional seconds/timezone suffix).
// Anything that doesn't match this pattern cannot be a valid PostgreSQL TIMESTAMP.
var validTimestampRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}`)

// pgSanitizeString strips null bytes (0x00) from a string. SQLite stores these
// fine but PostgreSQL rejects them with "invalid byte sequence for encoding UTF8".
func pgSanitizeString(s string) string {
	if strings.ContainsRune(s, '\x00') {
		return strings.ReplaceAll(s, "\x00", "")
	}
	return s
}

// pgSanitizeDatetime returns the datetime string if it is a valid timestamp
// for PostgreSQL, or nil (SQL NULL) if it is empty, a zero sentinel, or
// otherwise unparseable. SQLite stores datetime as TEXT and can contain values
// like "", "0000-00-00 00:00:00", or "Not a time" that PostgreSQL rejects.
func pgSanitizeDatetime(s string) interface{} {
	s = pgSanitizeString(s)
	if s == "" {
		return nil
	}
	if !validTimestampRe.MatchString(s) {
		return nil
	}
	// Year 0000 is out of range for PostgreSQL TIMESTAMP
	if s[:4] == "0000" {
		return nil
	}
	return s
}

// PostgresStore manages all PostgreSQL operations for a 4n6time database.
// It implements the Store interface.
type PostgresStore struct {
	connStr string
	conn    *sql.DB
	dialect Dialect
}

// OpenPostgres opens an existing 4n6time PostgreSQL database.
func OpenPostgres(connStr string) (*PostgresStore, error) {
	d := &PostgresDialect{}

	conn, err := sql.Open(d.DriverName(), d.DSN(connStr))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	db := &PostgresStore{connStr: connStr, conn: conn, dialect: d}

	// Migrate: add bookmark column if it doesn't exist (for pre-0.8.0 databases)
	db.migrate()

	return db, nil
}

// CreatePostgres creates a new 4n6time schema on a PostgreSQL database.
// The database itself must already exist; this creates the tables and indexes.
func CreatePostgres(connStr string, indexFields []string) (*PostgresStore, error) {
	d := &PostgresDialect{}

	conn, err := sql.Open(d.DriverName(), d.DSN(connStr))
	if err != nil {
		return nil, fmt.Errorf("creating database: %w", err)
	}

	db := &PostgresStore{connStr: connStr, conn: conn, dialect: d}

	if err := db.createSchema(indexFields); err != nil {
		conn.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *PostgresStore) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Path returns the connection string used to connect to the database.
func (db *PostgresStore) Path() string {
	return db.connStr
}

// Conn returns the underlying *sql.DB connection for advanced query usage.
func (db *PostgresStore) Conn() *sql.DB {
	return db.conn
}

// migrate applies schema migrations for backward compatibility.
func (db *PostgresStore) migrate() {
	// Add bookmark column if missing
	var count int
	err := db.conn.QueryRow(
		db.dialect.SchemaCheckColumnSQL("log2timeline", "bookmark"),
	).Scan(&count)
	if err == nil && count == 0 {
		db.conn.Exec("ALTER TABLE log2timeline ADD COLUMN bookmark INT DEFAULT 0")
	}
}

// Migrate applies any pending schema migrations.
func (db *PostgresStore) Migrate() error {
	db.migrate()
	return nil
}

// ToggleBookmark toggles the bookmark flag on an event and returns the new value.
func (db *PostgresStore) ToggleBookmark(rowid int64) (int64, error) {
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

// createSchema builds all tables and indexes for a new database.
func (db *PostgresStore) createSchema(indexFields []string) error {
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
func (db *PostgresStore) InsertEvent(e *model.Event) error {
	_, err := db.conn.Exec(db.dialect.InsertEventSQL(),
		pgSanitizeString(e.Timezone), pgSanitizeString(e.MACB),
		pgSanitizeString(e.Source), pgSanitizeString(e.SourceType), pgSanitizeString(e.Type),
		pgSanitizeString(e.User), pgSanitizeString(e.Host), pgSanitizeString(e.Desc),
		pgSanitizeString(e.Filename), pgSanitizeString(e.Inode),
		pgSanitizeString(e.Notes), pgSanitizeString(e.Format), pgSanitizeString(e.Extra),
		pgSanitizeDatetime(e.Datetime), pgSanitizeString(e.ReportNotes),
		pgSanitizeString(e.InReport), pgSanitizeString(e.Tag), pgSanitizeString(e.Color),
		e.Offset, e.StoreNumber,
		e.StoreIndex, e.VSSStoreNumber, pgSanitizeString(e.URL),
		pgSanitizeString(e.RecordNumber),
		pgSanitizeString(e.EventID), pgSanitizeString(e.EventType),
		pgSanitizeString(e.SourceName), pgSanitizeString(e.UserSID),
		pgSanitizeString(e.ComputerName),
		e.Bookmark,
	)
	return err
}

// InsertEvents inserts a batch of events inside a single transaction.
// The onProgress callback is called every 10,000 events with the current count.
// Pass nil for onProgress if you don't need progress updates.
func (db *PostgresStore) InsertEvents(events []*model.Event, onProgress func(count int)) (int, error) {
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
			pgSanitizeString(e.Timezone), pgSanitizeString(e.MACB),
			pgSanitizeString(e.Source), pgSanitizeString(e.SourceType), pgSanitizeString(e.Type),
			pgSanitizeString(e.User), pgSanitizeString(e.Host), pgSanitizeString(e.Desc),
			pgSanitizeString(e.Filename), pgSanitizeString(e.Inode),
			pgSanitizeString(e.Notes), pgSanitizeString(e.Format), pgSanitizeString(e.Extra),
			pgSanitizeDatetime(e.Datetime), pgSanitizeString(e.ReportNotes),
			pgSanitizeString(e.InReport), pgSanitizeString(e.Tag), pgSanitizeString(e.Color),
			e.Offset, e.StoreNumber,
			e.StoreIndex, e.VSSStoreNumber, pgSanitizeString(e.URL),
			pgSanitizeString(e.RecordNumber),
			pgSanitizeString(e.EventID), pgSanitizeString(e.EventType),
			pgSanitizeString(e.SourceName), pgSanitizeString(e.UserSID),
			pgSanitizeString(e.ComputerName),
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
// Uses Pattern A column order with PostgreSQL reserved words quoted.
func (db *PostgresStore) QueryEvents(whereClause string, args []interface{}, orderBy string, limit, offset int) ([]*model.Event, error) {
	idCol := db.dialect.IDColumn()
	query := "SELECT " + idCol + `, timezone, MACB, source, sourcetype, type, "user", host, ` +
		`"desc", filename, inode, notes, format, extra, datetime, reportnotes, ` +
		`inreport, tag, color, "offset", store_number, store_index, vss_store_number, ` +
		`URL, record_number, event_identifier, event_type, source_name, user_sid, ` +
		`computer_name, bookmark FROM log2timeline`

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

	return pgScanEvents(rows)
}

// CountEvents returns the total number of events, optionally filtered by a WHERE clause.
func (db *PostgresStore) CountEvents(whereClause string, args []interface{}) (int64, error) {
	idCol := db.dialect.IDColumn()
	query := "SELECT COUNT(" + idCol + ") FROM log2timeline"
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	var count int64
	err := db.conn.QueryRow(query, args...).Scan(&count)
	return count, err
}

// GetMinMaxDate returns the earliest and latest datetime values in the database.
// Uses to_char to convert PostgreSQL TIMESTAMP to a consistent text format
// that matches the string slicing used by GetTimelineHistogram.
func (db *PostgresStore) GetMinMaxDate() (minDate, maxDate string, err error) {
	err = db.conn.QueryRow(
		`SELECT COALESCE(to_char(min(datetime), 'YYYY-MM-DD HH24:MI:SS'), ''),
		        COALESCE(to_char(max(datetime), 'YYYY-MM-DD HH24:MI:SS'), '')
		 FROM log2timeline WHERE datetime > '1970-01-01' AND datetime < '2100-01-01'`,
	).Scan(&minDate, &maxDate)
	return
}

// GetDistinctValues returns a map of distinct values and their counts for a given column.
// Uses pgQuoteCol to handle PostgreSQL reserved word columns.
func (db *PostgresStore) GetDistinctValues(fieldName string) (map[string]int64, error) {
	if !isValidField(fieldName) {
		return nil, fmt.Errorf("invalid field name: %s", fieldName)
	}

	col := pgQuoteCol(fieldName)
	query := fmt.Sprintf(
		"SELECT %s, COUNT(%s) FROM log2timeline GROUP BY %s", col, col, col)

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
func (db *PostgresStore) GetDistinctTags() ([]string, error) {
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

// UpdateEvent updates specific fields of an event identified by its id.
// Uses pgQuoteCol to handle PostgreSQL reserved word columns in SET clauses.
func (db *PostgresStore) UpdateEvent(rowid int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	setClauses := make([]string, 0, len(fields))
	args := make([]interface{}, 0, len(fields)+1)

	paramIdx := 1
	for field, value := range fields {
		if !isValidField(field) {
			return fmt.Errorf("invalid field name: %s", field)
		}
		setClauses = append(setClauses, pgQuoteCol(field)+" = "+db.dialect.Placeholder(paramIdx))
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

// UpdateMetadata refreshes all metadata tables with current distinct values.
// Uses pgQuoteCol for column references that may be PostgreSQL reserved words.
func (db *PostgresStore) UpdateMetadata() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, f := range metadataFields {
		tableName := "l2t_" + f + "s"
		col := pgQuoteCol(f)

		// Clear existing metadata
		_, err = tx.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
		if err != nil {
			return fmt.Errorf("clearing %s: %w", tableName, err)
		}

		// Repopulate with current values
		_, err = tx.Exec(fmt.Sprintf(
			"INSERT INTO %s (%s, frequency) SELECT %s, COUNT(%s) FROM log2timeline WHERE %s <> '' GROUP BY %s",
			tableName, col, col, col, col, col))
		if err != nil {
			return fmt.Errorf("populating %s: %w", tableName, err)
		}
	}

	// Update tags table
	_, err = tx.Exec("DELETE FROM l2t_tags")
	if err != nil {
		return fmt.Errorf("clearing l2t_tags: %w", err)
	}

	// Get distinct tags (need to split comma-separated values in Go).
	// Collect all results first and close the cursor before doing inserts,
	// because PostgreSQL does not allow concurrent operations on a single connection.
	rows, err := tx.Query("SELECT DISTINCT tag FROM log2timeline WHERE tag <> ''")
	if err != nil {
		return fmt.Errorf("querying tags: %w", err)
	}

	var rawTags []string
	for rows.Next() {
		var tagStr string
		if err := rows.Scan(&tagStr); err != nil {
			rows.Close()
			return err
		}
		rawTags = append(rawTags, tagStr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Now that the cursor is closed, prepare and execute the inserts
	seen := make(map[string]bool)
	tagStmt, err := tx.Prepare("INSERT INTO l2t_tags (tag) VALUES (" + db.dialect.Placeholder(1) + ")")
	if err != nil {
		return fmt.Errorf("preparing tag insert: %w", err)
	}
	defer tagStmt.Close()

	for _, tagStr := range rawTags {
		for _, t := range strings.Split(tagStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" && !seen[t] {
				seen[t] = true
				if _, err = tagStmt.Exec(t); err != nil {
					return fmt.Errorf("inserting tag: %w", err)
				}
			}
		}
	}

	return tx.Commit()
}

// RebuildIndexes drops all existing indexes and creates new ones for the given fields.
func (db *PostgresStore) RebuildIndexes(indexFields []string) error {
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

// GetSavedQueries returns all saved queries from the database.
func (db *PostgresStore) GetSavedQueries() ([]SavedQuery, error) {
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
func (db *PostgresStore) SaveQuery(name, query string) error {
	_, err := db.conn.Exec(
		"INSERT INTO l2t_saved_query (name, query) VALUES ("+db.dialect.Placeholder(1)+", "+db.dialect.Placeholder(2)+")",
		name, query,
	)
	return err
}

// DeleteQuery removes a saved query by name.
func (db *PostgresStore) DeleteQuery(name string) error {
	_, err := db.conn.Exec(
		"DELETE FROM l2t_saved_query WHERE name = "+db.dialect.Placeholder(1),
		name,
	)
	return err
}

// ExecuteQuery runs a pre-built SQL SELECT and scans results using model.Fields
// column order (Pattern B: id, datetime, timezone, MACB, ...).
func (db *PostgresStore) ExecuteQuery(sqlStr string, args []interface{}) ([]*model.Event, error) {
	rows, err := db.conn.Query(sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()
	return pgScanFieldsOrderEvents(rows)
}

// ExecuteCountQuery runs a pre-built COUNT query and returns the result.
func (db *PostgresStore) ExecuteCountQuery(sqlStr string, args []interface{}) (int64, error) {
	var count int64
	err := db.conn.QueryRow(sqlStr, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("executing count query: %w", err)
	}
	return count, nil
}

// GetTimelineHistogram returns event counts bucketed by time interval.
// Uses to_char for PostgreSQL TIMESTAMP formatting and date range detection.
func (db *PostgresStore) GetTimelineHistogram(whereClause string, whereArgs []interface{}) ([]TimelineBucket, error) {
	// Get date range to determine bucket size.
	// Use to_char to produce consistent text output from TIMESTAMP columns.
	rangeSQL := `SELECT COALESCE(to_char(MIN(datetime), 'YYYY-MM-DD HH24:MI:SS'), ''),
	                    COALESCE(to_char(MAX(datetime), 'YYYY-MM-DD HH24:MI:SS'), '')
	             FROM log2timeline`
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

	// Choose bucket format based on date range span.
	// These are strftime-style strings; DateFormatSQL maps them to to_char equivalents.
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

// pgScanEvents converts sql.Rows into a slice of Event pointers using
// NULL-safe scanning for PostgreSQL. PostgreSQL returns actual NULLs for
// empty columns, unlike SQLite which returns empty strings. All TEXT columns
// are scanned through sql.NullString and all INT columns through sql.NullInt64
// to gracefully convert NULLs to Go zero values.
//
// This is Pattern A column order (matching PostgresStore.QueryEvents SELECT):
//
//	id, timezone, MACB, source, sourcetype, type, user, host, desc,
//	filename, inode, notes, format, extra, datetime, reportnotes,
//	inreport, tag, color, offset, store_number, store_index,
//	vss_store_number, URL, record_number, event_identifier, event_type,
//	source_name, user_sid, computer_name, bookmark
func pgScanEvents(rows *sql.Rows) ([]*model.Event, error) {
	var events []*model.Event
	for rows.Next() {
		var (
			id                                                      int64
			timezone, macb, source, sourcetype, typ                 sql.NullString
			user, host, desc, filename, inode                       sql.NullString
			notes, format, extra, datetime, reportnotes             sql.NullString
			inreport, tag, color                                    sql.NullString
			offset, storeNumber, storeIndex, vssStoreNumber         sql.NullInt64
			url, recordNumber, eventID, eventType                   sql.NullString
			sourceName, userSID, computerName                       sql.NullString
			bookmark                                                sql.NullInt64
		)

		err := rows.Scan(
			&id, &timezone, &macb, &source, &sourcetype,
			&typ, &user, &host, &desc, &filename,
			&inode, &notes, &format, &extra, &datetime,
			&reportnotes, &inreport, &tag, &color, &offset,
			&storeNumber, &storeIndex, &vssStoreNumber, &url,
			&recordNumber, &eventID, &eventType, &sourceName,
			&userSID, &computerName, &bookmark,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}

		e := &model.Event{
			ID:             id,
			Timezone:       timezone.String,
			MACB:           macb.String,
			Source:         source.String,
			SourceType:     sourcetype.String,
			Type:           typ.String,
			User:           user.String,
			Host:           host.String,
			Desc:           desc.String,
			Filename:       filename.String,
			Inode:          inode.String,
			Notes:          notes.String,
			Format:         format.String,
			Extra:          extra.String,
			Datetime:       datetime.String,
			ReportNotes:    reportnotes.String,
			InReport:       inreport.String,
			Tag:            tag.String,
			Color:          color.String,
			Offset:         offset.Int64,
			StoreNumber:    storeNumber.Int64,
			StoreIndex:     storeIndex.Int64,
			VSSStoreNumber: vssStoreNumber.Int64,
			URL:            url.String,
			RecordNumber:   recordNumber.String,
			EventID:        eventID.String,
			EventType:      eventType.String,
			SourceName:     sourceName.String,
			UserSID:        userSID.String,
			ComputerName:   computerName.String,
			Bookmark:       bookmark.Int64,
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// pgScanFieldsOrderEvents converts sql.Rows into a slice of Event pointers
// using NULL-safe scanning for PostgreSQL. Same NULL handling rationale as
// pgScanEvents.
//
// This is Pattern B column order (matching query.go Build() SELECT via model.Fields):
//
//	id, datetime, timezone, MACB, source, sourcetype, type, user, host, desc,
//	filename, inode, notes, format, extra, reportnotes, inreport, tag, color,
//	offset, store_number, store_index, vss_store_number, URL, record_number,
//	event_identifier, event_type, source_name, user_sid, computer_name, bookmark
func pgScanFieldsOrderEvents(rows *sql.Rows) ([]*model.Event, error) {
	var events []*model.Event
	for rows.Next() {
		var (
			id                                                      int64
			datetime, timezone, macb, source, sourcetype, typ       sql.NullString
			user, host, desc, filename                              sql.NullString
			inode, notes, format, extra, reportnotes                sql.NullString
			inreport, tag, color                                    sql.NullString
			offset, storeNumber, storeIndex, vssStoreNumber         sql.NullInt64
			url, recordNumber, eventID, eventType                   sql.NullString
			sourceName, userSID, computerName                       sql.NullString
			bookmark                                                sql.NullInt64
		)

		err := rows.Scan(
			&id, &datetime, &timezone, &macb, &source, &sourcetype,
			&typ, &user, &host, &desc, &filename,
			&inode, &notes, &format, &extra, &reportnotes,
			&inreport, &tag, &color, &offset, &storeNumber,
			&storeIndex, &vssStoreNumber, &url, &recordNumber,
			&eventID, &eventType, &sourceName, &userSID, &computerName,
			&bookmark,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}

		e := &model.Event{
			ID:             id,
			Datetime:       datetime.String,
			Timezone:       timezone.String,
			MACB:           macb.String,
			Source:         source.String,
			SourceType:     sourcetype.String,
			Type:           typ.String,
			User:           user.String,
			Host:           host.String,
			Desc:           desc.String,
			Filename:       filename.String,
			Inode:          inode.String,
			Notes:          notes.String,
			Format:         format.String,
			Extra:          extra.String,
			ReportNotes:    reportnotes.String,
			InReport:       inreport.String,
			Tag:            tag.String,
			Color:          color.String,
			Offset:         offset.Int64,
			StoreNumber:    storeNumber.Int64,
			StoreIndex:     storeIndex.Int64,
			VSSStoreNumber: vssStoreNumber.Int64,
			URL:            url.String,
			RecordNumber:   recordNumber.String,
			EventID:        eventID.String,
			EventType:      eventType.String,
			SourceName:     sourceName.String,
			UserSID:        userSID.String,
			ComputerName:   computerName.String,
			Bookmark:       bookmark.Int64,
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
