package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/jackc/pgx/v5/pgconn"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Open failure actions: which call failed.
const (
	openActionOpen    = "open"    // OpenDatabase (SQLite file)
	openActionConnect = "connect" // ConnectPostgres
	openActionCreate  = "create"  // CreatePostgresDatabase
)

// Plain-language explanations shown by OpenFailureDialog. Anything not
// recognized gets openMsgGeneric; the raw error is always shown as well.
const (
	openMsgNotDatabase  = "This file isn't a 4n6time database."
	openMsgCantOpenFile = "The database file could not be opened. Check that it exists and that you have permission to read it."
	openMsgCorrupt      = "The database file is damaged."
	openMsgUnreachable  = "Could not reach the PostgreSQL server."
	openMsgAuth         = "The PostgreSQL server rejected the username or password."
	openMsgNoPGDatabase = "The PostgreSQL database does not exist."
	openMsgNoPGSchema   = "This PostgreSQL database has no 4n6time schema. Use Create & Connect to create one."
	openMsgPermission   = "The PostgreSQL user does not have permission to use this database."
	openMsgGeneric      = "The database could not be opened."
	// openMsgTimeoutFmt takes the connect timeout in effect (timeoutText).
	openMsgTimeoutFmt = "Could not reach the PostgreSQL server (no response after %s)."
)

// OpenFailure describes an open, connect, or create that failed for any
// reason other than an older database on read-only storage, which has its own
// ReadOnlyOpenFailure. The current database, if any, is still open: every
// path opens the new store before closing the old one. The frontend reads it
// with GetOpenFailure after the call rejects and shows OpenFailureDialog.
type OpenFailure struct {
	Backend string `json:"backend"` // "sqlite" or "postgres"
	Action  string `json:"action"`  // openActionOpen, openActionConnect, or openActionCreate
	// Target is the SQLite file path, or the PostgreSQL connection string
	// with the password removed (database.MaskConnStr).
	Target  string `json:"target"`
	Message string `json:"message"` // plain-language explanation (openFailureMessage)
	Error   string `json:"error"`   // the raw error text, as the status bar shows it
	// StillOpen names the database that is still open (its Path, masked for
	// PostgreSQL), or "" when none is.
	StillOpen string `json:"stillOpen"`
}

// GetOpenFailure returns the failure from the most recent open, connect, or
// create call, or nil when that call succeeded, was cancelled, or failed
// because of read-only storage.
func (a *App) GetOpenFailure() *OpenFailure {
	a.importFailMu.Lock()
	defer a.importFailMu.Unlock()
	return a.lastOpenFailure
}

// setOpenFailure stores f (nil clears it). It shares importFailMu with the
// other last-failure records.
func (a *App) setOpenFailure(f *OpenFailure) {
	a.importFailMu.Lock()
	a.lastOpenFailure = f
	a.importFailMu.Unlock()
}

// recordOpenFailure logs a failed open, connect, or create at ERROR and
// stores it for GetOpenFailure. target must already be masked for
// PostgreSQL. err is the error the method returns.
func (a *App) recordOpenFailure(backend, action, target string, err error) {
	stillOpen := ""
	if a.store != nil {
		stillOpen = a.store.Path()
	}
	a.logError(fmt.Sprintf("Database %s failed: %s %s: %s", action, backend, target, err.Error()))
	a.setOpenFailure(&OpenFailure{
		Backend:   backend,
		Action:    action,
		Target:    target,
		Message:   openFailureMessage(err),
		Error:     err.Error(),
		StillOpen: stillOpen,
	})
}

// openFailureMessage explains err in plain language. It checks the driver's
// typed errors first (PostgreSQL SQLSTATE codes, SQLite result codes, network
// errors), then known text, and falls back to openMsgGeneric.
func openFailureMessage(err error) string {
	// Before the network checks: a timeout is also a net.Error.
	var timeoutErr *database.ConnectTimeoutError
	if errors.As(err, &timeoutErr) {
		return fmt.Sprintf(openMsgTimeoutFmt, timeoutText(timeoutErr.Timeout))
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "28P01", "28000": // invalid_password, invalid_authorization_specification
			return openMsgAuth
		case "3D000": // invalid_catalog_name
			return openMsgNoPGDatabase
		case "42P01": // undefined_table
			return openMsgNoPGSchema
		case "42501": // insufficient_privilege
			return openMsgPermission
		}
	}

	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() & 0xff {
		case sqlite3.SQLITE_NOTADB:
			return openMsgNotDatabase
		case sqlite3.SQLITE_CANTOPEN:
			return openMsgCantOpenFile
		case sqlite3.SQLITE_CORRUPT:
			return openMsgCorrupt
		}
	}

	text := err.Error()
	switch {
	case strings.Contains(text, "file is not a database"),
		strings.Contains(text, "no such table: log2timeline"):
		// The second is a valid SQLite file without the 4n6time tables.
		return openMsgNotDatabase
	case strings.Contains(text, `relation "log2timeline" does not exist`):
		return openMsgNoPGSchema
	case strings.Contains(text, "unable to open database file"):
		return openMsgCantOpenFile
	}

	var opErr *net.OpError
	var dnsErr *net.DNSError
	if errors.As(err, &opErr) || errors.As(err, &dnsErr) ||
		strings.Contains(text, "connection refused") ||
		strings.Contains(text, "no such host") ||
		strings.Contains(text, "i/o timeout") ||
		strings.Contains(text, "network is unreachable") {
		return openMsgUnreachable
	}
	return openMsgGeneric
}

// timeoutText writes a connect timeout as "10 seconds" or "1 second", or in
// Go duration form when it is not whole seconds.
func timeoutText(d time.Duration) string {
	if d%time.Second != 0 {
		return d.String()
	}
	if n := int(d / time.Second); n != 1 {
		return fmt.Sprintf("%d seconds", n)
	}
	return "1 second"
}
