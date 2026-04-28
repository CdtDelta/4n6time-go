# Changelog

All notable changes to 4n6time-go are documented in this file.

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
