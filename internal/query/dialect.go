package query

// QueryDialect abstracts SQL syntax differences needed for query building.
// Each database backend provides an implementation. The default is SQLite.
type QueryDialect interface {
	// Placeholder returns the parameter placeholder for the given 1-based index.
	// SQLite returns "?" (ignoring the index), PostgreSQL returns "$1", "$2", etc.
	Placeholder(index int) string

	// IDColumn returns the name of the row identifier column.
	// SQLite uses "rowid", PostgreSQL uses "id".
	IDColumn() string

	// DateBetweenSQL returns the SQL fragment for a datetime range filter.
	// paramIdx1 and paramIdx2 are the 1-based parameter indices for the two date values.
	// SQLite: "(datetime BETWEEN datetime(?) AND datetime(?))"
	// PostgreSQL would use "$N::timestamp" syntax instead.
	DateBetweenSQL(paramIdx1, paramIdx2 int) string

	// QuoteColumn returns the column name quoted appropriately for the dialect.
	// SQLite returns the name unchanged. PostgreSQL wraps reserved words
	// (user, desc, offset) in double quotes.
	QuoteColumn(name string) string
}

// sqliteQueryDialect is the default dialect, producing SQLite-compatible SQL.
type sqliteQueryDialect struct{}

func (d sqliteQueryDialect) Placeholder(index int) string { return "?" }
func (d sqliteQueryDialect) IDColumn() string             { return "rowid" }
func (d sqliteQueryDialect) QuoteColumn(name string) string { return name }

func (d sqliteQueryDialect) DateBetweenSQL(paramIdx1, paramIdx2 int) string {
	return "(datetime BETWEEN datetime(?) AND datetime(?))"
}

// DefaultDialect is the query dialect used when none is explicitly set.
// It produces SQLite-compatible SQL.
var DefaultDialect QueryDialect = sqliteQueryDialect{}
