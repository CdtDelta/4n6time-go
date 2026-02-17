package database

import "fmt"

// SQLiteDialect implements the Dialect interface for SQLite databases.
// It also satisfies query.QueryDialect through structural typing.
type SQLiteDialect struct{}

func (d *SQLiteDialect) DriverName() string                { return "sqlite" }
func (d *SQLiteDialect) DSN(pathOrConnStr string) string    { return pathOrConnStr }
func (d *SQLiteDialect) Placeholder(index int) string       { return "?" }
func (d *SQLiteDialect) IDColumn() string                   { return "rowid" }
func (d *SQLiteDialect) QuoteColumn(name string) string     { return name }

func (d *SQLiteDialect) DateBetweenSQL(paramIdx1, paramIdx2 int) string {
	return "(datetime BETWEEN datetime(?) AND datetime(?))"
}

func (d *SQLiteDialect) DateFormatSQL(column, format string) string {
	return fmt.Sprintf("strftime('%s', %s)", format, column)
}

func (d *SQLiteDialect) SchemaCheckColumnSQL(table, column string) string {
	return fmt.Sprintf(
		"SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'", table, column)
}

func (d *SQLiteDialect) CreateTableSQL() string {
	return `CREATE TABLE IF NOT EXISTS log2timeline (
		timezone TEXT, MACB TEXT, source TEXT, sourcetype TEXT,
		type TEXT, user TEXT, host TEXT, desc TEXT, filename TEXT,
		inode TEXT, notes TEXT, format TEXT, extra TEXT,
		datetime DATETIME, reportnotes TEXT, inreport TEXT,
		tag TEXT, color TEXT, offset INT, store_number INT,
		store_index INT, vss_store_number INT, URL TEXT,
		record_number TEXT, event_identifier TEXT, event_type TEXT,
		source_name TEXT, user_sid TEXT, computer_name TEXT,
		bookmark INT DEFAULT 0
	)`
}

func (d *SQLiteDialect) CreateMetadataTableSQL(tableName, columnName string) string {
	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (%s TEXT, frequency INT)", tableName, columnName)
}

func (d *SQLiteDialect) CreateTagsTableSQL() string {
	return "CREATE TABLE IF NOT EXISTS l2t_tags (tag TEXT)"
}

func (d *SQLiteDialect) CreateSavedQueryTableSQL() string {
	return "CREATE TABLE IF NOT EXISTS l2t_saved_query (name TEXT, query TEXT)"
}

func (d *SQLiteDialect) CreateDiskTableSQL() string {
	return `CREATE TABLE IF NOT EXISTS l2t_disk (
		disk_type INT, mount_path TEXT, dd_path TEXT,
		dd_offset TEXT, storage_file TEXT, export_path TEXT
	)`
}

func (d *SQLiteDialect) InsertDefaultDiskSQL() string {
	return `INSERT INTO l2t_disk
		(disk_type, mount_path, dd_path, dd_offset, storage_file, export_path)
		VALUES (0, '', '', '', '', '')`
}

func (d *SQLiteDialect) CreateIndexSQL(indexName, tableName, column string) string {
	return fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, tableName, column)
}

func (d *SQLiteDialect) DropIndexSQL(indexName string) string {
	return fmt.Sprintf("DROP INDEX IF EXISTS %s", indexName)
}

func (d *SQLiteDialect) InsertEventSQL() string {
	return `INSERT INTO log2timeline (
		timezone, MACB, source, sourcetype, type, user, host, desc, filename,
		inode, notes, format, extra, datetime, reportnotes, inreport, tag, color,
		offset, store_number, store_index, vss_store_number, URL, record_number,
		event_identifier, event_type, source_name, user_sid, computer_name, bookmark
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
}
