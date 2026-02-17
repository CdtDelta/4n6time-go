# Changelog

All notable changes to 4n6time-go are documented in this file.

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
