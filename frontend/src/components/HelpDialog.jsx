import { useState } from 'react'

const helpSections = [
  {
    id: 'getting-started',
    title: 'Getting Started',
    content: `4n6time is a forensic timeline analysis tool that helps investigators view, filter, and analyze digital forensic timelines.

To begin, you can either open an existing database, import a timeline file, or connect to a PostgreSQL server:

Open Database: Opens a previously created SQLite database file (.db). Use File > Open Database or the Open button in the toolbar.

Import Timeline: Imports a timeline file and creates a new SQLite database. Use File > Import Timeline or the Import button in the toolbar. You will be prompted to choose or create a database file, then select the timeline file to import. If you are already connected to a PostgreSQL database, importing writes directly to the server without prompting for a local file.

PostgreSQL: Connects to a PostgreSQL server. Click the PostgreSQL button on the welcome screen. See the PostgreSQL Support section for details.

Supported import formats:
- CSV: Standard log2timeline/Plaso CSV output (comma-delimited with standard forensic timeline columns)
- JSONL: Plaso JSON Lines output (both psort json_line format and raw Plaso storage format are supported)
- TLN: 5-field pipe-delimited timeline format
- L2TTLN: 7-field pipe-delimited extended timeline format
- Dynamic CSV: Plaso default output with variable columns defined by header row

After import, events are stored in the database (SQLite or PostgreSQL) for fast querying and can be reopened at any time without reimporting.`
  },
  {
    id: 'filtering',
    title: 'Filtering Events',
    content: `The filter panel lets you narrow down events based on field values and date ranges. Click the Filters button in the toolbar to open it.

Adding Filters:
Click "Add Filter" to create a new filter row. Each filter has three parts: a field name (e.g., source, sourcetype, desc), an operator (equals, not equals, contains, not contains), and a value. For equals/not equals, a dropdown shows all distinct values in that field. For contains/not contains, type any text.

Filter Logic:
Use the AND/OR toggle to control how multiple filters combine. AND means all filters must match. OR means any filter can match.

Date Range:
Set a start and end date to limit results to a specific time window. The date range works alongside other filters using AND logic.

Applying and Clearing:
Click Apply to run the filters. Click Clear to remove all filters and show all events. The status bar shows how many filters are active.`
  },
  {
    id: 'searching',
    title: 'Searching',
    content: `The search bar in the toolbar performs a text search across multiple event fields simultaneously.

Type your search term and press Enter or click the Search button. The search looks for matches (case-insensitive, partial match) in these fields: description, filename, source, source type, type, user, host, extra, tag, URL, source name, computer name, format, and notes.

Matching text is highlighted in the grid and detail panel. Highlight colors adapt to the active theme for optimal readability.

Search works alongside filters. If you have active filters and perform a search, both conditions must be satisfied (AND logic). The status bar shows the active search term.

To clear the search, click the "x" button next to the search input.`
  },
  {
    id: 'advanced-search',
    title: 'Advanced Search',
    content: `Advanced search mode lets you write SQL WHERE clauses directly for precise queries. Toggle between simple keyword search and SQL mode using the Aa/SQL button next to the search bar.

Toggling modes: Click the Aa/SQL button to switch. In simple mode (Aa), the search bar performs keyword matching across multiple fields. In SQL mode (SQL), the search bar accepts a raw WHERE clause.

SQL syntax: Write any valid SQL WHERE clause using the field names from the database. Examples:

source = 'FILE'
desc LIKE '%malware%' AND host = 'WORKSTATION1'
datetime BETWEEN '2025-01-01' AND '2025-06-01'
source = 'WEBHIST' OR source = 'OLECF'
tag LIKE '%important%'

Field reference: Click the ? button next to the search bar to see all 30 available field names and supported operators (=, !=, LIKE, NOT LIKE, >, <, >=, <=, AND, OR, BETWEEN, IN).

PostgreSQL note: When connected to a PostgreSQL database, the columns desc, user, and offset are SQL reserved words. These are automatically double-quoted before execution, so you can use them as-is in your queries.

Saving advanced queries: Click the Save button (floppy disk icon) to save the current SQL query with a name. Saved advanced queries appear in the Saved Queries panel with a "SQL:" prefix. Loading a saved advanced query automatically switches to SQL mode.

Advanced search works alongside the bookmark-only filter. If both are active, only bookmarked events matching the WHERE clause are returned.`
  },
  {
    id: 'bookmarks',
    title: 'Bookmarks',
    content: `Bookmark events to flag them for later review. Bookmarks are stored in the database and persist between sessions.

Click the star icon in the leftmost grid column to toggle a bookmark on any event. You can also toggle bookmarks from the event detail panel using the star button next to "Event Detail."

To view only bookmarked events, click the star button in the toolbar (next to the search bar). The status bar shows when the bookmark filter is active. Click the star button again to show all events.

Bookmarks work alongside filters and search. When the bookmark filter is active, only bookmarked events matching your other criteria are shown.`
  },
  {
    id: 'timeline',
    title: 'Timeline Histogram',
    content: `The timeline histogram provides a visual overview of event distribution over time. Click the Timeline button in the toolbar to show or hide it.

The histogram automatically adjusts its time buckets based on the date range of your data: monthly buckets for multi-year spans, daily for within a single year, and hourly for a single day.

Clicking a bar in the histogram sets the date range filter to that time period and opens the filter panel. This lets you quickly drill into activity spikes. The histogram also respects active filters and search terms, so you can see the time distribution of your filtered results.

Hovering over a bar shows the time period and event count.`
  },
  {
    id: 'saved-queries',
    title: 'Saved Queries',
    content: `Saved queries let you store and recall filter combinations for repeated use. Click the Saved Queries button in the toolbar to open the panel.

To save a query: Set up your desired filters, then open the Saved Queries panel and type a name for the query. Click Save. The current filter configuration (including date range and logic) is stored.

To load a query: Click on a saved query name to apply its filters immediately.

To delete a query: Click the delete button next to the query name.

Saved queries are stored per-database, so each database has its own set of saved queries.`
  },
  {
    id: 'columns',
    title: 'Column Chooser',
    content: `The column chooser lets you control which columns are visible in the event grid. Click the Columns button in the toolbar to open it.

Toggle columns on or off by clicking them. Some columns are hidden by default to reduce clutter (ID, Filename, Inode, Notes, Format, Extra, Color, URL, Record Number, Event ID, Event Type, Source Name, User SID, Computer). You can show any of these by toggling them in the column chooser.

Columns can also be resized by dragging the column header borders, and reordered by dragging column headers.`
  },
  {
    id: 'event-detail',
    title: 'Event Details',
    content: `Click any row in the event grid to see its full details in the detail panel below the grid.

The detail panel shows all fields for the selected event, including fields not visible in the grid. You can resize the detail panel by dragging the resize handle between the grid and the detail panel.

Notes and Report: Each event has Notes and Report Notes fields you can edit. You can also mark events as "In Report" and assign a color tag for visual highlighting. Click Save after making changes.

Color-tagged events appear with a colored left border and subtle background tint in the grid for easy identification.`
  },
  {
    id: 'examiner-notes',
    title: 'Examiner Notes',
    content: `Examiner notes let you add your own timestamped investigation notes directly into the timeline grid alongside evidence events.

Adding a note: Click the + button in the toolbar to open the Add Note dialog. Enter a date and time for the note (click "Now" to use the current time), a description, and an optional tag. The tag "examiner_entry" is always included automatically. Click Add Note to create it.

How notes appear: Examiner notes appear in the main event grid with source "EXAMINER" and source type "Examiner Note". They are interleaved with evidence events by datetime, so they appear in chronological context. Notes use negative IDs internally to distinguish them from evidence events.

Editing notes: Examiner notes are immutable after creation. You can change the color and toggle the bookmark, but you cannot edit the description, tag, or datetime. To correct a note, delete it and create a new one.

Deleting notes: Select an examiner note in the grid to open it in the detail panel. The detail panel shows a "Delete Note" button for examiner notes. Click it to permanently remove the note.

Color coding and bookmarks: You can assign a color to an examiner note using the color picker in the detail panel, and toggle its bookmark star. Colors and bookmarks work the same as for evidence events.

Filtering: Use the source filter with value "EXAMINER" to show only examiner notes, or use advanced search with source = 'EXAMINER'. When filtering by a different source (e.g., source = 'FILE'), examiner notes are automatically excluded from results.`
  },
  {
    id: 'bulk-editing',
    title: 'Bulk Editing',
    content: `Bulk editing lets you apply changes to multiple events at once.

Selecting multiple rows: Hold Shift and click to select a range of rows. Hold Ctrl (Cmd on macOS) and click to toggle individual rows. Selected rows are highlighted in the grid.

Bulk action bar: When more than one row is selected, the detail panel is replaced by a bulk action bar. The bar shows the number of selected events and provides these controls:

Color swatches: Click a color swatch to stage that color for application. The selected swatch shows a blue outline. Click the "None" swatch (dashed border) to stage clearing the color.

Tag input: Type a tag name to stage it for application. Tags are appended to existing tags on each event, avoiding duplicates.

Apply Changes: Click this button to apply the staged color and/or tag to all selected events. The button is disabled until you select a color or enter a tag. Both can be applied in a single action.

Bookmark All / Unbookmark All: Set or clear the bookmark flag on all selected events immediately (no need to click Apply Changes).

Clear Selection: Deselects all rows and returns to the normal detail panel view.

Examiner note protection: When bulk editing a mixed selection of evidence events and examiner notes, tag changes only apply to evidence events. Examiner note tags are immutable and are skipped during bulk tag operations. Color and bookmark changes apply to both.`
  },
  {
    id: 'multi-import',
    title: 'Multi-Import',
    content: `When a database is already open, importing a timeline file adds the new data to the existing database instead of creating a new one.

How it works: If you have a SQLite database open and click Import (or File > Import Timeline), the file chooser opens directly without prompting for a new database location. The imported events are added to the existing database alongside any events already there. The grid refreshes to show the combined data.

Use case: This is useful for building a single investigation database from multiple evidence sources. For example, you might import a filesystem timeline first, then import a browser history timeline, then a Windows event log timeline, all into the same database. Each import adds to the existing data.

PostgreSQL: The same behavior applies when connected to a PostgreSQL database. Importing always writes directly to the connected server.

Note: There is no undo for an import. If you import the wrong file, you would need to start with a fresh database. Consider keeping backups of your database file before importing additional sources.`
  },
  {
    id: 'export',
    title: 'Exporting Data',
    content: `Export your current view to a CSV file using File > Export CSV or the Export CSV button in the toolbar.

The export respects your current filters, search, date range, and bookmark-only filter. Only the events matching your current query are exported. This is useful for creating focused reports or sharing subsets of timeline data with other analysts.

You will be prompted to choose a filename and location for the exported CSV file.`
  },
  {
    id: 'themes',
    title: 'Themes',
    content: `4n6time includes multiple color themes to suit different preferences and working conditions. Access themes through View > Theme or Ctrl+T.

Available themes: Forensic Dark (default), Classic Dark, High Contrast, Light, Solarized Dark, Monokai, Dracula, Nord, Gruvbox, Matrix, and Forensic Blue.

High Contrast is designed for maximum readability. Light theme is available for well-lit environments. Your theme selection is saved and persists across sessions.`
  },
  {
    id: 'postgresql',
    title: 'PostgreSQL Support',
    content: `4n6time can use a PostgreSQL server as an alternative to local SQLite databases. This is useful for team environments, larger datasets, or when you want centralized storage.

Connecting: Click the PostgreSQL button on the welcome screen to open the connection dialog. Enter the host, port (default 5432), database name, username, and password. The SSL mode dropdown lets you choose between disable, require, verify-ca, and verify-full depending on your server configuration.

Connect vs Create & Connect: Use "Connect" when the database already has the 4n6time schema (the log2timeline table and associated metadata tables). Use "Create & Connect" when connecting to an empty database for the first time. This creates all the required tables and indexes, then connects.

Importing into PostgreSQL: When you are connected to a PostgreSQL database, clicking Import (or File > Import Timeline) imports the timeline file directly into the server. The save dialog for choosing a local database file is skipped since events go straight to PostgreSQL.

Push to PostgreSQL: If you have a SQLite database open and want to copy its data to a PostgreSQL server, click the "Push to PostgreSQL" button in the toolbar. This opens the same connection dialog. After connecting, all events from the SQLite database are copied to the PostgreSQL server. Progress is reported in a dialog. The SQLite database remains open afterward so you can continue working locally.

Switching back: To return to working with local SQLite databases, close the current database (File > Close Database or Ctrl+W) and open or import as usual.`
  },
  {
    id: 'pagination',
    title: 'Pagination',
    content: `Events are displayed 1,000 per page with pagination controls in the toolbar.

Navigation buttons: First goes to page 1. Prev goes back one page. Next advances one page. Last goes to the final page. Buttons are disabled when you are already at the corresponding boundary.

Page input: The page number between Prev and Next is an editable field. Type a page number and press Enter to jump directly to that page. The value is validated to be between 1 and the total page count. If you type an invalid value and leave the field, it reverts to the current page number.

The total event count matching your current filters and search is displayed to the right of the pagination controls.`
  },
  {
    id: 'logging',
    title: 'Logging',
    content: `4n6time includes a logging system for troubleshooting. Access it from Help > Logging.

Enabling logging: Click "Enable Logging" in the Logging dialog. You will be prompted to choose a location and filename for the log file. Once enabled, the dialog shows the log file path and a green "Enabled" badge.

What gets logged: Application startup and shutdown, database open and close events, import operations (start, duration, event count), push to PostgreSQL operations, query errors, export CSV operations, PostgreSQL connection events, and logging enable/disable events. Each entry includes a timestamp and level (INFO or ERROR).

Disabling logging: Click "Disable Logging" to stop writing to the log file. The file is closed and can be reviewed with any text editor.

Persistence: Check "Resume logging on next launch" to have logging automatically restart when you open 4n6time. The log file is reopened in append mode so previous entries are preserved. Uncheck the option to stop automatic logging on future launches. This setting is stored in your system config directory.`
  },
  {
    id: 'data-formats',
    title: 'Data Formats',
    content: `4n6time supports the following import formats:

CSV (log2timeline): The standard CSV format produced by Plaso's psort tool with the l2tcsv output module. Has 17 fixed columns including date, time, timezone, MACB, source, sourcetype, type, user, host, short description, filename, and more.

JSONL (Plaso JSON Lines): Two sub-formats are supported. The psort json_line format contains fields like timestamp, datetime, source_short, source_long, and message. The raw Plaso storage format contains data_type (e.g., fs:stat, windows:evtx:record) and nested date_time objects. The parser auto-detects the format and maps data_type values to the appropriate Source and Source Type. Multiple timestamp formats are handled: Filetime, PosixTime, PosixTimeInMicroseconds, WebKitTime, CocoaTime, JavaTime, and FATDateTime.

TLN (Timeline): A simple 5-field pipe-delimited format with fields: Time (Unix epoch), Source, Host, User, Description. Generated by Plaso with "psort -o tln". The description field often contains a composite of "datetime; timestamp_desc; message" which is automatically parsed.

L2TTLN (Extended Timeline): A 7-field pipe-delimited extension of TLN adding Timezone and Notes fields. The Notes field typically contains filename and inode information which is automatically extracted. Generated by Plaso with "psort -o l2ttln".

Dynamic CSV: Plaso's default output format with variable columns defined by the header row. Default fields are datetime, timestamp_desc, source, source_long, message, parser, display_name, and tag. Custom fields added via Plaso's --fields and --additional_fields options are automatically mapped where possible, with unrecognized fields collected into the Extra column. Generated by Plaso with "psort -o dynamic".

MACB Notation: Timestamps are categorized using MACB notation where M = Modified, A = Accessed, C = Changed (metadata), B = Born (created). These map from the timestamp_desc field in Plaso output.

Format Auto-Detection: When importing, 4n6time automatically detects the file format based on the file extension and content structure. Files with .jsonl extension are treated as JSONL, .tln as TLN. For .csv and .txt files, the parser tries L2T CSV first (fixed 17 columns), then TLN (pipe-delimited), then dynamic CSV (header-based).`
  }
]

function HelpDialog({ visible, onClose }) {
  const [activeSection, setActiveSection] = useState('getting-started')

  if (!visible) return null

  const section = helpSections.find(s => s.id === activeSection)

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="help-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="help-header">
          <span className="help-title">User Guide</span>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="help-body">
          <div className="help-nav">
            {helpSections.map(s => (
              <div
                key={s.id}
                className={`help-nav-item ${activeSection === s.id ? 'active' : ''}`}
                onClick={() => setActiveSection(s.id)}
              >
                {s.title}
              </div>
            ))}
          </div>
          <div className="help-content">
            <h3>{section?.title}</h3>
            <div className="help-text">
              {section?.content.split('\n\n').map((para, i) => (
                <p key={i}>{para}</p>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default HelpDialog
