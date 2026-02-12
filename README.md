# 4n6time-go

Forensic timeline analysis tool, rewritten from Python to Go. Desktop application for analyzing large-scale forensic datasets, particularly timeline data from log2timeline (L2T) format files.

## Features

- Import L2T CSV and Plaso JSONL files (tested with 2GB+ files, millions of events)
- Server-side pagination for fast navigation of large datasets
- Filter panel with AND/OR logic, date range, and multi-field filters
- Timeline histogram with click-to-filter and drag-to-select range
- Resizable event detail panel with editable tags, colors, and notes
- Color-coded rows for marking events of interest
- Saved queries (stored in the database file)
- Column visibility toggle (show/hide any of the 24+ columns)
- Export filtered results to CSV
- 11 UI themes (Forensic Dark, Classic Dark, High Contrast, Light, Solarized, Monokai, Dracula, Nord, Gruvbox, Matrix, Forensic Blue)
- Native desktop menus with keyboard shortcuts
- Multi-platform builds via GitHub Actions (Linux, Windows, macOS)

## Tech Stack

- **Backend:** Go, SQLite (modernc.org/sqlite, pure Go)
- **Frontend:** React, AG Grid, Recharts
- **Framework:** Wails v2 (native desktop, no Electron)

## Building

### Prerequisites

- Go 1.23+
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
2. Click **Import** to import an L2T CSV or Plaso JSONL file, or **Open** to load an existing database
3. Use the **Filters** panel to narrow results by source, host, type, user, or date range
4. Click **Timeline** to visualize event distribution over time
5. Click any row to view full event details and add tags/notes/colors
6. Use **Saved Queries** to store and recall frequently used filter sets
7. Use **Columns** to show or hide fields in the grid
8. Use **Export CSV** to save filtered results
9. Change the UI theme via **View > Theme** (Ctrl+T)

## Acknowledgments

Special thanks to David Nides for creating the original 4n6time application, which served as the inspiration for this project.

## License

TBD
