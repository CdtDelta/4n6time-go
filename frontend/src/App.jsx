import { useState, useCallback, useEffect, useMemo, useRef } from 'react'
import { AgGridReact } from 'ag-grid-react'
import 'ag-grid-community/styles/ag-grid.css'
import 'ag-grid-community/styles/ag-theme-alpine.css'

import { OpenDatabase, ImportCSV, CloseDatabase, QueryEvents, ExportCSV, GetVersion } from '../wailsjs/go/main/App'
import ImportProgress from './components/ImportProgress'
import FilterPanel from './components/FilterPanel'
import EventDetail from './components/EventDetail'
import SavedQueries from './components/SavedQueries'
import ColumnChooser from './components/ColumnChooser'
import TimelineChart from './components/TimelineChart'
import ThemePicker from './components/ThemePicker'
import themes, { lightThemes } from './themes'

const PAGE_SIZE = 1000

// Column definitions for the forensic timeline grid
const defaultColDefs = [
  { field: 'id', headerName: 'ID', width: 70, hide: true },
  { field: 'datetime', headerName: 'Date/Time', width: 170, sort: 'asc' },
  { field: 'timezone', headerName: 'TZ', width: 60 },
  { field: 'macb', headerName: 'MACB', width: 70 },
  { field: 'source', headerName: 'Source', width: 80 },
  { field: 'sourcetype', headerName: 'Source Type', width: 140 },
  { field: 'type', headerName: 'Type', width: 120 },
  { field: 'user', headerName: 'User', width: 100 },
  { field: 'host', headerName: 'Host', width: 120 },
  { field: 'desc', headerName: 'Description', flex: 1, minWidth: 300 },
  { field: 'filename', headerName: 'Filename', width: 200, hide: true },
  { field: 'inode', headerName: 'Inode', width: 80, hide: true },
  { field: 'notes', headerName: 'Notes', width: 150, hide: true },
  { field: 'format', headerName: 'Format', width: 100, hide: true },
  { field: 'extra', headerName: 'Extra', width: 150, hide: true },
  { field: 'tag', headerName: 'Tag', width: 100 },
  { field: 'color', headerName: 'Color', width: 80, hide: true },
  { field: 'url', headerName: 'URL', width: 200, hide: true },
  { field: 'record_number', headerName: 'Record #', width: 90, hide: true },
  { field: 'event_identifier', headerName: 'Event ID', width: 90, hide: true },
  { field: 'event_type', headerName: 'Event Type', width: 100, hide: true },
  { field: 'source_name', headerName: 'Source Name', width: 120, hide: true },
  { field: 'user_sid', headerName: 'User SID', width: 120, hide: true },
  { field: 'computer_name', headerName: 'Computer', width: 120, hide: true },
]

function App() {
  const [dbInfo, setDbInfo] = useState(null)
  const [events, setEvents] = useState([])
  const [totalCount, setTotalCount] = useState(0)
  const [currentPage, setCurrentPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [status, setStatus] = useState('')
  const [importing, setImporting] = useState(false)
  const [showFilters, setShowFilters] = useState(false)
  const [showSavedQueries, setShowSavedQueries] = useState(false)
  const [showColumnChooser, setShowColumnChooser] = useState(false)
  const [showTimeline, setShowTimeline] = useState(false)
  const [showThemePicker, setShowThemePicker] = useState(false)
  const [currentTheme, setCurrentTheme] = useState(() => {
    try { return window.localStorage?.getItem('4n6time-theme') || 'forensic-dark' }
    catch { return 'forensic-dark' }
  })
  const [columnDefs, setColumnDefs] = useState(defaultColDefs)

  // Apply theme CSS variables to document root
  const applyTheme = useCallback((themeId) => {
    const theme = themes[themeId]
    if (!theme) return
    const root = document.documentElement
    Object.entries(theme.vars).forEach(([key, value]) => {
      root.style.setProperty(key, value)
    })
  }, [])

  // Apply theme on mount and when changed
  useEffect(() => {
    applyTheme(currentTheme)
  }, [currentTheme, applyTheme])

  const handleSelectTheme = useCallback((themeId) => {
    setCurrentTheme(themeId)
    try { window.localStorage?.setItem('4n6time-theme', themeId) }
    catch { /* ignore */ }
    setShowThemePicker(false)
  }, [])
  const [activeFilters, setActiveFilters] = useState(null)
  const [selectedEvent, setSelectedEvent] = useState(null)
  const [detailHeight, setDetailHeight] = useState(280)
  const [version, setVersion] = useState('')
  const gridRef = useRef(null)
  const resizingRef = useRef(false)

  // Load version on mount
  useEffect(() => {
    GetVersion().then(v => { if (v) setVersion(v) })
  }, [])

  const totalPages = Math.max(1, Math.ceil(totalCount / PAGE_SIZE))

  const defaultColDef = useMemo(() => ({
    sortable: true,
    resizable: true,
    filter: true,
  }), [])

  // Build the query request from current filters
  const buildQueryRequest = useCallback((page, filterState) => {
    const req = {
      filters: [],
      logic: 'AND',
      orderBy: 'datetime',
      page: page,
      pageSize: PAGE_SIZE,
    }

    const fs = filterState || activeFilters
    if (fs) {
      req.filters = fs.filters || []
      req.logic = fs.logic || 'AND'

      if (fs.dateFrom && fs.dateTo) {
        req.filters = [
          ...req.filters,
          { field: 'datetime', operator: '>=', value: fs.dateFrom },
          { field: 'datetime', operator: '<=', value: fs.dateTo },
        ]
      }
    }

    return req
  }, [activeFilters])

  const loadPage = useCallback(async (page, info, filterState) => {
    const db = info || dbInfo
    if (!db) return

    setLoading(true)
    setStatus('Loading events...')
    try {
      const req = buildQueryRequest(page, filterState)
      const result = await QueryEvents(req)

      if (result) {
        setEvents(result.events || [])
        setTotalCount(result.totalCount)
        setCurrentPage(result.page)

        const filterCount = (filterState || activeFilters)?.filters?.length || 0
        const filterLabel = filterCount > 0 ? ` (${filterCount} filter${filterCount > 1 ? 's' : ''} active)` : ''
        setStatus(`Showing ${result.events?.length || 0} of ${result.totalCount.toLocaleString()} events${filterLabel}`)
      }
    } catch (err) {
      setStatus('Error: ' + err)
    } finally {
      setLoading(false)
    }
  }, [dbInfo, buildQueryRequest, activeFilters])

  const handleOpenDB = useCallback(async () => {
    try {
      setStatus('Opening database...')
      const info = await OpenDatabase()
      if (info) {
        setDbInfo(info)
        setActiveFilters(null)
        setShowFilters(false)
        setSelectedEvent(null)
        setStatus(`Opened: ${info.path} (${info.eventCount.toLocaleString()} events)`)
        await loadPage(1, info, null)
      } else {
        setStatus('')
      }
    } catch (err) {
      setStatus('Error: ' + err)
    }
  }, [loadPage])

  const handleImportCSV = useCallback(async () => {
    try {
      setImporting(true)
      setStatus('Importing timeline...')
      const info = await ImportCSV()
      if (info) {
        setDbInfo(info)
        setActiveFilters(null)
        setShowFilters(false)
        setSelectedEvent(null)
        setStatus(`Imported: ${info.eventCount.toLocaleString()} events`)
        await loadPage(1, info, null)
      } else {
        setStatus('')
      }
    } catch (err) {
      setStatus('Error: ' + err)
    } finally {
      setImporting(false)
    }
  }, [loadPage])

  const handleCloseDB = useCallback(async () => {
    try {
      await CloseDatabase()
      setDbInfo(null)
      setEvents([])
      setTotalCount(0)
      setCurrentPage(1)
      setActiveFilters(null)
      setShowFilters(false)
      setSelectedEvent(null)
      setStatus('')
    } catch (err) {
      setStatus('Error: ' + err)
    }
  }, [])

  const handlePrevPage = useCallback(() => {
    if (currentPage > 1) loadPage(currentPage - 1)
  }, [currentPage, loadPage])

  const handleNextPage = useCallback(() => {
    if (currentPage < totalPages) loadPage(currentPage + 1)
  }, [currentPage, totalPages, loadPage])

  const handleApplyFilters = useCallback((filterState) => {
    setActiveFilters(filterState)
    setCurrentPage(1)
    setSelectedEvent(null)
    loadPage(1, null, filterState)
  }, [loadPage])

  const handleClearFilters = useCallback(() => {
    setActiveFilters(null)
    setCurrentPage(1)
    setSelectedEvent(null)
    loadPage(1, null, { filters: [], logic: 'AND', dateFrom: '', dateTo: '' })
  }, [loadPage])

  const toggleFilters = useCallback(() => {
    setShowFilters(prev => !prev)
  }, [])

  const toggleSavedQueries = useCallback(() => {
    setShowSavedQueries(prev => !prev)
  }, [])

  const toggleColumnChooser = useCallback(() => {
    setShowColumnChooser(prev => !prev)
  }, [])

  const handleToggleColumn = useCallback((field) => {
    setColumnDefs(prev => prev.map(col => {
      if (col.field === field) {
        return { ...col, hide: !col.hide }
      }
      return col
    }))
  }, [])

  const handleExportCSV = useCallback(async () => {
    try {
      setStatus('Exporting...')
      const req = buildQueryRequest(1, activeFilters)
      const result = await ExportCSV(req)
      if (result) {
        setStatus(result)
      } else {
        setStatus('')
      }
    } catch (err) {
      setStatus('Export error: ' + err)
    }
  }, [buildQueryRequest, activeFilters])

  const toggleTimeline = useCallback(() => {
    setShowTimeline(prev => !prev)
  }, [])

  const handleTimelineSelectRange = useCallback((startTs, endTs) => {
    // Expand timestamps to cover the full bucket range
    let from = startTs
    let to = endTs

    // If it's a single bucket click (same start/end), expand to cover the bucket
    if (startTs === endTs) {
      // Monthly bucket: "2024-01" -> full month
      if (startTs.length === 7) {
        from = startTs + '-01 00:00:00'
        // Approximate end of month
        to = startTs + '-31 23:59:59'
      }
      // Daily bucket: "2024-01-15" -> full day
      else if (startTs.length === 10) {
        from = startTs + ' 00:00:00'
        to = startTs + ' 23:59:59'
      }
      // Hourly bucket: "2024-01-15 14:00:00" -> full hour
      else if (startTs.length >= 19) {
        from = startTs
        to = startTs.substring(0, 14) + '59:59'
      }
    }

    const newFilters = {
      filters: activeFilters?.filters || [],
      logic: activeFilters?.logic || 'AND',
      dateFrom: from,
      dateTo: to,
    }
    setActiveFilters(newFilters)
    setShowFilters(true)
    setCurrentPage(1)
    setSelectedEvent(null)
    loadPage(1, null, newFilters)
  }, [activeFilters, loadPage])

  const handleLoadSavedQuery = useCallback((filterState) => {
    setActiveFilters(filterState)
    setShowFilters(true)
    setCurrentPage(1)
    setSelectedEvent(null)
    loadPage(1, null, filterState)
  }, [loadPage])

  const handleRowSelected = useCallback((event) => {
    const selectedRows = event.api.getSelectedRows()
    if (selectedRows.length > 0) {
      setSelectedEvent(selectedRows[0])
    }
  }, [])

  const handleCloseDetail = useCallback(() => {
    setSelectedEvent(null)
    // Deselect rows in the grid
    if (gridRef.current?.api) {
      gridRef.current.api.deselectAll()
    }
  }, [])

  const handleEventUpdate = useCallback((id, fields) => {
    // Update the event in the local state so the grid reflects changes
    setEvents(prev => prev.map(e => {
      if (e.id === id) {
        return { ...e, ...fields }
      }
      return e
    }))
    // Also update the selected event
    setSelectedEvent(prev => {
      if (prev && prev.id === id) {
        return { ...prev, ...fields }
      }
      return prev
    })
    // Refresh row styles in the grid (for color changes)
    setTimeout(() => {
      if (gridRef.current?.api) {
        gridRef.current.api.redrawRows()
      }
    }, 50)
    setStatus(`Event ${id} updated`)
  }, [])

  // Drag resize for detail panel
  const handleResizeStart = useCallback((e) => {
    e.preventDefault()
    resizingRef.current = true
    const startY = e.clientY
    const startHeight = detailHeight

    const onMouseMove = (moveEvent) => {
      if (!resizingRef.current) return
      const delta = startY - moveEvent.clientY
      const newHeight = Math.min(Math.max(startHeight + delta, 120), 600)
      setDetailHeight(newHeight)
    }

    const onMouseUp = () => {
      resizingRef.current = false
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }

    document.body.style.cursor = 'row-resize'
    document.body.style.userSelect = 'none'
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  }, [detailHeight])

  // Listen for native menu events from Go
  const EventsOn = window.runtime?.EventsOn || function() { return () => {} }

  useEffect(() => {
    const cancelOpen = EventsOn('menu:open-database', () => { handleOpenDB() })
    const cancelImport = EventsOn('menu:import-csv', () => { handleImportCSV() })
    const cancelClose = EventsOn('menu:close-database', () => { handleCloseDB() })
    const cancelExport = EventsOn('menu:export-csv', () => { handleExportCSV() })
    const cancelTheme = EventsOn('menu:theme', () => { setShowThemePicker(true) })
    return () => {
      if (typeof cancelOpen === 'function') cancelOpen()
      if (typeof cancelImport === 'function') cancelImport()
      if (typeof cancelClose === 'function') cancelClose()
      if (typeof cancelExport === 'function') cancelExport()
      if (typeof cancelTheme === 'function') cancelTheme()
    }
  }, [handleOpenDB, handleImportCSV, handleCloseDB, handleExportCSV])

  // Color-coded row styling based on the event's color field
  const getRowStyle = useCallback((params) => {
    const color = params.data?.color
    if (!color) return null
    const colorMap = {
      'RED':    { background: 'rgba(231, 76, 60, 0.15)', borderLeft: '3px solid #e74c3c' },
      'ORANGE': { background: 'rgba(230, 126, 34, 0.15)', borderLeft: '3px solid #e67e22' },
      'YELLOW': { background: 'rgba(241, 196, 15, 0.12)', borderLeft: '3px solid #f1c40f' },
      'GREEN':  { background: 'rgba(46, 204, 113, 0.15)', borderLeft: '3px solid #2ecc71' },
      'BLUE':   { background: 'rgba(52, 152, 219, 0.15)', borderLeft: '3px solid #3498db' },
      'PURPLE': { background: 'rgba(155, 89, 182, 0.15)', borderLeft: '3px solid #9b59b6' },
      'WHITE':  { background: 'rgba(236, 240, 241, 0.1)', borderLeft: '3px solid #ecf0f1' },
      'BLACK':  { background: 'rgba(44, 62, 80, 0.3)', borderLeft: '3px solid #2c3e50' },
    }
    return colorMap[color] || null
  }, [])

  const hasActiveFilters = activeFilters && (
    (activeFilters.filters && activeFilters.filters.length > 0) ||
    (activeFilters.dateFrom && activeFilters.dateTo)
  )

  // If no database is open, show the welcome screen
  if (!dbInfo) {
    return (
      <div className="app-container">
        <ImportProgress visible={importing} />
        <div className="welcome">
          <h1>4n6time</h1>
          <p>Forensic Timeline Viewer</p>
          <div className="actions">
            <button onClick={handleOpenDB}>Open Database</button>
            <button onClick={handleImportCSV}>Import Timeline</button>
          </div>
        </div>
        <div className="status-bar">
          <span className="status-left">{status}</span>
          <span className="status-right">{version ? 'v' + version : ''}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="app-container">
      <ImportProgress visible={importing} />

      <div className="toolbar">
        <button onClick={handleOpenDB}>Open</button>
        <button onClick={handleImportCSV}>Import</button>
        <button onClick={handleCloseDB}>Close</button>
        <div className="toolbar-separator" />
        <button
          className={showFilters ? 'active' : ''}
          onClick={toggleFilters}
        >
          Filters {hasActiveFilters ? '*' : ''}
        </button>
        <button
          className={showSavedQueries ? 'active' : ''}
          onClick={toggleSavedQueries}
        >
          Saved Queries
        </button>
        <button onClick={toggleColumnChooser}>Columns</button>
        <button
          className={showTimeline ? 'active' : ''}
          onClick={toggleTimeline}
        >
          Timeline
        </button>
        <div className="toolbar-separator" />
        <button onClick={handleExportCSV}>Export CSV</button>
        <span className="db-info">
          {dbInfo.path} | {dbInfo.eventCount.toLocaleString()} events
          {dbInfo.minDate && ` | ${dbInfo.minDate} to ${dbInfo.maxDate}`}
        </span>
      </div>

      <ColumnChooser
        visible={showColumnChooser}
        columns={columnDefs}
        onToggle={handleToggleColumn}
        onClose={() => setShowColumnChooser(false)}
      />

      <ThemePicker
        visible={showThemePicker}
        currentTheme={currentTheme}
        onSelect={handleSelectTheme}
        onClose={() => setShowThemePicker(false)}
      />

      <div className="main-content">
        <FilterPanel
          visible={showFilters}
          onApply={handleApplyFilters}
          onClear={handleClearFilters}
          dbInfo={dbInfo}
          activeFilters={activeFilters}
        />

        <SavedQueries
          visible={showSavedQueries}
          onLoad={handleLoadSavedQuery}
          currentFilters={activeFilters}
          dbInfo={dbInfo}
        />

        <div className="grid-wrapper">
          <TimelineChart
            visible={showTimeline}
            filters={activeFilters}
            dbInfo={dbInfo}
            onSelectRange={handleTimelineSelectRange}
          />

          <div className={`grid-container ${lightThemes.has(currentTheme) ? 'ag-theme-alpine' : 'ag-theme-alpine-dark'}`}>
            <AgGridReact
              ref={gridRef}
              rowData={events}
              columnDefs={columnDefs}
              defaultColDef={defaultColDef}
              animateRows={false}
              rowSelection="single"
              suppressCellFocus={false}
              getRowId={(params) => String(params.data.id)}
              getRowStyle={getRowStyle}
              onSelectionChanged={handleRowSelected}
              overlayLoadingTemplate='<span>Loading events...</span>'
              overlayNoRowsTemplate='<span>No events to display</span>'
              loading={loading}
            />
          </div>

          {selectedEvent && (
            <div className="resize-handle" onMouseDown={handleResizeStart} />
          )}

          <EventDetail
            event={selectedEvent}
            onUpdate={handleEventUpdate}
            onClose={handleCloseDetail}
            height={detailHeight}
          />

          <div className="pagination">
            <button onClick={handlePrevPage} disabled={currentPage <= 1 || loading}>
              Previous
            </button>
            <span className="page-info">
              Page {currentPage} of {totalPages.toLocaleString()}
              {' '}({totalCount.toLocaleString()} total events)
            </span>
            <button onClick={handleNextPage} disabled={currentPage >= totalPages || loading}>
              Next
            </button>
          </div>
        </div>
      </div>

      <div className="status-bar">
        <span className="status-left">{status}</span>
        <span className="status-right">{version ? 'v' + version : ''}</span>
      </div>
    </div>
  )
}

export default App
