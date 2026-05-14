import { useState, useCallback, useEffect, useMemo, useRef } from 'react'
import { AgGridReact } from 'ag-grid-react'
import 'ag-grid-community/styles/ag-grid.css'
import 'ag-grid-community/styles/ag-theme-alpine.css'

import { OpenDatabase, ImportCSV, ImportFolderRecursive, GetCurrentDBInfo, CloseDatabase, QueryEvents, ExportCSV, GetVersion, ToggleBookmark, ConnectPostgres, CreatePostgresDatabase, PushToPostgres, AddExaminerNote, DeleteExaminerNote, UpdateExaminerNoteColor, AdvancedSearch, SaveQuery, BulkUpdateColor, BulkAddTag, BulkSetBookmark, GetTabLimit, SaveTabSession, LoadTabSession, GetAutoRestoreTabs, ForceQuit } from '../wailsjs/go/main/App'
import ImportProgress from './components/ImportProgress'
import PostgresDialog from './components/PostgresDialog'
import FilterPanel from './components/FilterPanel'
import EventDetail from './components/EventDetail'
import SavedQueries from './components/SavedQueries'
import ColumnChooser from './components/ColumnChooser'
import TimelineChart from './components/TimelineChart'
import ThemePicker from './components/ThemePicker'
import AboutDialog from './components/AboutDialog'
import HelpDialog from './components/HelpDialog'
import LoggingDialog from './components/LoggingDialog'
import AddNoteDialog from './components/AddNoteDialog'
import HighlightText from './components/HighlightText'
import TabBar from './components/TabBar'
import SettingsDialog from './components/SettingsDialog'
import RecursiveImportSummaryDialog from './components/RecursiveImportSummaryDialog'
import themes, { lightThemes } from './themes'

const PAGE_SIZE = 1000

const MAIN_TAB_ID = 'tab-main'

const makeTab = (id, label, baseQuery = null) => ({
  id,
  label,
  isMain: id === MAIN_TAB_ID,
  baseQuery,    // { field, op, value } | null
  stale: false,
  // savedState groups all per-tab UI fields. applyTabState() is the single reader;
  // handleTabClick/handleTabClose/handleOpenInNewTab are the writers.
  // Adding new per-tab state requires touching only: (1) this shape, (2) applyTabState, (3) the save call.
  savedState: {
    page: 1,
    search: '',
    searchMode: 'simple',
    searchText: '',
    scrollRow: 0,
    filters: null,
    bookmarkOnly: false,
  },
})

// Read a tab's savedState, accepting either the new object shape or the legacy flat fields
// (savedPage, savedSearch, etc.) from sessions persisted before this refactor.
const getTabSavedState = (tab) => {
  if (tab.savedState) return tab.savedState
  return {
    page: tab.savedPage || 1,
    search: tab.savedSearch || '',
    searchMode: tab.savedSearchMode || 'simple',
    searchText: tab.savedSearchText || '',
    scrollRow: tab.savedScrollRow || 0,
    filters: tab.savedFilters || null,
    bookmarkOnly: false,
  }
}

// Serialize non-main tabs into session JSON for SaveTabSession.
// activeTabLiveState is passed separately because the active tab's savedState is only written
// on tab switches; at close time the live values are in React state, not on the tab object.
// Returns empty string when only the main tab is open (signals "clear session").
const buildTabSessionJSON = (tabs, activeTabId, activeTabLiveState) => {
  const nonMain = tabs.filter(t => !t.isMain)
  if (nonMain.length === 0) return ''
  return JSON.stringify({
    tabs: nonMain.map(t => {
      const ss = t.id === activeTabId ? activeTabLiveState : getTabSavedState(t)
      return { id: t.id, label: t.label, baseQuery: t.baseQuery, savedState: ss }
    }),
    activeTabId,
  })
}

// Named color options matching the database format and EventDetail's color picker
const bulkColorOptions = [
  '', 'RED', 'ORANGE', 'YELLOW', 'GREEN', 'BLUE', 'PURPLE', 'WHITE', 'BLACK',
]
const bulkColorDisplayMap = {
  '': 'transparent',
  'RED': '#e74c3c',
  'ORANGE': '#e67e22',
  'YELLOW': '#f1c40f',
  'GREEN': '#2ecc71',
  'BLUE': '#3498db',
  'PURPLE': '#9b59b6',
  'WHITE': '#ecf0f1',
  'BLACK': '#2c3e50',
}

// Column definitions for the forensic timeline grid
const defaultColDefs = [
  { field: 'bookmark', headerName: '☆', width: 45, pinned: 'left',
    sortable: true, filter: false, resizable: false,
    cellStyle: { textAlign: 'center', cursor: 'pointer', fontSize: '16px', padding: 0 },
  },
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
  const [searchText, setSearchText] = useState('')
  const [activeSearch, setActiveSearch] = useState('')
  const [searchMode, setSearchMode] = useState('simple')
  const [showSearchHelp, setShowSearchHelp] = useState(false)
  const [searchError, setSearchError] = useState('')
  const [showSaveQueryPrompt, setShowSaveQueryPrompt] = useState(false)
  const [saveQueryName, setSaveQueryName] = useState('')
  const [showAbout, setShowAbout] = useState(false)
  const [showHelp, setShowHelp] = useState(false)
  const [showLogging, setShowLogging] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [showRecursiveSummary, setShowRecursiveSummary] = useState(false)
  const [recursiveSummary, setRecursiveSummary] = useState(null)
  const [showPostgres, setShowPostgres] = useState(false)
  const [showPushPostgres, setShowPushPostgres] = useState(false)
  const [showAddNote, setShowAddNote] = useState(false)
  const [filterVersion, setFilterVersion] = useState(0)
  const [pageInputValue, setPageInputValue] = useState('1')
  const [bookmarkOnly, setBookmarkOnly] = useState(false)
  const [currentTheme, setCurrentTheme] = useState(() => {
    try { return window.localStorage?.getItem('4n6time-theme') || 'forensic-dark' }
    catch { return 'forensic-dark' }
  })
  const [columnDefs, setColumnDefs] = useState(defaultColDefs)

  // Tab state
  const [tabs, setTabs] = useState(() => [makeTab(MAIN_TAB_ID, 'All Events')])
  const [activeTabId, setActiveTabId] = useState(MAIN_TAB_ID)
  const [tabLimit, setTabLimit] = useState(5)
  const [contextMenu, setContextMenu] = useState(null)

  const [activeFilters, setActiveFilters] = useState(null)
  const [selectedEvent, setSelectedEvent] = useState(null)
  const [selectedEvents, setSelectedEvents] = useState([])
  const [bulkTag, setBulkTag] = useState('')
  const [bulkColor, setBulkColor] = useState('')
  const [detailHeight, setDetailHeight] = useState(280)
  const [version, setVersion] = useState('')
  const gridRef = useRef(null)
  const resizingRef = useRef(false)
  // Holds the active tab's baseQuery for use in loadPage without adding to deps
  const activeBaseQueryRef = useRef(null)
  // Suppresses the [activeSearch] and [bookmarkOnly] effects during tab switches so the new tab's
  // saved search state doesn't trigger an extraneous loadPage(1) for the old tab.
  const tabSwitchingRef = useRef(false)
  // Suppresses the TimelineChart histogram fetch during user-initiated tab switches only.
  // NOT set during handleRestoreTabSession so the histogram loads immediately on database open.
  const histogramSuppressRef = useRef(false)
  // Scroll row to restore after the next [events] update (tab switch).
  const pendingScrollRowRef = useRef(0)
  // Signals that the next [events] update should scroll to row 0 (page navigation).
  const pendingScrollTopRef = useRef(false)
  // Refs tracking latest state for callbacks with empty/limited deps arrays.
  const tabsRef = useRef([makeTab(MAIN_TAB_ID, 'All Events')])
  const activeTabIdRef = useRef(MAIN_TAB_ID)
  const dbInfoRef = useRef(null)
  // Continuously updated snapshot of the active tab's live state. Used when building session
  // JSON at close time, because the tab object's savedState is only written during tab switches.
  const liveTabStateRef = useRef({ page: 1, search: '', searchMode: 'simple', searchText: '', filters: null, bookmarkOnly: false })

  // Pending session loaded from DB — shown in notification bar for manual restore.
  const [pendingSession, setPendingSession] = useState(null)
  // True when the user clicked the window close button — shows the confirm dialog.
  const [showCloseConfirm, setShowCloseConfirm] = useState(false)
  // Bumped by the [activeTabId] effect after each tab switch completes.
  // Passed to TimelineChart so its suppressed effect re-fires with the final state.
  const [histogramVersion, setHistogramVersion] = useState(0)

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

  // Keep refs in sync so callbacks with empty/limited deps always see current values.
  useEffect(() => { tabsRef.current = tabs }, [tabs])
  useEffect(() => { activeTabIdRef.current = activeTabId }, [activeTabId])
  useEffect(() => { dbInfoRef.current = dbInfo }, [dbInfo])
  useEffect(() => {
    liveTabStateRef.current = { page: currentPage, search: activeSearch, searchMode, searchText, filters: activeFilters, bookmarkOnly }
  }, [currentPage, activeSearch, searchMode, searchText, activeFilters, bookmarkOnly])

  // Load version and tab limit on mount
  useEffect(() => {
    GetVersion().then(v => { if (v) setVersion(v) })
    GetTabLimit().then(limit => { if (limit > 0) setTabLimit(limit) }).catch(() => {})
  }, [])

  const totalPages = Math.max(1, Math.ceil(totalCount / PAGE_SIZE))

  // Keep page input in sync when currentPage changes via button navigation or loadPage
  useEffect(() => {
    setPageInputValue(String(currentPage))
  }, [currentPage])

  // Cell renderer that highlights search matches (skips bookmark column)
  const HighlightCellRenderer = useCallback((params) => {
    if (params.colDef.field === 'bookmark') {
      return params.data?.bookmark ? '★' : '☆'
    }
    const search = params.context?.activeSearch
    if (!search || !params.value) return params.value ?? ''
    return <HighlightText text={String(params.value)} search={search} />
  }, [])

  const defaultColDef = useMemo(() => ({
    sortable: true,
    resizable: true,
    filter: true,
    cellRenderer: HighlightCellRenderer,
  }), [HighlightCellRenderer])

  // Handle bookmark toggle
  const handleBookmarkToggle = useCallback(async (rowid) => {
    try {
      const newVal = await ToggleBookmark(rowid)
      setEvents(prev => prev.map(e =>
        e.id === rowid ? { ...e, bookmark: newVal } : e
      ))
      setSelectedEvent(prev =>
        prev && prev.id === rowid ? { ...prev, bookmark: newVal } : prev
      )
    } catch (err) {
      console.error('Error toggling bookmark:', err)
    }
  }, [])

  // Build the query request from current filters and active tab's base query
  const buildQueryRequest = useCallback((page, filterState) => {
    const bq = activeBaseQueryRef.current
    const req = {
      filters: [],
      logic: 'AND',
      orderBy: 'datetime',
      page: page,
      pageSize: PAGE_SIZE,
      searchText: activeSearch,
      bookmarkOnly: bookmarkOnly,
      baseField: bq?.field || '',
      baseOp: bq?.op || '',
      baseValue: bq?.value || '',
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
  }, [activeFilters, activeSearch, bookmarkOnly])

  const loadPage = useCallback(async (page, info, filterState) => {
    const db = info || dbInfo
    if (!db) return

    setLoading(true)
    setStatus('Loading events...')
    setSearchError('')

    try {
      let result

      if (searchMode === 'advanced' && activeSearch) {
        // Inject the active tab's base query into the raw WHERE clause
        const bq = activeBaseQueryRef.current
        let clause = activeSearch
        if (bq && bq.field) {
          const safeVal = bq.value.replace(/'/g, "''")
          clause = `${bq.field} ${bq.op} '${safeVal}' AND (${activeSearch})`
        }
        result = await AdvancedSearch(clause, page, PAGE_SIZE)
      } else {
        const req = buildQueryRequest(page, filterState)
        result = await QueryEvents(req)
      }

      if (result) {
        setEvents(result.events || [])
        setTotalCount(result.totalCount)
        setCurrentPage(result.page)

        const filterCount = (filterState || activeFilters)?.filters?.length || 0
        const filterLabel = filterCount > 0 ? ` (${filterCount} filter${filterCount > 1 ? 's' : ''} active)` : ''
        const searchLabel = activeSearch ? (searchMode === 'advanced' ? ' | Advanced: ' + activeSearch : ` | Search: "${activeSearch}"`) : ''
        const bookmarkLabel = bookmarkOnly ? ' | ★ Bookmarked only' : ''
        setStatus(`Showing ${result.events?.length || 0} of ${result.totalCount.toLocaleString()} events${filterLabel}${searchLabel}${bookmarkLabel}`)
      }
    } catch (err) {
      if (searchMode === 'advanced') {
        setSearchError(String(err))
      }
      setStatus('Error: ' + err)
    } finally {
      setLoading(false)
    }
  }, [dbInfo, buildQueryRequest, activeFilters, searchMode, activeSearch])

  // Apply a tab's savedState to active React state. This is the only place these setStates
  // live for tab-swap purposes. User-initiated switch callers set tabSwitchingRef.current and
  // histogramSuppressRef.current = true before calling, then call setActiveTabId() after;
  // the [activeTabId] effect clears both flags and triggers loadPage.
  const applyTabState = useCallback((tab) => {
    const s = getTabSavedState(tab)
    activeBaseQueryRef.current = tab.baseQuery || null
    setCurrentPage(s.page)
    setActiveSearch(s.search)
    setSearchMode(s.searchMode)
    setSearchText(s.searchText)
    setSearchError('')
    setActiveFilters(s.filters)
    setBookmarkOnly(s.bookmarkOnly)
    pendingScrollRowRef.current = s.scrollRow
    setSelectedEvent(null)
    setSelectedEvents([])
  }, [])

  const handleRestoreTabSession = useCallback((sessionData) => {
    try {
      const { tabs: savedTabs, activeTabId: savedActiveId } = JSON.parse(sessionData)
      if (!Array.isArray(savedTabs) || savedTabs.length === 0) return
      const restored = savedTabs.map(t => {
        const tab = makeTab(t.id, t.label, t.baseQuery || null)
        // Merge savedState from session if present (new shape); legacy sessions have none.
        if (t.savedState) tab.savedState = { ...tab.savedState, ...t.savedState }
        return tab
      })
      setTabs([makeTab(MAIN_TAB_ID, 'All Events'), ...restored])
      const targetId = savedActiveId || restored[0].id
      const targetTab = restored.find(t => t.id === targetId) || restored[0]
      tabSwitchingRef.current = true
      // histogramSuppressRef is intentionally NOT set here. During session restore the
      // histogram should load immediately. Only user-initiated tab switches suppress it.
      applyTabState(targetTab)
      setActiveTabId(targetId)
    } catch { /* malformed JSON */ }
  }, [applyTabState])

  const handleDismissSession = useCallback(() => {
    SaveTabSession('').catch(() => {})
    setPendingSession(null)
  }, [])

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
        try {
          const sessionData = await LoadTabSession()
          if (sessionData) {
            const autoRestore = await GetAutoRestoreTabs().catch(() => false)
            if (autoRestore) {
              handleRestoreTabSession(sessionData)
            } else {
              setPendingSession(sessionData)
            }
          }
        } catch { /* session load failure is non-fatal */ }
      } else {
        setStatus('')
      }
    } catch (err) {
      setStatus('Error: ' + err)
    }
  }, [loadPage, handleRestoreTabSession])

  const handlePostgresConnect = useCallback(async (info) => {
    setShowPostgres(false)
    setDbInfo(info)
    setActiveFilters(null)
    setShowFilters(false)
    setSelectedEvent(null)
    setStatus(`Connected: ${info.path} (${info.eventCount.toLocaleString()} events)`)
    await loadPage(1, info, null)
    try {
      const sessionData = await LoadTabSession()
      if (sessionData) {
        const autoRestore = await GetAutoRestoreTabs().catch(() => false)
        if (autoRestore) {
          handleRestoreTabSession(sessionData)
        } else {
          setPendingSession(sessionData)
        }
      }
    } catch { /* session load failure is non-fatal */ }
  }, [loadPage, handleRestoreTabSession])

  const handlePushToPostgres = useCallback(async (host, port, dbName, user, password, sslMode) => {
    setShowPushPostgres(false)
    setImporting(true)
    setStatus('Pushing data to PostgreSQL...')
    try {
      const result = await PushToPostgres(host, port, dbName, user, password, sslMode)
      if (result) {
        setStatus(result)
      }
    } catch (err) {
      setStatus('Push error: ' + err)
    } finally {
      setImporting(false)
    }
  }, [])

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
        // Only offer session restore if no non-main tabs are currently open.
        const currentNonMain = tabsRef.current.filter(t => !t.isMain)
        if (currentNonMain.length === 0) {
          try {
            const sessionData = await LoadTabSession()
            if (sessionData) {
              const autoRestore = await GetAutoRestoreTabs().catch(() => false)
              if (autoRestore) {
                handleRestoreTabSession(sessionData)
              } else {
                setPendingSession(sessionData)
              }
            }
          } catch { /* non-fatal */ }
        }
      } else {
        setStatus('')
      }
    } catch (err) {
      setStatus('Error: ' + err)
    } finally {
      setImporting(false)
    }
  }, [loadPage, handleRestoreTabSession])

  const handleImportFolderRecursive = useCallback(async () => {
    try {
      setImporting(true)
      setStatus('Importing folder (recursive)...')
      const summary = await ImportFolderRecursive()
      if (summary) {
        const info = await GetCurrentDBInfo()
        if (info) {
          setDbInfo(info)
          setActiveFilters(null)
          setShowFilters(false)
          setSelectedEvent(null)
          setStatus(`Imported: ${summary.totalEvents.toLocaleString()} events from ${summary.totalFilesProcessed} files`)
          await loadPage(1, info, null)
        }
        setRecursiveSummary(summary)
        setShowRecursiveSummary(true)
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
      // Save per-tab state; empty string clears if main-only.
      const sessionData = buildTabSessionJSON(tabsRef.current, activeTabIdRef.current, liveTabStateRef.current)
      await SaveTabSession(sessionData).catch(() => {})
      await CloseDatabase()
      dbInfoRef.current = null
      setDbInfo(null)
      setEvents([])
      setTotalCount(0)
      setCurrentPage(1)
      setActiveFilters(null)
      setShowFilters(false)
      setSelectedEvent(null)
      setStatus('')
      setPendingSession(null)
      // Reset tabs to just the main tab
      setTabs([makeTab(MAIN_TAB_ID, 'All Events')])
      setActiveTabId(MAIN_TAB_ID)
      activeBaseQueryRef.current = null
    } catch (err) {
      setStatus('Error: ' + err)
    }
  }, [])

  const handlePrevPage = useCallback(() => {
    if (currentPage > 1) { pendingScrollTopRef.current = true; loadPage(currentPage - 1) }
  }, [currentPage, loadPage])

  const handleNextPage = useCallback(() => {
    if (currentPage < totalPages) { pendingScrollTopRef.current = true; loadPage(currentPage + 1) }
  }, [currentPage, totalPages, loadPage])

  const handleFirstPage = useCallback(() => {
    if (currentPage > 1) { pendingScrollTopRef.current = true; loadPage(1) }
  }, [currentPage, loadPage])

  const handleLastPage = useCallback(() => {
    if (currentPage < totalPages) { pendingScrollTopRef.current = true; loadPage(totalPages) }
  }, [currentPage, totalPages, loadPage])

  const handlePageInputSubmit = useCallback(() => {
    const num = parseInt(pageInputValue, 10)
    if (!isNaN(num) && num >= 1 && num <= totalPages && num !== currentPage) {
      pendingScrollTopRef.current = true
      loadPage(num)
    } else {
      setPageInputValue(String(currentPage))
    }
  }, [pageInputValue, totalPages, currentPage, loadPage])

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

  const handleSearch = useCallback(() => {
    setSearchError('')
    setActiveSearch(searchText)
    setCurrentPage(1)
    setSelectedEvent(null)
  }, [searchText])

  const handleClearSearch = useCallback(() => {
    setSearchText('')
    setActiveSearch('')
    setSearchError('')
    setCurrentPage(1)
    setSelectedEvent(null)
  }, [])

  const handleToggleSearchMode = useCallback(() => {
    const newMode = searchMode === 'simple' ? 'advanced' : 'simple'
    setSearchMode(newMode)
    setSearchText('')
    setActiveSearch('')
    setSearchError('')
    setCurrentPage(1)
    setSelectedEvent(null)
  }, [searchMode])

  const handleSaveAdvancedQuery = useCallback(async () => {
    const name = saveQueryName.trim()
    if (!name || !activeSearch) return
    try {
      const queryData = JSON.stringify({ advanced: true, whereClause: activeSearch })
      await SaveQuery(name, queryData)
      setSaveQueryName('')
      setShowSaveQueryPrompt(false)
      setStatus(`Query saved: ${name}`)
    } catch (err) {
      setStatus('Error saving query: ' + err)
    }
  }, [saveQueryName, activeSearch])

  // Reload when activeSearch changes
  useEffect(() => {
    if (tabSwitchingRef.current) return
    if (dbInfo) {
      loadPage(1)
    }
    if (gridRef.current?.api) {
      gridRef.current.api.refreshCells({ force: true })
    }
  }, [activeSearch]) // eslint-disable-line react-hooks/exhaustive-deps

  // Reload when bookmarkOnly filter changes
  useEffect(() => {
    if (tabSwitchingRef.current) return
    if (dbInfo) {
      loadPage(1)
    }
  }, [bookmarkOnly]) // eslint-disable-line react-hooks/exhaustive-deps

  // Reload when active tab changes (new baseQuery takes effect)
  useEffect(() => {
    tabSwitchingRef.current = false
    histogramSuppressRef.current = false
    if (dbInfo) {
      const tab = tabs.find(t => t.id === activeTabId)
      const page = tab ? getTabSavedState(tab).page : 1
      loadPage(page)
      // Signal TimelineChart to re-fetch now that the switch is complete and the flag is cleared.
      setHistogramVersion(prev => prev + 1)
    }
  }, [activeTabId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Scroll after events load: to top on page navigation, or to saved row on tab switch.
  useEffect(() => {
    if (pendingScrollTopRef.current) {
      pendingScrollTopRef.current = false
      setTimeout(() => { gridRef.current?.api?.ensureIndexVisible?.(0, 'top') }, 50)
    } else {
      const row = pendingScrollRowRef.current
      if (row > 0 && gridRef.current?.api) {
        pendingScrollRowRef.current = 0
        setTimeout(() => { gridRef.current?.api?.ensureIndexVisible?.(row, 'top') }, 50)
      }
    }
  }, [events])

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

  // Mark all tabs OTHER than the active one as stale
  const markOtherTabsStale = useCallback(() => {
    setTabs(prev => prev.map(t => t.id !== activeTabId ? { ...t, stale: true } : t))
  }, [activeTabId])

  const handleNoteAdded = useCallback(() => {
    loadPage(currentPage)
    setFilterVersion(v => v + 1)
    markOtherTabsStale()
  }, [currentPage, loadPage, markOtherTabsStale])

  const handleDeleteExaminerNote = useCallback(async (negatedId) => {
    try {
      await DeleteExaminerNote(negatedId)
      setSelectedEvent(null)
      loadPage(currentPage)
      setFilterVersion(v => v + 1)
      setStatus('Examiner note deleted')
      markOtherTabsStale()
    } catch (err) {
      console.error('Error deleting examiner note:', err)
      setStatus('Error deleting note: ' + err)
    }
  }, [currentPage, loadPage, markOtherTabsStale])

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
    let from = startTs
    let to = endTs

    if (startTs === endTs) {
      // Monthly bucket: "2024-01" -> full month
      if (startTs.length === 7) {
        from = startTs + '-01 00:00:00'
        // Compute actual last day: new Date(year, month, 0) returns the last day of the given month
        const [y, m] = startTs.split('-').map(Number)
        const lastDay = new Date(y, m, 0).getDate()
        to = startTs + '-' + String(lastDay).padStart(2, '0') + ' 23:59:59'
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
    if (filterState && filterState.advanced && filterState.whereClause) {
      setSearchMode('advanced')
      setSearchText(filterState.whereClause)
      setActiveSearch(filterState.whereClause)
      setSearchError('')
      setCurrentPage(1)
      setSelectedEvent(null)
      return
    }
    setActiveFilters(filterState)
    setShowFilters(true)
    setCurrentPage(1)
    setSelectedEvent(null)
    loadPage(1, null, filterState)
  }, [loadPage])

  const handleRowSelected = useCallback((event) => {
    const selectedRows = event.api.getSelectedRows()
    setSelectedEvents(selectedRows)
    if (selectedRows.length === 1) {
      setSelectedEvent(selectedRows[0])
    } else {
      setSelectedEvent(null)
    }
  }, [])

  const handleCloseDetail = useCallback(() => {
    setSelectedEvent(null)
    setSelectedEvents([])
    if (gridRef.current?.api) {
      gridRef.current.api.deselectAll()
    }
  }, [])

  const handleBulkApply = useCallback(async () => {
    const color = bulkColor
    const tag = bulkTag.trim()
    if (!color && !tag) return
    const ids = selectedEvents.map(e => e.id)
    try {
      if (color) {
        await BulkUpdateColor(ids, color)
      }
      if (tag) {
        await BulkAddTag(ids, tag)
      }
      setBulkTag('')
      setBulkColor('')
      loadPage(currentPage)
      markOtherTabsStale()
    } catch (err) {
      console.error('Bulk apply error:', err)
    }
  }, [selectedEvents, bulkColor, bulkTag, loadPage, currentPage, markOtherTabsStale])

  const handleBulkBookmark = useCallback(async (value) => {
    const ids = selectedEvents.map(e => e.id)
    try {
      await BulkSetBookmark(ids, value)
      setEvents(prev => prev.map(e => ids.includes(e.id) ? { ...e, bookmark: value } : e))
      setSelectedEvents(prev => prev.map(e => ({ ...e, bookmark: value })))
      setTimeout(() => { if (gridRef.current?.api) gridRef.current.api.redrawRows() }, 0)
      markOtherTabsStale()
    } catch (err) {
      console.error('Bulk bookmark error:', err)
    }
  }, [selectedEvents, markOtherTabsStale])

  const handleEventUpdate = useCallback((id, fields) => {
    setEvents(prev => prev.map(e => {
      if (e.id === id) {
        return { ...e, ...fields }
      }
      return e
    }))
    setSelectedEvent(prev => {
      if (prev && prev.id === id) {
        return { ...prev, ...fields }
      }
      return prev
    })
    setTimeout(() => {
      if (gridRef.current?.api) {
        gridRef.current.api.redrawRows()
      }
    }, 50)
    setStatus(`Event ${id} updated`)
    markOtherTabsStale()
  }, [markOtherTabsStale])

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

  // --- Tab management ---

  const handleTabClick = useCallback((tabId) => {
    if (tabId === activeTabId || !dbInfo) return
    const newTab = tabs.find(t => t.id === tabId)
    if (!newTab) return

    tabSwitchingRef.current = true
    histogramSuppressRef.current = true

    // Snapshot scroll position before the grid changes.
    const firstRow = gridRef.current?.api?.getFirstDisplayedRow?.() ?? 0

    // Save all outgoing tab state under savedState.
    setTabs(prev => prev.map(t => t.id === activeTabId ? {
      ...t,
      savedState: { page: currentPage, search: activeSearch, searchMode, searchText, scrollRow: firstRow, filters: activeFilters, bookmarkOnly },
    } : t))

    applyTabState(newTab)
    setActiveTabId(tabId)
  }, [activeTabId, tabs, currentPage, dbInfo, activeSearch, searchMode, searchText, activeFilters, bookmarkOnly, applyTabState])

  const handleTabClose = useCallback((tabId) => {
    const idx = tabs.findIndex(t => t.id === tabId)
    const prevTab = tabs[idx - 1] || tabs[0]

    setTabs(prev => prev.filter(t => t.id !== tabId))

    if (tabId === activeTabId) {
      tabSwitchingRef.current = true
      histogramSuppressRef.current = true
      applyTabState(prevTab)
      setActiveTabId(prevTab?.id || MAIN_TAB_ID)
    }
  }, [activeTabId, tabs, applyTabState])

  const handleTabRefresh = useCallback(() => {
    setTabs(prev => prev.map(t => t.id === activeTabId ? { ...t, stale: false } : t))
    setFilterVersion(v => v + 1)
    loadPage(currentPage)
  }, [activeTabId, currentPage, loadPage])

  const handleSetTabLimit = useCallback((newLimit) => {
    setTabLimit(newLimit)
  }, [])

  // Open a new tab with a base query filter from context menu
  const handleOpenInNewTab = useCallback((field, op, value, label) => {
    if (tabs.length >= tabLimit) {
      setStatus(`Tab limit reached (${tabLimit}). Close a tab or increase the limit in settings.`)
      return
    }
    const id = 'tab-' + Date.now()
    const newTab = makeTab(id, label, { field, op, value })

    tabSwitchingRef.current = true
    histogramSuppressRef.current = true

    // Snapshot scroll position before the grid changes.
    const firstRow = gridRef.current?.api?.getFirstDisplayedRow?.() ?? 0

    setTabs(prev => [
      ...prev.map(t => t.id === activeTabId ? {
        ...t,
        savedState: { page: currentPage, search: activeSearch, searchMode, searchText, scrollRow: firstRow, filters: activeFilters, bookmarkOnly },
      } : t),
      newTab,
    ])
    // New tab always starts with cleared user-applied state (no search, no bookmark, no filters).
    applyTabState(newTab)
    setActiveTabId(id)
  }, [tabs, tabLimit, activeTabId, currentPage, activeSearch, searchMode, searchText, activeFilters, bookmarkOnly, applyTabState])

  const handleSaveTabQuery = useCallback(async (tabId, name) => {
    const tab = tabs.find(t => t.id === tabId)
    if (!tab?.baseQuery) return
    const { field, op, value } = tab.baseQuery
    const queryData = JSON.stringify({
      tabQuery: true,
      field,
      op,
      value,
      label: tab.label,
    })
    try {
      await SaveQuery(name, queryData)
      setStatus(`Saved tab query: "${name}"`)
    } catch (err) {
      setStatus('Error saving tab query: ' + err)
    }
  }, [tabs])

  // Right-click context menu on grid cells
  const handleCellContextMenu = useCallback((params) => {
    params.event.preventDefault()
    const row = params.data
    if (!row) return

    const fieldDefs = [
      { key: 'filename',         label: 'filename' },
      { key: 'host',             label: 'host' },
      { key: 'user',             label: 'user' },
      { key: 'source',           label: 'source' },
      { key: 'sourcetype',       label: 'sourcetype' },
      { key: 'desc',             label: 'desc' },
      { key: 'url',              label: 'URL',           dbField: 'URL' },
      { key: 'computer_name',    label: 'computer_name' },
      { key: 'event_identifier', label: 'event_identifier' },
      { key: 'source_name',      label: 'source_name' },
    ]

    const items = []
    for (const f of fieldDefs) {
      const { key, label } = f
      const val = row[key]
      if (val != null && String(val).trim()) {
        const strVal = String(val).trim()
        const displayVal = key === 'desc' && strVal.length > 60
          ? strVal.substring(0, 60) + '...'
          : strVal
        const dbField = f.dbField || f.key
        const exactFields = new Set(['event_identifier', 'filename', 'URL'])
        items.push({
          tabLabel: `${label}: ${strVal.substring(0, 30)}`,
          menuLabel: `${label}: ${displayVal}`,
          field: dbField,
          op: exactFields.has(dbField) ? '=' : 'LIKE',
          value: strVal,
        })
      }
    }

    const estimatedHeight = 40 + items.length * 30
    const estimatedWidth = 400
    let menuX = params.event.clientX
    let menuY = params.event.clientY
    if (menuX + estimatedWidth > window.innerWidth) menuX = window.innerWidth - estimatedWidth - 8
    if (menuY + estimatedHeight > window.innerHeight) menuY = window.innerHeight - estimatedHeight - 8
    if (menuX < 8) menuX = 8
    if (menuY < 8) menuY = 8

    setContextMenu({ x: menuX, y: menuY, items })
  }, [])

  // Close context menu on outside click or Escape
  useEffect(() => {
    if (!contextMenu) return
    const onClick = () => setContextMenu(null)
    const onKey = (e) => { if (e.key === 'Escape') setContextMenu(null) }
    window.addEventListener('click', onClick)
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('click', onClick)
      window.removeEventListener('keydown', onKey)
    }
  }, [contextMenu])

  // Listen for native menu events from Go
  const EventsOn = window.runtime?.EventsOn || function() { return () => {} }

  useEffect(() => {
    const cancelOpen = EventsOn('menu:open-database', () => { handleOpenDB() })
    const cancelImport = EventsOn('menu:import-csv', () => { handleImportCSV() })
    const cancelImportEZ = EventsOn('menu:import-folder-recursive', () => { handleImportFolderRecursive() })
    const cancelClose = EventsOn('menu:close-database', () => { handleCloseDB() })
    const cancelExport = EventsOn('menu:export-csv', () => { handleExportCSV() })
    const cancelTheme = EventsOn('menu:theme', () => { setShowThemePicker(true) })
    const cancelCut = EventsOn('menu:cut', () => { document.execCommand('cut') })
    const cancelCopy = EventsOn('menu:copy', () => { document.execCommand('copy') })
    const cancelPaste = EventsOn('menu:paste', () => { document.execCommand('paste') })
    const cancelSelectAll = EventsOn('menu:select-all', () => { document.execCommand('selectAll') })
    const cancelAbout = EventsOn('menu:about', () => { setShowAbout(true) })
    const cancelHelp = EventsOn('menu:help', () => { setShowHelp(true) })
    const cancelLogging = EventsOn('menu:logging', () => { setShowLogging(true) })
    const cancelSettings = EventsOn('menu:settings', () => { setShowSettings(true) })
    return () => {
      if (typeof cancelOpen === 'function') cancelOpen()
      if (typeof cancelImport === 'function') cancelImport()
      if (typeof cancelImportEZ === 'function') cancelImportEZ()
      if (typeof cancelClose === 'function') cancelClose()
      if (typeof cancelExport === 'function') cancelExport()
      if (typeof cancelTheme === 'function') cancelTheme()
      if (typeof cancelCut === 'function') cancelCut()
      if (typeof cancelCopy === 'function') cancelCopy()
      if (typeof cancelPaste === 'function') cancelPaste()
      if (typeof cancelSelectAll === 'function') cancelSelectAll()
      if (typeof cancelAbout === 'function') cancelAbout()
      if (typeof cancelHelp === 'function') cancelHelp()
      if (typeof cancelLogging === 'function') cancelLogging()
      if (typeof cancelSettings === 'function') cancelSettings()
    }
  }, [handleOpenDB, handleImportCSV, handleImportFolderRecursive, handleCloseDB, handleExportCSV])

  // Handle window close button and File > Quit — show React confirmation dialog.
  // Registered once with empty deps; actual save+quit handled in the dialog's confirm handler.
  // menu:quit skips the dialog when no database is open.
  useEffect(() => {
    const cancelBeforeClose = EventsOn('app:before-close', () => {
      if (dbInfoRef.current) {
        setShowCloseConfirm(true)
      } else {
        ForceQuit()
      }
    })
    const cancelQuit = EventsOn('menu:quit', () => {
      if (dbInfoRef.current) {
        setShowCloseConfirm(true)
      } else {
        ForceQuit()
      }
    })
    return () => {
      if (typeof cancelBeforeClose === 'function') cancelBeforeClose()
      if (typeof cancelQuit === 'function') cancelQuit()
    }
  }, [])

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

  const activeTab = tabs.find(t => t.id === activeTabId)

  // If no database is open, show the welcome screen
  if (!dbInfo) {
    return (
      <div className="app-container">
        <ImportProgress visible={importing} />
        <ThemePicker
          visible={showThemePicker}
          currentTheme={currentTheme}
          onSelect={handleSelectTheme}
          onClose={() => setShowThemePicker(false)}
        />
        <AboutDialog
          visible={showAbout}
          version={version}
          onClose={() => setShowAbout(false)}
        />
        <HelpDialog
          visible={showHelp}
          onClose={() => setShowHelp(false)}
        />
        <LoggingDialog
          visible={showLogging}
          onClose={() => setShowLogging(false)}
        />
        <SettingsDialog
          visible={showSettings}
          onClose={() => setShowSettings(false)}
          onTabLimitChange={handleSetTabLimit}
        />
        <PostgresDialog
          visible={showPostgres}
          onConnect={handlePostgresConnect}
          onClose={() => setShowPostgres(false)}
        />
        <RecursiveImportSummaryDialog
          visible={showRecursiveSummary}
          summary={recursiveSummary}
          onClose={() => setShowRecursiveSummary(false)}
        />
        <div className="welcome">
          <h1>4n6time</h1>
          <p>Forensic Timeline Viewer</p>
          <div className="actions">
            <button onClick={handleOpenDB}>Open Database</button>
            <button onClick={handleImportCSV}>Import Timeline</button>
            <button onClick={handleImportFolderRecursive} title="Walks the selected folder up to 3 levels deep and imports any supported EZ Tools CSV files.">Import Folder (Recursive)</button>
            <button onClick={() => setShowPostgres(true)}>Connect to PostgreSQL</button>
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
        <div className="search-bar">
          <button
            className={`search-mode-btn ${searchMode === 'advanced' ? 'active' : ''}`}
            onClick={handleToggleSearchMode}
            title={searchMode === 'simple' ? 'Switch to advanced SQL mode' : 'Switch to simple keyword mode'}
          >
            {searchMode === 'simple' ? 'Aa' : 'SQL'}
          </button>
          <input
            type="text"
            className={searchMode === 'advanced' ? 'search-input-advanced' : ''}
            placeholder={searchMode === 'advanced' ? "source = 'FILE' AND datetime > '2025-01-01'" : 'Search events...'}
            value={searchText}
            onChange={(e) => { setSearchText(e.target.value); setSearchError('') }}
            onKeyDown={(e) => { if (e.key === 'Enter') handleSearch() }}
          />
          {activeSearch && (
            <button className="search-clear" onClick={handleClearSearch} title="Clear search">x</button>
          )}
          <button className="search-btn" onClick={handleSearch}>Search</button>
          {searchMode === 'advanced' && (
            <>
              <button
                className="search-help-btn"
                onClick={() => setShowSearchHelp(prev => !prev)}
                title="Show available fields and operators"
              >
                ?
              </button>
              {activeSearch && (
                <button
                  className="search-save-btn"
                  onClick={() => setShowSaveQueryPrompt(true)}
                  title="Save this query"
                >
                  Save
                </button>
              )}
            </>
          )}
          {showSearchHelp && (
            <div className="search-help-popup">
              <div className="search-help-header">
                <span>Advanced Search Help</span>
                <button onClick={() => setShowSearchHelp(false)}>x</button>
              </div>
              <div className="search-help-body">
                <p><strong>Fields:</strong> datetime, timezone, MACB, source, sourcetype, type, user, host, desc, filename, inode, notes, format, extra, reportnotes, inreport, tag, color, offset, store_number, store_index, vss_store_number, URL, record_number, event_identifier, event_type, source_name, user_sid, computer_name, bookmark</p>
                <p><strong>Operators:</strong> =, !=, LIKE, NOT LIKE, &gt;, &lt;, &gt;=, &lt;=, AND, OR, BETWEEN</p>
                <p><strong>PostgreSQL note:</strong> The columns <em>desc</em>, <em>user</em>, and <em>offset</em> are reserved words and will be auto-quoted when using a PostgreSQL database.</p>
                <p><strong>Examples:</strong></p>
                <code>source = 'EXAMINER'</code>
                <code>desc LIKE '%malware%' AND host = 'WORKSTATION1'</code>
                <code>datetime BETWEEN '2025-01-01' AND '2025-06-01'</code>
              </div>
            </div>
          )}
          {showSaveQueryPrompt && (
            <div className="search-save-popup">
              <input
                type="text"
                placeholder="Query name..."
                value={saveQueryName}
                onChange={(e) => setSaveQueryName(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') handleSaveAdvancedQuery(); if (e.key === 'Escape') setShowSaveQueryPrompt(false) }}
                autoFocus
              />
              <button onClick={handleSaveAdvancedQuery}>Save</button>
              <button onClick={() => setShowSaveQueryPrompt(false)}>x</button>
            </div>
          )}
          {searchError && <div className="search-error">{searchError}</div>}
        </div>
        <button
          className={`bookmark-filter-btn ${bookmarkOnly ? 'active' : ''}`}
          onClick={() => setBookmarkOnly(prev => !prev)}
          title={bookmarkOnly ? 'Show all events' : 'Show bookmarked only'}
        >
          {bookmarkOnly ? '★' : '☆'}
        </button>
        <button
          className="add-note-btn"
          onClick={() => setShowAddNote(true)}
          title="Add Examiner Note"
        >
          +
        </button>
        <div className="toolbar-separator" />
        <button onClick={handleExportCSV}>Export CSV</button>
        {dbInfo.driver === 'sqlite' && (
          <button onClick={() => setShowPushPostgres(true)}>Push to PostgreSQL</button>
        )}
        <span className="db-info">
          {dbInfo.path} | {dbInfo.eventCount.toLocaleString()} events
          {dbInfo.minDate && ` | ${dbInfo.minDate} to ${dbInfo.maxDate}`}
        </span>
      </div>

      <TabBar
        tabs={tabs}
        activeTabId={activeTabId}
        onTabClick={handleTabClick}
        onTabClose={handleTabClose}
        isActiveStale={activeTab?.stale || false}
        onRefresh={handleTabRefresh}
        onSaveTab={handleSaveTabQuery}
      />

      {pendingSession && (() => {
        let tabCount = 0
        try { tabCount = JSON.parse(pendingSession).tabs?.length || 0 } catch {}
        return (
          <div className="session-restore-bar">
            <span className="session-restore-msg">
              {tabCount} saved tab{tabCount !== 1 ? 's' : ''} from last session
            </span>
            <button
              className="session-restore-btn"
              onClick={() => { handleRestoreTabSession(pendingSession); setPendingSession(null) }}
            >Restore</button>
            <button
              className="session-dismiss-btn"
              onClick={handleDismissSession}
            >Dismiss</button>
          </div>
        )
      })()}

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

      <AboutDialog
        visible={showAbout}
        version={version}
        onClose={() => setShowAbout(false)}
      />

      <HelpDialog
        visible={showHelp}
        onClose={() => setShowHelp(false)}
      />

      <LoggingDialog
        visible={showLogging}
        onClose={() => setShowLogging(false)}
      />

      <SettingsDialog
        visible={showSettings}
        onClose={() => setShowSettings(false)}
        onTabLimitChange={handleSetTabLimit}
      />

      <RecursiveImportSummaryDialog
        visible={showRecursiveSummary}
        summary={recursiveSummary}
        onClose={() => setShowRecursiveSummary(false)}
      />

      <PostgresDialog
        visible={showPushPostgres}
        mode="push"
        onPush={handlePushToPostgres}
        onClose={() => setShowPushPostgres(false)}
      />

      <AddNoteDialog
        visible={showAddNote}
        onClose={() => setShowAddNote(false)}
        onAdded={handleNoteAdded}
      />

      {showCloseConfirm && (
        <div className="modal-overlay">
          <div className="settings-dialog" onClick={e => e.stopPropagation()}>
            <div className="logging-header">
              <h2>Close 4n6time?</h2>
            </div>
            <div className="logging-content">
              <p>Any open tabs will be saved for next session.</p>
              <div className="logging-actions">
                <button onClick={async () => {
                  try {
                    if (dbInfoRef.current) {
                      const sessionData = buildTabSessionJSON(tabsRef.current, activeTabIdRef.current, liveTabStateRef.current)
                      await SaveTabSession(sessionData).catch(() => {})
                    }
                  } catch { /* ignore save errors */ }
                  ForceQuit()
                }}>Close</button>
                <button className="logging-close-btn" onClick={() => setShowCloseConfirm(false)}>Cancel</button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Right-click context menu overlay */}
      {contextMenu && (
        <div
          className="context-menu"
          style={{ top: contextMenu.y, left: contextMenu.x }}
          onClick={e => e.stopPropagation()}
        >
          <div className="context-menu-header">
            <span>Search in new tab</span>
            <button
              className="context-menu-close"
              onClick={() => setContextMenu(null)}
              title="Dismiss"
            >×</button>
          </div>
          {contextMenu.items.length === 0 ? (
            <div className="context-menu-item disabled">No fields available</div>
          ) : (
            contextMenu.items.map((item, i) => (
              <div
                key={i}
                className="context-menu-item"
                onClick={() => {
                  setContextMenu(null)
                  handleOpenInNewTab(item.field, item.op, item.value, item.tabLabel)
                }}
              >
                {item.menuLabel}
              </div>
            ))
          )}
        </div>
      )}

      <div className="main-content">
        <FilterPanel
          visible={showFilters}
          onApply={handleApplyFilters}
          onClear={handleClearFilters}
          dbInfo={dbInfo}
          activeFilters={activeFilters}
          filterVersion={filterVersion}
          baseQuery={activeTab?.baseQuery || null}
        />

        <SavedQueries
          visible={showSavedQueries}
          onLoad={handleLoadSavedQuery}
          onOpenInNewTab={({ field, op, value, label }) => handleOpenInNewTab(field, op, value, label)}
          currentFilters={activeFilters}
          dbInfo={dbInfo}
        />

        <div className="grid-wrapper">
          <TimelineChart
            visible={showTimeline}
            filters={activeFilters}
            dbInfo={dbInfo}
            onSelectRange={handleTimelineSelectRange}
            theme={currentTheme}
            activeSearch={activeSearch}
            searchMode={searchMode}
            bookmarkOnly={bookmarkOnly}
            baseQuery={activeTab?.baseQuery || null}
            histogramSuppressRef={histogramSuppressRef}
            histogramVersion={histogramVersion}
          />

          <div className={`grid-container ${lightThemes.has(currentTheme) ? 'ag-theme-alpine' : 'ag-theme-alpine-dark'}`}>
            <AgGridReact
              ref={gridRef}
              rowData={events}
              columnDefs={columnDefs}
              defaultColDef={defaultColDef}
              context={{ activeSearch }}
              animateRows={false}
              rowSelection="multiple"
              suppressRowClickSelection={false}
              suppressCellFocus={false}
              suppressContextMenu={true}
              getRowId={(params) => String(params.data.id)}
              getRowStyle={getRowStyle}
              onSelectionChanged={handleRowSelected}
              onCellClicked={(params) => {
                if (params.colDef.field === 'bookmark') {
                  handleBookmarkToggle(params.data.id)
                }
              }}
              onCellContextMenu={handleCellContextMenu}
              overlayLoadingTemplate='<span>Loading events...</span>'
              overlayNoRowsTemplate='<span>No events to display</span>'
              loading={loading}
            />
          </div>

          {selectedEvents.length > 1 ? (
            <div className="bulk-action-bar">
              <span className="bulk-count">{selectedEvents.length} events selected</span>
              <div className="bulk-actions">
                <label className="bulk-action-label">Color:</label>
                <div className="bulk-color-swatches">
                  {bulkColorOptions.map(c => (
                    <button
                      key={c || 'none'}
                      className={`bulk-color-swatch ${bulkColor === c ? 'selected' : ''}`}
                      style={{
                        background: bulkColorDisplayMap[c] || 'transparent',
                        border: c === '' ? '1px dashed #808080' : '1px solid transparent',
                      }}
                      title={c || 'None'}
                      onClick={() => setBulkColor(c)}
                    />
                  ))}
                </div>
                <span className="bulk-separator" />
                <label className="bulk-action-label">Tag:</label>
                <input
                  type="text"
                  className="bulk-tag-input"
                  placeholder="Add tag..."
                  value={bulkTag}
                  onChange={(e) => setBulkTag(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleBulkApply() }}
                />
                <span className="bulk-separator" />
                <button className="bulk-tag-apply" onClick={handleBulkApply} disabled={!bulkColor && !bulkTag.trim()}>Apply Changes</button>
                <span className="bulk-separator" />
                <button className="bulk-bookmark-btn" onClick={() => handleBulkBookmark(1)} title="Bookmark all selected">Bookmark All</button>
                <button className="bulk-bookmark-btn" onClick={() => handleBulkBookmark(0)} title="Unbookmark all selected">Unbookmark All</button>
                <span className="bulk-separator" />
                <button className="bulk-clear-btn" onClick={handleCloseDetail}>Clear Selection</button>
              </div>
            </div>
          ) : (
            <>
              {selectedEvent && (
                <div className="resize-handle" onMouseDown={handleResizeStart} />
              )}
              <EventDetail
                event={selectedEvent}
                onUpdate={handleEventUpdate}
                onClose={handleCloseDetail}
                height={detailHeight}
                searchText={activeSearch}
                onToggleBookmark={handleBookmarkToggle}
                onDeleteNote={handleDeleteExaminerNote}
              />
            </>
          )}

          <div className="pagination">
            <button onClick={handleFirstPage} disabled={currentPage <= 1 || loading}>
              First
            </button>
            <button onClick={handlePrevPage} disabled={currentPage <= 1 || loading}>
              Prev
            </button>
            <span className="page-info">Page</span>
            <input
              className="page-input"
              type="text"
              value={pageInputValue}
              onChange={(e) => setPageInputValue(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handlePageInputSubmit() }}
              onBlur={() => setPageInputValue(String(currentPage))}
              disabled={loading}
            />
            <span className="page-info">of {totalPages.toLocaleString()}</span>
            <button onClick={handleNextPage} disabled={currentPage >= totalPages || loading}>
              Next
            </button>
            <button onClick={handleLastPage} disabled={currentPage >= totalPages || loading}>
              Last
            </button>
            <span className="page-info page-total">({totalCount.toLocaleString()} total events)</span>
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
