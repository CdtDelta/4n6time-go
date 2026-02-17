package database

import "fmt"

// OpenStore opens an existing database using the specified driver.
// For SQLite, pathOrConnStr is the file path to the .db file.
// For PostgreSQL, pathOrConnStr is a connection string (e.g. "postgres://user:pass@host/db").
func OpenStore(driver, pathOrConnStr string) (Store, error) {
	switch driver {
	case "sqlite":
		return OpenSQLite(pathOrConnStr)
	case "postgres":
		return OpenPostgres(pathOrConnStr)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

// CreateStore creates a new database using the specified driver.
// For SQLite, pathOrConnStr is the file path for the new .db file.
// For PostgreSQL, pathOrConnStr is a connection string; the database must already exist.
// indexFields specifies which columns to index; pass nil for defaults.
func CreateStore(driver, pathOrConnStr string, indexFields []string) (Store, error) {
	switch driver {
	case "sqlite":
		return CreateSQLite(pathOrConnStr, indexFields)
	case "postgres":
		return CreatePostgres(pathOrConnStr, indexFields)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}
