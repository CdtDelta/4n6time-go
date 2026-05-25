<p align="center">
  <img src="4n6time-go.png" alt="4n6time-go">
</p>

# 4n6time-go

Forensic timeline analysis tool, rewritten from Python to Go. Desktop application for analyzing large-scale forensic datasets, particularly timeline data from log2timeline (L2T) format files.

## Features

- Import L2T CSV, Plaso JSONL, TLN, L2TTLN, dynamic CSV, and **EZ Tools CSV** files (tested with 2GB+ files, millions of events)
- **EZ Tools CSV import**: auto-detect and import output from Eric Zimmerman's tools with multi-timestamp expansion (EvtxECmd, PECmd, LECmd, JLECmd, SBECmd, MFTECmd, AmcacheParser, SrumECmd, RBCmd, WxTCmd, AppCompatCacheParser; 28 subtypes total)
- **Import EZ Tools Folder**: batch import all CSVs from a tool output directory
- **Tab system**: right-click any event to open a filtered view in a new tab; each tab has independent filters, search, and pagination; tab sessions persist per-database and are restored on next open
- **SQLite and PostgreSQL** database backends (SQLite for local work, PostgreSQL for team/server deployments)
- **Examiner notes**: add timestamped investigation notes directly into the timeline grid alongside evidence events
- **Advanced search**: toggle between keyword search and SQL WHERE clause mode with full query syntax
- **Bulk select and edit**: shift-click or ctrl-click to select multiple rows, then apply color, tags, or bookmarks to all at once
- **Multi-import**: import additional timeline files into an already-open database to combine evidence sources
- **Settings** (Tools > Settings): configure tab limit, auto-restore, and default PostgreSQL hostname
- Server-side pagination with First, Last, Go-to-page, and "Page X of Y" controls
- Full-text search across all key event fields with keyword highlighting
- Filter panel with AND/OR logic, date range, and multi-field filters
- Timeline histogram with click-to-filter and drag-to-select range
- Resizable event detail panel with editable tags, colors, and notes
- Bookmark events with star toggle (filter to show bookmarked only)
- Color-coded rows for marking events of interest
- Push SQLite data to a PostgreSQL server for sharing with a team
- Saved queries (stored in the database file)
- Column visibility toggle (show/hide any of the 24+ columns)
- Export filtered results to CSV
- 11 UI themes (Forensic Dark, Classic Dark, High Contrast, Light, Solarized, Monokai, Dracula, Nord, Gruvbox, Matrix, Forensic Blue)
- Built-in logging system for troubleshooting (Help > Logging)
- Built-in user guide (no internet required)
- Native desktop menus with keyboard shortcuts
- Multi-platform builds via GitHub Actions (Linux, Windows, macOS)

## Screenshots

![Main View](screenshots/main-view.png)

![Filter Panel](screenshots/filter-panel.png)

![Event Detail](screenshots/event-detail.png)

![Timeline Histogram](screenshots/timeline-histogram.png)

![Advanced Search](screenshots/advanced-search.png)

![Examiner Note](screenshots/examiner-note.png)

![Add Note Dialog](screenshots/add-note-dialog.png)

![Bulk Select](screenshots/bulk-select.png)

![PostgreSQL Connect](screenshots/postgres-connect.png)

![Pagination](screenshots/pagination.png)

![Logging Dialog](screenshots/logging-dialog.png)

![Themes](screenshots/themes.png)

## Tech Stack

- **Backend:** Go, SQLite (modernc.org/sqlite, pure Go), PostgreSQL (pgx)
- **Frontend:** React, AG Grid, Recharts
- **Framework:** Wails v2 (native desktop, no Electron)

## Building

### Prerequisites

- Go 1.26+
- Node.js 22 LTS
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Linux: `libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config`
- Windows: WebView2 runtime (included in Windows 10/11)
- macOS: No additional dependencies

### Build

```bash
cd frontend && npm install && npm run build && cd ..
wails build -tags webkit2_41   # Linux
wails build                     # Windows / macOS
```

The binary is output to `build/bin/`.

### Development (Docker)

```bash
docker compose up -d
docker compose exec dev bash
cd /workspace/frontend && npm run build
cd /workspace && wails build -tags webkit2_41
```

Run the binary on the host: `~/source/4n6time-go/build/bin/4n6time`

## Usage

1. Launch the application
2. Click **Import Timeline** to import a timeline file (L2T CSV, JSONL, TLN, L2TTLN, dynamic CSV, or EZ Tools CSV), or **Open** to load an existing database
3. Use the **Filters** panel to narrow results by source, host, type, user, or date range
4. Click **Timeline** to visualize event distribution over time
5. Click any row to view full event details and add tags/notes/colors
6. Use **Saved Queries** to store and recall frequently used filter sets
7. Use **Columns** to show or hide fields in the grid
8. Use **Export CSV** to save filtered results
9. Change the UI theme via **View > Theme** (Ctrl+T)

### Tab System

Right-click any row in the event grid to open a context menu with "Search in new tab" options for key fields (host, user, source, filename, and more). Each tab maintains its own independent filters, search, and pagination — the original tab is unchanged.

- Tabs can be saved to the Saved Queries list using the save icon on the tab; loading a saved tab query reopens it in a new tab
- When data is modified in one tab (color change, bookmark, etc.), other tabs show a stale indicator dot and can be refreshed with the refresh button
- Tab sessions are saved when you close the database or application; on next open you are prompted to restore them (or they restore automatically if auto-restore is enabled)
- Configure the tab limit (default 5) and auto-restore behavior via **Tools > Settings**

### Examiner Notes

Add timestamped investigation notes directly into the timeline alongside evidence events. Click the **+** button in the toolbar to open the Add Note dialog. Enter a date/time (or click "Now"), a description, and an optional tag. Notes appear in the grid with source "EXAMINER" and can be color-coded and bookmarked. Notes are immutable after creation; to change one, delete it from the detail panel and re-enter it.

### Advanced Search

Toggle between simple keyword search and SQL WHERE clause mode using the **Aa/SQL** button next to the search bar. In SQL mode, enter any valid WHERE clause using field names and SQL operators:

```
source = 'FILE'
desc LIKE '%malware%' AND host = 'WORKSTATION1'
datetime BETWEEN '2025-01-01' AND '2025-06-01'
```

Click the **?** button to see all available field names and operators. Advanced queries can be saved and loaded from the Saved Queries panel. On PostgreSQL, the reserved words `desc`, `user`, and `offset` are auto-quoted.

### Bulk Editing

Select multiple rows using shift-click (range) or ctrl/cmd-click (individual toggle). When multiple rows are selected, a bulk action bar appears with color swatches, a tag input, bookmark buttons, and an "Apply Changes" button. Select a color and/or enter a tag, then click Apply Changes to update all selected events at once. Examiner note tags are protected from bulk tag changes.

### Multi-Import

When a SQLite or PostgreSQL database is already open, importing a timeline file appends the data to the existing database instead of creating a new one. This lets you combine multiple evidence sources (e.g., multiple hard drive images) into a single investigation database.

### EZ Tools Import

4n6time can import CSV output from Eric Zimmerman's forensic artifact parsers, auto-detecting the tool from column headers and expanding multiple timestamp columns into individual timeline events with appropriate MACB notation.

**Single file:** Use the **Import Timeline** button or File > Import Timeline. EZ Tool CSVs are auto-detected alongside other supported formats.

**Directory:** Use the **Import EZ Tools Folder** button on the welcome screen or File > Import EZ Tools Folder to batch import all CSV files from a tool output directory.

Supported tools: **EvtxECmd** (Windows Event Logs), **PECmd** (Prefetch), **LECmd** (LNK files), **JLECmd** (Jump Lists), **SBECmd** (ShellBags), **MFTECmd** ($MFT, $J; $Boot and $SDS recognized but skipped), **AmcacheParser** (AssociatedFileEntries, UnassociatedFileEntries, ProgramEntries, DeviceContainers, DevicePnps, DriveBinaries, DriverPackages, ShortCuts), **SrumECmd** (AppResourceUseInfo, AppTimeline, EnergyUsage, NetworkConnections, NetworkUsages, PushNotifications, vfuprov), **RBCmd** (Recycle Bin), **WxTCmd** (Activity; PackageIDs recognized but skipped), **AppCompatCacheParser** (ShimCache).

### PostgreSQL Support

4n6time can connect to a PostgreSQL server as an alternative to local SQLite databases:

1. Click **PostgreSQL** on the welcome screen to open the connection dialog
2. Enter connection details: host, port, database name, username, password, and SSL mode
3. Click **Connect** to connect to an existing database, or **Create & Connect** to create the schema on an empty database
4. When connected to PostgreSQL, importing a timeline file writes directly to the server (no local file needed)
5. To push an existing SQLite database to PostgreSQL, open the SQLite database first, then click the **Push to PostgreSQL** button in the toolbar

## Acknowledgments

Special thanks to David Nides for creating the original 4n6time application, which served as the inspiration for this project.

## License

MIT License. See [LICENSE](LICENSE) for details.
