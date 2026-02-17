package database

import "fmt"

// pgQuoteCol wraps a column name in double quotes if it is a PostgreSQL reserved word.
// Columns like "user", "desc", and "offset" require quoting to avoid conflicts
// with PostgreSQL keywords. Non-reserved names are returned as-is so PostgreSQL
// folds them to lowercase consistently with unquoted DDL definitions.
func pgQuoteCol(name string) string {
	switch name {
	case "user", "desc", "offset":
		return `"` + name + `"`
	default:
		return name
	}
}

// strftimeToPostgres maps the SQLite strftime format strings used by
// GetTimelineHistogram to their PostgreSQL to_char equivalents.
var strftimeToPostgres = map[string]string{
	"%Y-%m-%d %H:00:00": "YYYY-MM-DD HH24:00:00",
	"%Y-%m-%d":           "YYYY-MM-DD",
	"%Y-%m":              "YYYY-MM",
}

// PostgresDialect implements the Dialect interface for PostgreSQL databases.
// It also satisfies query.QueryDialect through structural typing.
type PostgresDialect struct{}

func (d *PostgresDialect) DriverName() string              { return "pgx" }
func (d *PostgresDialect) DSN(pathOrConnStr string) string  { return pathOrConnStr }
func (d *PostgresDialect) Placeholder(index int) string     { return fmt.Sprintf("$%d", index) }
func (d *PostgresDialect) IDColumn() string                 { return "id" }
func (d *PostgresDialect) QuoteColumn(name string) string   { return pgQuoteCol(name) }

func (d *PostgresDialect) DateBetweenSQL(paramIdx1, paramIdx2 int) string {
	return fmt.Sprintf("(datetime BETWEEN %s AND %s)",
		d.Placeholder(paramIdx1), d.Placeholder(paramIdx2))
}

func (d *PostgresDialect) DateFormatSQL(column, format string) string {
	pgFmt, ok := strftimeToPostgres[format]
	if !ok {
		pgFmt = format
	}
	return fmt.Sprintf("to_char(%s, '%s')", column, pgFmt)
}

func (d *PostgresDialect) SchemaCheckColumnSQL(table, column string) string {
	return fmt.Sprintf(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_name='%s' AND column_name='%s'",
		table, column)
}

func (d *PostgresDialect) CreateTableSQL() string {
	return `CREATE TABLE IF NOT EXISTS log2timeline (
		id SERIAL PRIMARY KEY,
		timezone TEXT, MACB TEXT, source TEXT, sourcetype TEXT,
		type TEXT, "user" TEXT, host TEXT, "desc" TEXT, filename TEXT,
		inode TEXT, notes TEXT, format TEXT, extra TEXT,
		datetime TIMESTAMP, reportnotes TEXT, inreport TEXT,
		tag TEXT, color TEXT, "offset" INT, store_number INT,
		store_index INT, vss_store_number INT, URL TEXT,
		record_number TEXT, event_identifier TEXT, event_type TEXT,
		source_name TEXT, user_sid TEXT, computer_name TEXT,
		bookmark INT DEFAULT 0
	)`
}

func (d *PostgresDialect) CreateMetadataTableSQL(tableName, columnName string) string {
	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (%s TEXT, frequency INT)", tableName, pgQuoteCol(columnName))
}

func (d *PostgresDialect) CreateTagsTableSQL() string {
	return "CREATE TABLE IF NOT EXISTS l2t_tags (tag TEXT)"
}

func (d *PostgresDialect) CreateSavedQueryTableSQL() string {
	return "CREATE TABLE IF NOT EXISTS l2t_saved_query (name TEXT, query TEXT)"
}

func (d *PostgresDialect) CreateDiskTableSQL() string {
	return `CREATE TABLE IF NOT EXISTS l2t_disk (
		disk_type INT, mount_path TEXT, dd_path TEXT,
		dd_offset TEXT, storage_file TEXT, export_path TEXT
	)`
}

func (d *PostgresDialect) InsertDefaultDiskSQL() string {
	return `INSERT INTO l2t_disk
		(disk_type, mount_path, dd_path, dd_offset, storage_file, export_path)
		VALUES (0, '', '', '', '', '')`
}

func (d *PostgresDialect) CreateIndexSQL(indexName, tableName, column string) string {
	return fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, tableName, pgQuoteCol(column))
}

func (d *PostgresDialect) DropIndexSQL(indexName string) string {
	return fmt.Sprintf("DROP INDEX IF EXISTS %s", indexName)
}

func (d *PostgresDialect) InsertEventSQL() string {
	return `INSERT INTO log2timeline (
		timezone, MACB, source, sourcetype, type, "user", host, "desc", filename,
		inode, notes, format, extra, datetime, reportnotes, inreport, tag, color,
		"offset", store_number, store_index, vss_store_number, URL, record_number,
		event_identifier, event_type, source_name, user_sid, computer_name, bookmark
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30)`
}
