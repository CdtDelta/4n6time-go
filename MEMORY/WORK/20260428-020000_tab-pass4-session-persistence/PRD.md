---
task: Tab Pass 4 session persistence and restore
slug: 20260428-020000_tab-pass4-session-persistence
effort: advanced
phase: complete
progress: 48/48
mode: interactive
started: 2026-04-28T02:00:00Z
updated: 2026-04-28T03:00:00Z
---

## Context

Tab System Pass 4 of 4. Tabs are fully functional but ephemeral — they vanish when the database is closed or the app exits. This pass adds session persistence: tab layout is saved to the database (l2t_tab_sessions) on close and can be restored on next open. Only non-main tabs are saved (the main "All Events" tab is always created fresh). Session data is a JSON blob with tab definitions and the active tab ID. Works with both SQLite and PostgreSQL.

### Key Design Decisions
- l2t_tab_sessions table: one row max (delete-all-then-insert pattern)
- Auto-save: handleCloseDB saves before closing; beforeunload fires a best-effort save for window-close
- Restore prompt: notification bar below tab bar unless auto-restore is enabled in Settings
- appSettings extended with AutoRestoreTabs bool (stored in settings.json alongside TabLimit)
- Refs (tabsRef, activeTabIdRef) track latest tab state for the once-registered beforeunload handler

## Criteria

### Dialect (schema)
- [x] ISC-1: `dialect.go` Dialect interface has `CreateTabSessionsTableSQL() string`
- [x] ISC-2: `SQLiteDialect.CreateTabSessionsTableSQL` returns DDL with `INTEGER PRIMARY KEY` and `TEXT session_data`
- [x] ISC-3: `PostgresDialect.CreateTabSessionsTableSQL` returns DDL with `SERIAL PRIMARY KEY` and `TEXT session_data`

### Migration
- [x] ISC-4: `SQLiteStore.migrate()` creates l2t_tab_sessions via `db.dialect.CreateTabSessionsTableSQL()`
- [x] ISC-5: `PostgresStore.migrate()` creates l2t_tab_sessions via `db.dialect.CreateTabSessionsTableSQL()`

### Store interface
- [x] ISC-6: `store.go` has `SaveTabSession(data string) error`
- [x] ISC-7: `store.go` has `LoadTabSession() (string, error)`

### SQLiteStore implementation
- [x] ISC-8: `SQLiteStore.SaveTabSession` deletes all rows then inserts data (empty string skips insert)
- [x] ISC-9: `SQLiteStore.LoadTabSession` returns session_data string or empty string if no row

### PostgresStore implementation
- [x] ISC-10: `PostgresStore.SaveTabSession` deletes all rows then inserts data (empty string skips insert)
- [x] ISC-11: `PostgresStore.LoadTabSession` returns session_data string or empty string if no row

### Wails app.go methods
- [x] ISC-12: `app.go` has `SaveTabSession(sessionJSON string) error` calling `store.SaveTabSession`
- [x] ISC-13: `app.go` has `LoadTabSession() (string, error)` calling `store.LoadTabSession`
- [x] ISC-14: `appSettings` struct has `AutoRestoreTabs bool` field
- [x] ISC-15: `app.go` has `GetAutoRestoreTabs() bool` reading from settings.json
- [x] ISC-16: `app.go` has `SetAutoRestoreTabs(enabled bool) error` writing to settings.json

### JS bindings
- [x] ISC-17: `App.js` has `SaveTabSession(arg1)` export
- [x] ISC-18: `App.js` has `LoadTabSession()` export
- [x] ISC-19: `App.js` has `GetAutoRestoreTabs()` export
- [x] ISC-20: `App.js` has `SetAutoRestoreTabs(arg1)` export
- [x] ISC-21: `App.d.ts` has `SaveTabSession(arg1:string):Promise<void>`
- [x] ISC-22: `App.d.ts` has `LoadTabSession():Promise<string>`
- [x] ISC-23: `App.d.ts` has `GetAutoRestoreTabs():Promise<boolean>`
- [x] ISC-24: `App.d.ts` has `SetAutoRestoreTabs(arg1:boolean):Promise<void>`

### Frontend — save on close
- [x] ISC-25: `App.jsx` imports `SaveTabSession`, `LoadTabSession`, `GetAutoRestoreTabs`
- [x] ISC-26: `handleCloseDB` serializes non-main tabs and calls `SaveTabSession` before `CloseDatabase`
- [x] ISC-27: Empty session (main-tab-only) calls `SaveTabSession('')` to clear stored session
- [x] ISC-28: `tabsRef` and `activeTabIdRef` refs track latest tab state for beforeunload handler
- [x] ISC-29: One-time `beforeunload` handler registered in `useEffect([])` using refs, saves session fire-and-forget
- [x] ISC-30: `handleCloseDB` clears `pendingSession` state

### Frontend — restore on open
- [x] ISC-31: `handleOpenDB` calls `LoadTabSession()` after `loadPage` and sets `pendingSession` or auto-restores
- [x] ISC-32: `handleImportCSV` calls `LoadTabSession()` after `loadPage` (only if no non-main tabs already open)
- [x] ISC-33: `handlePostgresConnect` calls `LoadTabSession()` after `loadPage` and sets `pendingSession` or auto-restores
- [x] ISC-34: `handleRestoreTabSession` creates tabs via `makeTab`, sets tabs state, activeTabId, and activeBaseQueryRef
- [x] ISC-35: `handleDismissSession` calls `SaveTabSession('')` to clear and sets `pendingSession(null)`
- [x] ISC-36: Notification bar shown below tab bar when `pendingSession` is non-null
- [x] ISC-37: Notification bar shows tab count and has "Restore" and "Dismiss" buttons
- [x] ISC-38: Restore button calls `handleRestoreTabSession(pendingSession)` and clears `pendingSession`
- [x] ISC-39: `style.css` has `.session-restore-bar` and button styles

### Settings UI
- [x] ISC-40: `SettingsDialog.jsx` imports `GetAutoRestoreTabs` and `SetAutoRestoreTabs`
- [x] ISC-41: SettingsDialog has `autoRestore` state loaded from `GetAutoRestoreTabs()` on visible
- [x] ISC-42: SettingsDialog has "Auto-restore tabs" checkbox bound to `autoRestore` state
- [x] ISC-43: `handleApply` in SettingsDialog calls `SetAutoRestoreTabs(autoRestore)` alongside `SetTabLimit`

### Anti-criteria
- [x] ISC-A1: Main "All Events" tab never appears in saved session JSON
- [x] ISC-A2: Session save errors are silently ignored (do not block close)
- [x] ISC-A3: Restored tabs contain only id/label/baseQuery — no events, page, scroll, or filter data

## Decisions

### Delete-all-then-insert for SaveTabSession
One row max. Passing empty string deletes any existing row without inserting. This keeps the table lean and makes "clear session" the same as "save empty".

### beforeunload refs pattern
Register handler once (empty deps useEffect). Track latest tab state via tabsRef/activeTabIdRef that update on every render. Avoids re-registering on every tab change.

### Session restore after handleImportCSV
Only triggers if no non-main tabs are currently open. If tabs already exist (restored from earlier or user-created), the import flow doesn't disturb them.

### Auto-restore skips prompt
GetAutoRestoreTabs() is called inline in the open/import handlers. If true, handleRestoreTabSession is called immediately, no pendingSession set.

## Verification
