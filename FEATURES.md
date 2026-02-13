# 4n6time-go Feature Tracker

Last updated: 2026-02-12

## Planned / Discussed

| # | Feature | Notes |
|---|---------|-------|
| 1 | XLSX import support | Requires a Go library for reading Excel files. |
| 2 | Multi-database backend support (MySQL, PostgreSQL) | Abstract the database layer behind an interface. SQLite stays as default for portability. Addresses SQLite file size limitations for very large datasets. |
| 3 | Keyboard shortcuts reference | Add more shortcuts first (Ctrl+F search focus, Escape close panels, etc.), then document in Help dialog. |
| 4 | Report generation | Export tagged/colored events as a formatted summary. |
| 5 | Statistics panel | Event counts by source, type, host, time distribution. |
| 6 | Multi-database comparison (compare timelines) | Open multiple timelines side by side. |
| 7 | Undo/redo for edits | Undo changes to tags, notes, colors. |
| 8 | Bulk operations | Color/tag multiple selected rows at once. |

## Future (Blocked)

| # | Feature | Notes |
|---|---------|-------|
| 1 | Detachable event detail window | Waiting on Wails v3 for native multi-window support. |

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
| v0.8.0 | MIT License |
| v0.8.0 | TLN / L2TTLN import support (pipe-delimited, auto-detect, MACB mapping, composite description parsing) |
| v0.8.0 | Dynamic CSV import support (variable columns, 30+ field aliases, header-based mapping) |
| v0.8.0 | Keyword search highlighting across all themes (grid and detail panel) |
| v0.8.0 | Event bookmarking (star toggle in grid and detail panel, filter to show bookmarked only, stored in database) |
| v0.8.0 | Format auto-detection for all import types (extension-based with fallback validation) |
| v0.8.0 | Database migration for backward compatibility with pre-0.8.0 databases |
