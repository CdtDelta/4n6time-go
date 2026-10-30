package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/cdtdelta/4n6time/internal/database"
)

// postgresConnStr builds a PostgreSQL connection string with the same
// defaults and percent-encoding as ConnectPostgres and PushToPostgres.
func postgresConnStr(host, port, dbName, user, password, sslMode string) string {
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if sslMode == "" {
		sslMode = "disable"
	}
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     fmt.Sprintf("%s:%s", host, port),
		Path:     dbName,
		RawQuery: "sslmode=" + sslMode,
	}
	return u.String()
}

// NeedsOffsetUpgrade reports whether the open database's offset column is
// still INT. The frontend calls it before an import into a connected
// PostgreSQL database, so the answer is current even if another client
// upgraded the database since this one connected.
func (a *App) NeedsOffsetUpgrade() (bool, error) {
	if a.store == nil {
		return false, fmt.Errorf("no database open")
	}
	return a.store.OffsetNeedsUpgrade()
}

// UpgradeOffsetColumn widens the open PostgreSQL database's offset column to
// BIGINT. It can take a long time on a large table, which is locked while it runs.
func (a *App) UpgradeOffsetColumn() error {
	if a.store == nil {
		return fmt.Errorf("no database open")
	}
	if a.driver != "postgres" {
		return fmt.Errorf("offset upgrade applies only to PostgreSQL databases")
	}
	return a.runOffsetUpgrade(a.store.Path(), a.store.UpgradeOffsetColumn)
}

// CheckPushTargetOffset reports whether the Push to PostgreSQL target already
// has a log2timeline table whose offset is INT. A target with no schema yet
// returns false, because push creates it with the new DDL.
func (a *App) CheckPushTargetOffset(host, port, dbName, user, password, sslMode string) (bool, error) {
	connStr := postgresConnStr(host, port, dbName, user, password, sslMode)
	needs, err := database.PostgresOffsetNeedsUpgrade(connStr)
	if err != nil {
		a.logError("Push target offset check failed for " + database.MaskConnStr(connStr) + ": " + err.Error())
		return false, fmt.Errorf("checking PostgreSQL target: %w", err)
	}
	return needs, nil
}

// UpgradePushTargetOffset widens the offset column of the Push to PostgreSQL
// target before the push runs.
func (a *App) UpgradePushTargetOffset(host, port, dbName, user, password, sslMode string) error {
	connStr := postgresConnStr(host, port, dbName, user, password, sslMode)
	return a.runOffsetUpgrade(database.MaskConnStr(connStr), func() error {
		return database.UpgradePostgresOffset(connStr)
	})
}

// runOffsetUpgrade runs upgrade and logs its start, finish, and duration.
// target is a display name with any password already masked.
func (a *App) runOffsetUpgrade(target string, upgrade func() error) error {
	start := time.Now()
	a.logInfo("Offset column upgrade started: " + target)
	if err := upgrade(); err != nil {
		a.logError(fmt.Sprintf("Offset column upgrade failed on %s after %s: %v",
			target, time.Since(start).Round(time.Millisecond), err))
		return err
	}
	a.logInfo(fmt.Sprintf("Offset column upgrade finished: %s in %s",
		target, time.Since(start).Round(time.Millisecond)))
	return nil
}
