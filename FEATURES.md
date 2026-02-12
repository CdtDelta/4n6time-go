# 4n6time-go Feature Tracker

Last updated: 2026-02-11

## Planned / Discussed

| # | Feature | Notes |
|---|---------|-------|
| 1 | Detachable event detail window | Waiting on Wails v3 for native multi-window support. |
| 2 | TLN / L2TTLN import support | Pipe-delimited, 5 or 7 fixed fields. Trivial to implement. |
| 3 | Dynamic CSV import support | Variable column headers from Plaso dynamic output. |
| 4 | XLSX import support | Requires a Go library for reading Excel files. |
| 5 | Multi-database backend support (MySQL, PostgreSQL) | Abstract the database layer behind an interface. SQLite stays as default for portability. Addresses SQLite file size limitations for very large datasets. |
| 6 | Keyboard shortcuts reference | Add more shortcuts first (Ctrl+F search focus, Escape close panels, etc.), then document in Help dialog. |

## Potential Additions

| # | Feature | Notes |
|---|---------|-------|
| 7 | Keyword highlighting in grid/detail panel | Highlight search terms in visible results. |
| 8 | Bookmarking specific events | Beyond color coding, a dedicated bookmark/flag system. |
| 9 | Report generation | Export tagged/colored events as a formatted summary. |
| 10 | Statistics panel | Event counts by source, type, host, time distribution. |
| 11 | Multi-database support (compare timelines) | Open multiple timelines side by side. |
| 12 | Undo/redo for edits | Undo changes to tags, notes, colors. |
| 13 | Bulk operations | Color/tag multiple selected rows at once. |

## Completed

| Version | Feature |
|---------|---------|
| v0.1.0 | Phase 1 data layer (event model, database manager, query builder, CSV parser, 84 tests) |
| v0.2.0 | Import progress modal, filter panel, resizable event detail panel, color coding, saved queries, native menus, centralized versioning |
| v0.4.0 | Column visibility toggle, CSV export, timeline histogram with click/drag filtering, GitHub CI/CD for Linux/Windows/macOS |
| v0.5.0 | 11 UI themes (Forensic Dark, Classic Dark, High Contrast, Light, Solarized, Monokai, Dracula, Nord, Gruvbox, Matrix, Forensic Blue) |
| v0.6.0 | Plaso JSONL import support, updated file dialogs and menu labels |
| v0.6.1 | Timeline filter sync with date range inputs, README updates, David Nides acknowledgment |
| v0.7.0 | Raw Plaso storage format support (auto-detect, 70+ data_type mappings, Filetime/Posix/WebKit/Cocoa/Java/FAT timestamp conversion) |
| v0.7.0 | Search bar (full-text search across 14 columns), theme accessible from welcome screen via View menu |
| v0.7.0 | About dialog, built-in User Guide (Help > User Guide or F1) |
| v0.7.0 | Edit menu clipboard support (Cut/Copy/Paste/Select All working in webview) |
| v0.7.0 | Light theme filter dropdown fix (color-scheme property) |
