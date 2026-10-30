# Changelog

All notable changes to 4n6time-go are documented in this file.

## v0.19.0 (2026-10-30)

### Performance

- **The total event count is cached.** Opening, connecting to, or importing into a database, and loading the grid with no filters, now read the evidence total from a new `l2t_stats` table instead of counting every row. On a 77.5M-row PostgreSQL database that count took about 24 seconds each time. Every import and push updates the cached total in the same transaction that inserts the events, so a failed import never changes it and simultaneous imports into one PostgreSQL database add up correctly. Filtered counts and the examiner notes count are still counted live. New databases (created by an import, by Create & Connect, or as a new push target) start with a total of zero, so their first import is counted without a full count afterward.
- **Superseded grid and histogram loads are cancelled.** Paging, filtering, or searching again while a load is still running now cancels the older load, on the PostgreSQL server as well as in the app, so it no longer competes with the new one, and an older result can no longer replace a newer one. Cancelled loads show no error.
- **Bulk color and tag edits no longer reload the page.** The edited rows are updated in place, as bulk bookmarking already was, so an edit of a few rows on a very large database takes about as long as the edit itself instead of minutes. The page still reloads when the current view filters on the edited field (a filter, tab, keyword search, or advanced clause on color or tag). Bulk color is now one statement instead of one per row (on SQLite, one per 500 rows).
- **The filter panel loads its dropdown values from the metadata tables instead of scanning the event table.** Source, sourcetype, type, user, and host come from the `l2t_*` tables, and Import from the import records, in one call. On a 77.5M-row PostgreSQL database, Clear Filters used to take minutes while the panel scanned every event once per field. The event table is scanned only if the metadata was never built.
- **The filter panel no longer reloads on Apply, Clear, or histogram selection.** It reloads only when the data changes: opening or connecting to a database, an import, a push to PostgreSQL, adding or deleting an examiner note, or refreshing a tab.
- **Filter panel values are cached per tab.** Switching back to a tab reuses its values instead of querying again. The cache is cleared whenever the data changes.

### New

- **Tools > Recount Events** counts the events live and stores the result as the cached total, then shows the new total. Use it if the total shown looks wrong; the old and new values are logged.
- **Import failure dialog.** When a single-file import fails, a dialog now shows the file name, the error text, and what the failure left in the database (for example "No events from this file were added"), with a button to open the Imports list. Before, the error appeared only in the status bar, where the next grid load replaced it. The status bar message is kept.
- **Imports list (File > Imports...).** A read-only list of every import record in the open database: ID, label, file name, status, event count, import time (UTC), and error message. Failed imports are highlighted.
- **Import records keep the error text.** `l2t_imports` has a new `error_message` column, filled for failed imports. Push to PostgreSQL copies it.
- **Read-only databases that need an upgrade.** Opening a database created by an older version of 4n6time from read-only storage (a write-protected drive, or a read-only file or folder) now shows a dialog explaining that it needs a one-time upgrade, that the upgrade cannot be written there, and that you can open a copy saved to a writable location. The original file is not changed. Before, the raw database error appeared in the status bar. Databases that are already current still open normally from read-only storage.
- **Open failure dialog.** When a database file cannot be opened for any reason other than read-only storage, a dialog now shows the file, a plain explanation (for example "This file isn't a 4n6time database."), the raw error, and which database is still open. PostgreSQL connect and create failures appear inline in the connect dialog instead, with the same plain explanation (for example "Could not reach the PostgreSQL server.") above the raw error, so the settings can be corrected there. Before, only the status bar showed the error. The status bar message is kept.
- **Reset in the PostgreSQL connect dialog.** A Reset link under SSL Mode restores every field to its first-open value (the host from Settings) and clears the error. The password is cleared whenever the dialog closes; the other fields are kept while you stay on the welcome screen. The host is filled from Settings on the dialog's first open and on Reset only, so a host you typed is kept when you reopen the dialog.
- **PostgreSQL insert errors name the column.** An error such as `failed to encode args[18]` now also says which column and value caused it, for example `inserting event 15017285: column "offset" value 5747310864: ...`. The original error text is kept.

### Fixes

- **Switching databases clears the previous database's tabs.** After a successful open, connect, or create, the tab bar starts from All Events with no search or filters, and the new database's saved tabs are offered as usual. Before, the previous database's tabs stayed open and applied their filters to the new database.
- **PostgreSQL connections time out after 10 seconds.** Connecting to an unreachable server used to hang. Connect, Create & Connect, push targets, and the offset upgrade checks now give up after 10 seconds. The connect dialog then says "Could not reach the PostgreSQL server (no response after 10 seconds)."; push and the offset checks report "no response from the PostgreSQL server after 10s". A `PGCONNECT_TIMEOUT` environment variable still takes precedence.
- **Cancel works while connecting.** Cancel in the PostgreSQL dialog now stops a connect or create in progress; the current database stays open, the dialog stays open, and only "Connection attempt cancelled." is shown. Only the clicked button shows "Connecting..." or "Creating...", and the dialog's buttons wrap instead of running past its edge.
- **The SSL Mode list is readable in dark themes.** It used the system's white drop-down in some themes; it now uses the theme's colors like the other fields.
- **Opening a database no longer closes the current one first.** The new database is opened before the current one is closed, and the current database's tabs are saved first. If the open fails (including a read-only database that needs an upgrade), the current database stays open with its tabs and filters as they were. Before, a failed open left the window showing a database that was already closed, and switching databases lost the previous database's tabs.
- **Only annotation fields can be edited.** Event updates are limited to tag, color, notes, report notes, and bookmark. Requests to change evidence fields such as source, host, datetime, or the import ID are refused.
- **Recursive import tells insert failures from parse failures.** A file that parsed but whose events the database rejected is listed as skipped with "insert error: ..." instead of "parse error: ...".
- **Failed imports that committed events now refresh the view.** When an import fails after its events were saved (the import record or the filter panel metadata could not be updated), the grid, the database details, and the filter panel reload so those events appear.
- **Accurate wording when an import and its record both fail.** The failure dialog now says the record could not be updated and may still show as in progress, instead of "recorded as failed".
- **Import errors are now logged.** A failed import writes its error text at ERROR level with the file name and import ID, for single-file and recursive import. Before, the log recorded only the failed status, and a single-file import error was not logged at all.

### Behavior changes

- **Apply no longer refills blank date fields** with the database's full date range. Clear still resets them.

### Schema

- New table `l2t_stats (key, value)` holding the cached evidence count (`event_count`). Existing databases get it when opened; on read-only storage, a database without it gets the read-only upgrade dialog. A database with no events gets `event_count = 0` when the table is created or migrated; a database that already has events gets its row on first read.
- `l2t_imports` gains a nullable `error_message TEXT` column. Existing databases get it automatically when opened (SQLite and PostgreSQL); existing records read as having no error. A push target gets it when the push opens it.

## v0.18.1 (2026-10-29)

### Fixes

- **PostgreSQL: imports and Push to PostgreSQL no longer fail on large byte offsets.** The `offset` column was a 32-bit INT on PostgreSQL, so a Plaso event with an offset above 2,147,483,647 (any position past 2 GB in a large source file) failed the whole import with `unable to encode ... into binary format for int4`, and the single import transaction rolled back every row already inserted. New PostgreSQL databases now create `offset` as BIGINT. SQLite was not affected: its INTEGER is already 64-bit.

### New

- **Upgrade prompt for older PostgreSQL databases.** Connecting to a database whose `offset` column is still INT shows a dialog offering to upgrade it (**Upgrade now** or **Later**). The upgrade runs `ALTER TABLE log2timeline ALTER COLUMN "offset" TYPE BIGINT` and then `ANALYZE log2timeline`. It rewrites the event table, so on a large database it takes a long time, locks the table while it runs, and needs free disk space roughly equal to the table's size. **Later** lasts for this connection only; the prompt returns on the next connect.
- Importing into a connected PostgreSQL database, or pushing to a PostgreSQL database, whose `offset` is still INT shows the same dialog first, with **Upgrade now**, **Continue without upgrading**, and **Cancel**.
- The upgrade's start, finish, and duration are logged at INFO, and a failure at ERROR with the full error text.

### Schema

- New PostgreSQL databases create `log2timeline.id` as BIGSERIAL (was SERIAL). Existing databases keep SERIAL `id`; it is not migrated and there is no prompt for it.

## v0.18.0 (2026-10-21)

### Behavior changes (query results may differ from v0.17.0)

- **Examiner notes now filter like timeline events, in simple and advanced mode.** Each note behaves as a row with source `EXAMINER`, sourcetype `Examiner Note`, its description as desc, its own datetime, tag, color, and bookmark, and every other field empty. Consequences:
  - Advanced search conditions now apply to notes. Before, any clause without a source condition showed every note.
  - Simple OR filters on fields notes don't have (host, user, and so on) no longer hide all notes: `host = WS1 OR color = RED` shows red notes.
  - Keyword search now filters notes. On SQLite, searching "examiner" or "note" matches every note. Keyword search is case-sensitive on PostgreSQL, so search "EXAMINER" or "Examiner Note" there.
  - `source LIKE` and `source NOT LIKE` now match notes normally, and `source = 'examiner'` (lowercase) no longer matches them.
  - `sourcetype = 'Examiner Note'` selects notes.
- **Simple mode with OR logic: keyword search, bookmark-only, and the date range now narrow the results instead of widening them.** Before, they were ORed with your filters, so OR filters plus a date range could show events outside the range. Affects the grid, count, export, and histogram.
- **Keyword search now covers the URL field in the grid and export** (the histogram already did).
- **Histogram: a LIKE tab pivot now matches the grid** (wrapped as `%value%`), and in advanced mode a pivot operator other than `=` or `LIKE` is an error, as it already was for export.

### New

- **Examiner notes lane in the timeline histogram.** A strip below the bars has a marker for each time slot containing notes. Notes never add to bar heights, which count timeline events only. The tooltip shows both counts. Clicking a marker sets the date range like clicking a bar (simple mode). Window rules:
  - with a date range set, the chart covers that range and shows every note in it
  - with no date range, the chart covers the span of the matching events, and notes outside that span are listed in the grid but not drawn
  - a filter matching only notes scales the chart to those notes
- **Add Note dialog: the date and time are UTC.** Accepted: `YYYY-MM-DD HH:MM:SS`, or a timestamp pasted from the grid (`2009-07-14T04:58:36Z`). Rejected with a message: time zone offsets (never converted), fractional seconds, date-only input, and invalid dates. New notes are stored in the standard format.

### Fixes

- An advanced clause containing the text `order by` (for example `desc LIKE '% order by %'`) failed with a syntax error.
- On SQLite, notes whose time had been pasted from the grid were stored as `…T…Z`, dropped out of date ranges ending on their own day, and sorted out of place within that day. Existing notes are now read correctly with no stored data changed, and new notes are stored in the standard format.

### Internal

- Both examiner-notes filtering engines were replaced by one query that adds notes to the timeline before any filter runs.
- A filter matrix test suite (golden cases plus about 23,000 generated cases) runs on SQLite in every `go test ./...`, and on PostgreSQL when `TEST_PG_DSN` is set. A query plan check fails the tests if the grid or count stops using its indexes.
- Pre-existing gofmt drift fixed.

## v0.17.0 (2026-10-20)

### Added

- **mac_apt import (CSV and JSONL).** Output from ydkhatri's mac_apt (tested with v1.36.2) is imported through `internal/macaptparser`. Twenty fixed-schema outputs and the Spotlight store files produce timeline events, with one event per parseable timestamp column. Network_Details, Network_Interfaces, and FileSharing are recognized but have no timestamps, so folder import lists them as skipped. SQLite, TSV, and XLSX output is not supported.
- **mac_apt recursive import and dedupe.** mac_apt CSV and JSONL files are recursive formats in their own family, so Import Folder (Recursive) groups them in the summary. A CSV and JSONL with the same base name and output in one directory import once, keeping the JSONL.
- **Unparseable timestamp reporting.** The count of timestamp values that do not parse is reported three ways. The recursive summary shows it after each tool's and family's event count. A single-file import shows a notice whose text depends on whether logging was on at import time. When logging is on, the log records up to 10 examples per file as `row <n>, column <name>: "<value>"`. Only the mac_apt parser reports the count today.

## v0.16.1 (2026-10-19)

### Bug fixes

- **Push to PostgreSQL copied examiner notes into the evidence table.** Each note was written twice: once to `examiner_notes`, and once to `log2timeline` as an evidence row with a positive id, source EXAMINER, and sourcetype "Examiner Note", where it could be edited like evidence. Push now reads evidence only, and copies notes only to `examiner_notes`.

### Cleaning up databases pushed with v0.16.0 or earlier

**Who needs this:** any PostgreSQL database that received a push from v0.16.0 or earlier while the SQLite database had examiner notes.

**Pushing again is not a fix.** A second push into the same PostgreSQL database duplicates every event, note, and import record (see TODO.md).

Run the steps below against a copy first if you can. The procedure was verified against a 1.5M-event database reproduced with the v0.16.0 binary: after cleanup, event counts and metadata matched a clean v0.16.1 push exactly.

**Step 1: preview.** The count should equal the number of notes you pushed.

```sql
SELECT count(*) FROM log2timeline l
WHERE l.source = 'EXAMINER' AND l.sourcetype = 'Examiner Note'
  AND EXISTS (SELECT 1 FROM examiner_notes n
              WHERE n.datetime = l.datetime AND n.description = l."desc");
```

**Step 2: delete the evidence rows.** Run this inside a transaction. Compare the DELETE row count with the step 1 count. If they differ, run `ROLLBACK` and stop; do not run step 3.

```sql
BEGIN;
DELETE FROM log2timeline l
WHERE l.source = 'EXAMINER' AND l.sourcetype = 'Examiner Note'
  AND EXISTS (SELECT 1 FROM examiner_notes n
              WHERE n.datetime = l.datetime AND n.description = l."desc");
```

**Step 3: clean the metadata, then commit.** Run this in the same transaction as step 2.

```sql
DELETE FROM l2t_sourcetypes s
  WHERE NOT EXISTS (SELECT 1 FROM log2timeline l WHERE l.sourcetype = s.sourcetype);
DELETE FROM l2t_sources s
  WHERE NOT EXISTS (SELECT 1 FROM log2timeline l WHERE l.source = s.source);
DELETE FROM l2t_colors c
  WHERE NOT EXISTS (SELECT 1 FROM log2timeline l WHERE l.color = c.color);
DELETE FROM l2t_tags t
  WHERE NOT EXISTS (SELECT 1 FROM log2timeline l
                    WHERE l.tag = t.tag OR l.tag LIKE t.tag || ',%'
                       OR l.tag LIKE '%,' || t.tag OR l.tag LIKE '%,' || t.tag || ',%');
UPDATE l2t_sourcetypes s SET frequency = (SELECT count(*) FROM log2timeline l WHERE l.sourcetype = s.sourcetype);
UPDATE l2t_sources s SET frequency = (SELECT count(*) FROM log2timeline l WHERE l.source = s.source);
UPDATE l2t_colors c SET frequency = (SELECT count(*) FROM log2timeline l WHERE l.color = c.color);
COMMIT;
```

**Review, after the commit.** Do not delete anything based on this query:

```sql
SELECT count(*) FROM log2timeline WHERE source = 'EXAMINER' AND sourcetype = 'Examiner Note';
```

A non-zero result can mean either of two things:

- leftover rows whose note was deleted in SQLite after the push
- genuine rows from re-importing an exported CSV, since the export includes notes

You decide case by case which it is.

## v0.16.0 (2026-10-19)

### Features

- **Import records:** every import, single-file and recursive, creates a row in `l2t_imports` (filename, optional label, format, imported_at in UTC, event_count, status) and stamps its id on each event it inserts (`import_id`).
- **Import label:** after you choose a file or folder, you can enter an optional label, such as a hostname. It defaults to the file name without its extension, or to the folder name.
- **Event detail panel:** a read-only Import section shows the label, file, format, and import time. Events from before v0.16.0 show "Imported before v0.16.0 (no import record)".
- **Import filter and pivot:** an Import field in the filter panel selects one import, and "Import" in the right-click "Search in new tab" menu opens a tab for that import.
- **CSV export:** the export ends with an `import_file` column that names the file each event came from.
- **Push to PostgreSQL** copies the import records and remaps each event's import id to its new record.

### Bug fixes

- **Tab base query ANDed outside the filter logic.** In the grid and the result count, a pivot tab's base query was combined with the filter panel's AND/OR logic, so under OR it was ORed with the filters and the tab showed rows outside it. The base is now always ANDed, matching the histogram. **Behavior change:** pivot tabs combined with OR filters show fewer rows.
- **Simple-mode CSV export applies the tab base query.** Export from a pivot tab previously ignored the tab's base query. It now matches the grid, including examiner notes handling.
- **Simple-mode examiner notes respect `source`.** `source = EXAMINER` includes notes. `source = <other>` and `source != EXAMINER` exclude them. `source != <other>` includes them. LIKE on source excludes them. Previously a `source = FILE` pivot or filter still showed examiner notes.
- **Simple-mode notes under OR.** Compatible filters (datetime, bookmark, tag, color, desc) combine with OR on the examiner notes side. Previously they were ANDed, so `color = RED OR color = BLUE` showed no notes.
- **Simple-mode notes and tab base queries.** A tab base query on a compatible field (for example a desc pivot) now restricts examiner notes. Previously every note appeared in such a tab.
- **Schema creation on an existing pre-0.16 table.** Push to PostgreSQL and Create & Connect on a `log2timeline` table written before v0.16.0 now add `import_id` before creating its index. Previously they failed with "column import_id does not exist".

### Internal

- Import provenance migration runs on open and during schema creation. It is automatic and non-destructive. Opening a pre-0.16 database writes the migration, so a read-only pre-0.16 database now fails to open.
- `import_id` is write-once. The stores reject it as an update target.
- Single-file and recursive import record the same per-tool format label.
- File selection is split from import: `SelectImportFile` and `SelectImportFolder` return a path, and `ImportCSV(path, label)` and `ImportFolderRecursive(path, label)` run the import.
- Simple-mode examiner notes inclusion is decided by one planner, `simpleNotesPlan`, shared by the grid, the count, and the export.
- The export header and row mapping have one source: `csvparser.ExportHeader` and `csvparser.ExportRow`.
- Import status constants live in `internal/model`.

## v0.15.1 (2026-10-07)

### Bug fixes

- **Advanced search histogram now matches the grid.** In advanced (SQL) mode the grid, result count, and CSV export run only the tab's base query and the WHERE clause, but the histogram also applied the filter panel, date range, and bookmark-only toggle, so the two could show different events. The histogram now applies only the base query and the clause as well.
- **Controls that do not apply in advanced mode are disabled.** The filter panel (field filters, AND/OR, date range), the bookmark-only toggle, and histogram click and drag-to-select are disabled in advanced mode, with a tooltip explaining that they are not applied. Their values are kept and come back when you switch to simple mode. To filter by bookmark or date in advanced mode, add `bookmark = 1` or a `datetime` condition to the WHERE clause.

## v0.15.0 (2026-10-07)

### New parsers

- **macOS Unified Log** (Mandiant macos-UnifiedLogs `unifiedlog_iterator`, tested with v0.7.0): CSV and JSONL output, auto-detected on single-file import and picked up by recursive folder import. Events import with source UNIFIEDLOG and sourcetype "macOS Unified Log". Event Type maps to type, Log Type to event_type, Message to desc (falling back to Raw Message when empty), Process to filename, Subsystem to source_name, and EUID to user. All event types are imported, including Activity, Signpost, Simpledump, Statedump, and Loss records. The datetime column is second precision like other formats; the full-precision source timestamp is the first entry in the extra column. JSONL also carries message flags and the evidence path (trimmed to the part inside the `.logarchive`). Very large statedump records and activity IDs beyond the signed 64-bit range import correctly.

### Improvements

- **Recursive import summary grouped by tool family**: the post-import dialog groups per-tool counts under family headings (EZ Tools, UnifiedLog) with a subtotal for each family. Tools without a family appear under "Other".
- **Duplicate Unified Log outputs skipped**: when a folder holds more than one export of the same Unified Log data, recursive import imports it once. A CSV and JSONL with the same base name in one directory are duplicates, as are files whose first record matches on boot UUID, timestamp (to the millisecond), and process UUID. The JSONL file is kept because it carries every CSV field plus message flags and evidence. Skipped copies appear in the skipped-files list as "duplicate of <file> (same UnifiedLog data)". A shared boot UUID alone does not mark files as duplicates, since separate collections from the same boot share it.
- **Plaso JSONL with a UTF-8 BOM**: files starting with a byte order mark now validate and import. Previously they failed validation with "first line is not a JSON object".
- Plaso JSONL files found during recursive import are listed as skipped; recursive import continues to cover EZ Tools and Unified Log output only.

### Security

- **Advanced search rejects semicolons and SQL comments**: an advanced search WHERE clause containing an unquoted semicolon, `--`, or `/*` is now rejected with an error before it runs, on both SQLite and PostgreSQL. Semicolons and comment markers inside quoted string values are still allowed. This applies to the grid, the result count, the histogram, and CSV export. **Behavior change:** clauses that previously ran with a trailing comment or semicolon must be rewritten without them.
- Settings, the logging configuration, log files, and exported CSV files are now written readable only by the current user (0600).
- PostgreSQL connection strings in log lines and the UI header show `user@host` with no password or password placeholder.
- Pivot (base query) values used by advanced-mode export are validated against the known field list and the `=` and `LIKE` operators.

### Bug fixes

- **Export CSV in advanced search mode** now exports exactly the rows shown in the grid, including the tab's base query and examiner notes handling. Previously the export ran the WHERE clause as a keyword search, producing a different row set.
- **Examiner notes in advanced search**: clauses that combine source conditions are resolved in a fixed order, so `source = 'FILE' AND source != 'REGISTRY'` no longer includes examiner notes. **Behavior change:** source filters using `IN` or `LIKE` (for example `source IN ('EXAMINER','FILE')`) now exclude examiner notes; previously they included them. Use `source = 'EXAMINER'` to include notes explicitly.

### Internal

- New import format registry (`internal/importer`): single-file and recursive import share one ordered list of formats, each with detection, import, extension, recursive-eligibility, and tool-family settings. Recursive import moved from `internal/eztoolparser` to `internal/importer`.
- New `internal/unifiedlogparser` and `internal/textutil` (shared UTF-8 BOM-stripping reader) packages.
- Advanced search grid and export share one query builder, so they cannot drift.
- First app-level integration tests (`app_test.go`) covering advanced search validation and export parity on SQLite.
- CI pins the Wails CLI to v2.12.0.

## v0.14.1 (2026-06-15)

### Security

- PostgreSQL connection string no longer exposes the password in the UI header or log file. `PostgresStore.Path()` now returns a sanitized string with the password omitted (e.g., `postgres://user@host:port/dbname?sslmode=MODE`).

## v0.14.0 (2026-05-24)

### New parsers

- **WxTCmd** (Windows 10 Timeline): Activity output with multi-timestamp expansion (StartTime, EndTime, LastModifiedTime, LastModifiedOnClient, OriginalLastModifiedOnClient). PackageIDs output is recognized and skipped with an explanatory message (reference data, no timeline events).
- **RBCmd** (Recycle Bin): $I/$R records with DeletedOn timestamp.
- **AppCompatCacheParser** (ShimCache): LastModifiedTimeUTC timestamp; rows with empty or NA timestamps skipped silently.
- **SrumECmd AppResourceUseInfo**: seventh SrumECmd subtype, following the same base mapping pattern as existing SRUM types.
- **AmcacheParser DeviceContainers**: KeyLastWriteTimestamp.
- **AmcacheParser DevicePnps**: KeyLastWriteTimestamp and DriverVerDate (multi-expand).
- **AmcacheParser DriveBinaries**: KeyLastWriteTimestamp, DriverTimeStamp, and DriverLastWriteTime (multi-expand).
- **AmcacheParser DriverPackages**: KeyLastWriteTimestamp and Date (multi-expand).
- **AmcacheParser ShortCuts**: KeyLastWriteTimestamp.

### Restored parsers (regression fix)

- **AmcacheParser AssociatedFileEntries**: accidentally dropped in a prior pass; restored with filename-based detection to distinguish from UnassociatedFileEntries.
- **AmcacheParser ProgramEntries**: accidentally dropped; restored with up to five timestamp expansions (KeyLastWriteTimestamp, InstallDate, InstallDateArpLastModified, InstallDateMsi, InstallDateFromLinkFile).
- **MFTECmd $MFT** and **MFTECmd $J**: collapsed into a single constant in a prior pass, causing zero events to be imported; restored as separate constants with correct timestamp columns.

### Improvements

- **No-timestamp format recognition**: MFTECmd $Boot and MFTECmd $SDS are now recognized during recursive import and reported in the skipped-files list with the reason "no timestamp columns: no timeline data to import" rather than appearing as unknown formats. This mechanism is general and will apply to any future recognized-but-untimed formats.
- **Source field normalization**: all imported events now have their Source field stored in uppercase regardless of how individual parsers set it (AMCACHE, FILESYSTEM, REGISTRY, WINDOWSTIMELINE, SRUM, etc.).

## [0.13.0] - 2026-05-14

### Features

- Recursive folder import for KAPE and EZ Tools output. Replaces the flat "Import EZ Tools Folder" workflow. Walks the selected folder up to 3 levels deep, detects supported CSV formats per file, and presents a post-import summary dialog with per-tool counts and a collapsible list of skipped files. Help dialog documents the depth limit and supported tools.

### Bug fixes

- Histogram click and drag-select now correctly update the From/To date range fields in the filter panel.
- Histogram fetch now respects the keyword search box and the bookmark-only toggle, in addition to date range and filter panel filters. Grid and histogram cannot drift out of sync.
- Histogram now refetches correctly when switching between tabs. Filtered tabs no longer show stale data from a previously-active tab.
- Advanced SQL search mode now correctly filters the histogram on both SQLite and PostgreSQL.
- Examiner notes now respect the bookmark-only filter and the date range filter. Previously, examiner notes outside the filtered date range or that were not bookmarked would still appear in the grid and histogram.
- PostgreSQL connection now correctly handles passwords containing special characters (@, #, %, :, /, and others) via percent-encoding. Fixes the Connect, Create & Connect, and Push to PostgreSQL paths.

### Internal

- Per-tab UI state consolidated into a single savedState object on each tab, replacing scattered individual fields. New applyTabState helper centralizes state restoration. New buildTabSessionJSON helper centralizes session save construction. New liveTabStateRef captures in-progress state for close-time saves.
- Separated tabSwitchingRef (grid concerns) and histogramSuppressRef (histogram concerns) to prevent the histogram from being permanently silenced by ref state held during database open or session restore.
- New NotesFilter struct on the Store interface allows the examiner notes side of UNION queries to receive a field-mapped subset of the active filter set. ExecuteQuery and ExecuteCountQuery now accept an optional *NotesFilter parameter.
- GitHub Actions release workflow now generates and uploads SHA256SUMS.txt alongside binary release artifacts.

## [0.12.0] - 2026-04-28

### Added

- Tab system: right-click any event to open a filtered view in a new tab, keeping the original view intact
- Context menu with "Search in new tab" for 10 event fields (filename, host, user, source, sourcetype, desc, URL, computer_name, event_identifier, source_name)
- Scoped filter dropdowns: each tab's filter options are populated from that tab's filtered results, not the entire database
- Save tab queries to the saved queries list for reuse; saved tab queries open in a new tab when loaded
- Tab session persistence: open tabs are saved per-database and restored when reopening (with prompt or auto-restore option)
- Close confirmation dialog when closing the app with a database open, ensuring tab sessions are saved
- Tab limit setting (default 5) configurable via Tools > Settings
- Auto-restore tabs setting in Tools > Settings
- Default PostgreSQL hostname setting in Tools > Settings (pre-fills the connection dialog)
- Settings menu (Tools > Settings) for centralized application preferences
- Stale data indicator on tabs with refresh button when data is modified in another tab
- Tab bar with close buttons, active tab highlighting, and theme-aware styling

### Fixed

- Examiner notes no longer appear in filtered views when filtering on fields they don't have (host, filename, sourcetype, etc.)
- Filter panel date range correctly scoped per tab
- Histogram month-end date calculation now handles all months correctly (previously hardcoded day 31)
- SQLite busy timeout set to 5 seconds to prevent lock contention errors during concurrent operations

## [0.11.1] - 2026-04-27

### Fixed

- macOS release archive now preserves the .app bundle structure (previously extracted to just Contents/ without the .app wrapper)
- macOS release binary now has the executable bit set (previously lost during GitHub Actions artifact transfer)

## [0.11.0] - 2026-03-29

### Added

- EZ Tools CSV import: auto-detect and import CSV output from Eric Zimmerman's forensic tools (EvtxECmd, PECmd, LECmd, JLECmd, AmcacheParser, SrumECmd, MFTECmd, SBECmd)
- Multi-timestamp expansion: each timestamp column in an EZ Tool CSV becomes a separate timeline event with appropriate MACB notation
- Import EZ Tools Folder: batch import all CSV files from a tool output directory via welcome screen button or File menu
- Single EZ Tool CSV files are auto-detected when using the normal Import Timeline function
- Support for 19 EZ Tool CSV subtypes including 6 AmcacheParser variants and 6 SrumECmd variants
- In-app help documentation for EZ Tools import

## [0.10.2] - 2026-03-27

### Changed

- Updated Go from 1.25 to 1.26
- Updated all Go dependencies (modernc.org/sqlite 1.37.0 to 1.48.0, Wails 2.11.0 to 2.12.0, pgx 5.8.0 to 5.9.1, golang.org/x/crypto 0.33.0 to 0.49.0)
- Updated GitHub Actions workflow (checkout v6, setup-go v6, setup-node v6, upload-artifact v6, download-artifact v8)
- Updated frontend npm dependencies

## [0.10.1] - 2026-02-22

### Fixed

- Timeline histogram drag selection not updating the date range filter on the first use
- PostgreSQL error when using histogram drag selection due to partial date format (e.g., "2025-02") not being expanded to a full timestamp

## [0.10.0] - 2026-02-19

### Added

- Examiner notes: manually add timestamped investigation notes that appear in the main timeline grid alongside evidence events. Notes use source "EXAMINER", support color coding and bookmarking, and are immutable after creation (delete and re-enter to change). Stored in a separate examiner_notes table with negative IDs to distinguish from evidence events.
- Advanced search mode: toggle between simple keyword search and SQL WHERE clause mode. Supports full SQL syntax with field names, operators, AND/OR logic. Includes a help popup showing available fields and operators. Advanced queries can be saved and loaded from the saved queries panel.
- Bulk select and edit: shift-click or ctrl-click to select multiple grid rows. Apply color, tags, or bookmark status to all selected events at once. Examiner note tags are protected from bulk tag changes.
- Multi-import into existing SQLite databases: importing a timeline file when a SQLite database is already open appends to the existing database instead of creating a new one. Enables combining multiple evidence sources into a single investigation database.
- PostgreSQL reserved word auto-quoting in advanced search (desc, user, offset)

### Fixed

- Examiner notes no longer appear in advanced search results when filtering by a specific non-EXAMINER source value

## [0.9.0] - 2026-02-16

### Added

- PostgreSQL database support with connection dialog (host, port, database, username, password, SSL mode)
- Create schema on empty PostgreSQL databases ("Create & Connect")
- Import timeline files directly into PostgreSQL when connected
- Push SQLite data to PostgreSQL with progress reporting (toolbar button, visible when SQLite is open)
- Enhanced pagination controls: First, Last, Go-to-page input, "Page X of Y" display
- Logging system under Help menu with enable/disable, file location prompt, optional persistence between sessions
- Database abstraction layer (Store interface, Dialect system, factory pattern)

### Fixed

- Export CSV now respects bookmark-only filter
- Export CSV now respects search text filter

### Changed

- Internal refactoring: Store interface, SQL dialect abstraction, raw SQL removed from app.go
- Query builder generates dialect-aware SQL (placeholder style, column quoting, date functions)

## [0.8.1] - 2026-02-12

### Fixed

- Minor bug fixes and stability improvements

## [0.8.0] - 2026-02-10

### Added

- TLN and L2TTLN import support (pipe-delimited, auto-detect, MACB mapping, composite description parsing)
- Dynamic CSV import support (variable columns, 30+ field aliases, header-based mapping)
- Keyword search highlighting across all themes (grid and detail panel)
- Event bookmarking (star toggle in grid and detail panel, filter to show bookmarked only, stored in database)
- Format auto-detection for all import types (extension-based with fallback validation)
- Database migration for backward compatibility with pre-0.8.0 databases
- Saved queries stored per-database
- Column visibility toggle (show/hide any of the 24+ columns)
- Export filtered results to CSV
- 11 UI themes (Forensic Dark, Classic Dark, High Contrast, Light, Solarized, Monokai, Dracula, Nord, Gruvbox, Matrix, Forensic Blue)
- Built-in user guide (Help > User Guide or F1)
- Native desktop menus with keyboard shortcuts
- Multi-platform builds via GitHub Actions (Linux, Windows, macOS)
- MIT License

## [0.7.0] - 2026-02-06

### Added

- Go/Wails rewrite of the original Python 4n6time application
- SQLite database backend (pure Go, no CGo dependencies)
- L2T CSV import with server-side pagination (1,000 events per page)
- Plaso JSONL import (psort json_line and raw Plaso storage formats)
- Raw Plaso storage format support (auto-detect, 70+ data_type mappings, multiple timestamp conversions)
- Full-text search across 14 event fields
- Filter panel with AND/OR logic, date range, and multi-field filters
- Timeline histogram with click-to-filter and drag-to-select range
- Resizable event detail panel with editable tags, colors, and notes
- Color-coded rows for marking events of interest
- About dialog
- Edit menu clipboard support (Cut/Copy/Paste/Select All)
