package database

// Dialect abstracts all database-specific SQL generation.
// Each database backend (SQLite, PostgreSQL, etc.) implements this interface.
// The Placeholder, IDColumn, and DateBetweenSQL methods match the query.QueryDialect
// interface through Go structural typing, so a Dialect can also serve as a QueryDialect.
type Dialect interface {
	// DriverName returns the database/sql driver name (e.g. "sqlite", "postgres").
	DriverName() string

	// DSN returns the data source name for opening a connection.
	// For SQLite this is the file path; for PostgreSQL it would be a connection string.
	DSN(pathOrConnStr string) string

	// Placeholder returns the parameter placeholder for the given 1-based index.
	// SQLite: "?" (ignoring index), PostgreSQL: "$1", "$2", etc.
	Placeholder(index int) string

	// IDColumn returns the row identifier column name.
	// SQLite: "rowid" (implicit), PostgreSQL: "id" (explicit serial).
	IDColumn() string

	// DateBetweenSQL returns the SQL fragment for a datetime range filter.
	// paramIdx1 and paramIdx2 are the 1-based parameter indices.
	DateBetweenSQL(paramIdx1, paramIdx2 int) string

	// DateFormatSQL returns a SQL expression that formats/truncates a datetime column.
	// SQLite uses strftime; PostgreSQL uses to_char or date_trunc.
	DateFormatSQL(column, format string) string

	// SchemaCheckColumnSQL returns a SQL query that counts how many times a column
	// appears in a table's schema. Used for migration checks.
	// SQLite queries pragma_table_info; PostgreSQL queries information_schema.
	SchemaCheckColumnSQL(table, column string) string

	// CreateTableSQL returns the DDL for the main log2timeline event table.
	CreateTableSQL() string

	// CreateMetadataTableSQL returns DDL for a metadata frequency table.
	CreateMetadataTableSQL(tableName, columnName string) string

	// CreateTagsTableSQL returns DDL for the l2t_tags table.
	CreateTagsTableSQL() string

	// CreateSavedQueryTableSQL returns DDL for the l2t_saved_query table.
	CreateSavedQueryTableSQL() string

	// CreateDiskTableSQL returns DDL for the l2t_disk configuration table.
	CreateDiskTableSQL() string

	// InsertDefaultDiskSQL returns the INSERT statement for the default disk config row.
	InsertDefaultDiskSQL() string

	// CreateIndexSQL returns DDL to create an index on a table column.
	CreateIndexSQL(indexName, tableName, column string) string

	// DropIndexSQL returns DDL to drop an index by name.
	DropIndexSQL(indexName string) string

	// InsertEventSQL returns the parameterized INSERT statement for a single event.
	// The statement has 30 columns and 30 placeholders.
	InsertEventSQL() string

	// QuoteColumn returns the column name quoted appropriately for the dialect.
	// SQLite returns the name unchanged. PostgreSQL wraps reserved words in double quotes.
	// This matches query.QueryDialect.QuoteColumn for structural typing.
	QuoteColumn(name string) string

	// CreateExaminerNotesTableSQL returns DDL for the examiner_notes table.
	// Examiner notes are manually created timeline entries that appear alongside
	// evidence events in the grid using a UNION ALL with negative IDs.
	CreateExaminerNotesTableSQL() string

	// InsertExaminerNoteSQL returns the parameterized INSERT statement for a single examiner note.
	InsertExaminerNoteSQL() string
}
