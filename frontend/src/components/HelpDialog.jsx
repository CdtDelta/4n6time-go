import { useState } from 'react'

const helpSections = [
  {
    id: 'getting-started',
    title: 'Getting Started',
    content: `4n6time is a forensic timeline analysis tool that helps investigators view, filter, and analyze digital forensic timelines.

To begin, you can either open an existing database or import a timeline file:

Open Database: Opens a previously created SQLite database file (.db). Use File > Open Database or the Open button in the toolbar.

Import Timeline: Imports a timeline file and creates a new SQLite database. Use File > Import Timeline or the Import button in the toolbar. You will be prompted to choose or create a database file, then select the timeline file to import.

Supported import formats:
- CSV: Standard log2timeline/Plaso CSV output (comma-delimited with standard forensic timeline columns)
- JSONL: Plaso JSON Lines output (both psort json_line format and raw Plaso storage format are supported)

After import, events are stored in a local SQLite database for fast querying and can be reopened at any time without reimporting.`
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
    id: 'export',
    title: 'Exporting Data',
    content: `Export your current view to a CSV file using File > Export CSV or the Export CSV button in the toolbar.

The export respects your current filters, search, and date range. Only the events matching your current query are exported. This is useful for creating focused reports or sharing subsets of timeline data with other analysts.

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
