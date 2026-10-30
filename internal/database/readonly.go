package database

import (
	"errors"
	"os"
	"path/filepath"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrReadOnlyUpgrade is wrapped by OpenSQLite when the database needs a
// schema migration (it was created by an older version) and the migration
// could not write because the file or its directory is read-only. The store
// is closed and the file is unchanged: a read-only file cannot be written,
// and a read-only directory blocks the rollback journal SQLite needs before
// it changes anything.
//
// A database that needs no migration opens normally on read-only storage:
// 4n6time never enables WAL, so reads need no -wal or -shm file, and the
// open-time CREATE ... IF NOT EXISTS statements write nothing when the
// objects exist.
var ErrReadOnlyUpgrade = errors.New("database is on read-only storage and needs a one-time upgrade to open")

// isReadOnlyMigrationError reports whether a migration failure on the SQLite
// database at path came from read-only storage. SQLITE_READONLY (any extended
// code) means the file opened read-only. SQLITE_CANTOPEN means SQLite could
// not create its journal; it counts as read-only only when the directory is
// not writable, so other open failures keep their own error.
func isReadOnlyMigrationError(err error, path string) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	switch se.Code() & 0xff {
	case sqlite3.SQLITE_READONLY:
		return true
	case sqlite3.SQLITE_CANTOPEN:
		return !dirWritable(filepath.Dir(path))
	}
	return false
}

// dirWritable reports whether a file can be created in dir. It runs only
// after a migration has already failed with SQLITE_CANTOPEN, so on read-only
// storage it creates nothing; on writable storage the probe file is removed.
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".4n6time-write-check-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
