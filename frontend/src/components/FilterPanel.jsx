import { useState, useEffect, useCallback, useRef } from 'react'
import { GetFilterValues, GetMinMaxDate, GetFilteredMinMaxDate } from '../../wailsjs/go/main/App'

const ADVANCED_MODE_TIP = 'Not applied in advanced search mode. Add conditions to your WHERE clause.'
const OPERATORS = ['=', '!=', 'LIKE', 'NOT LIKE']

// importOptionLabel is the dropdown text for one import: its label and file
// name when it has a label, otherwise its file name. Records missing from the
// map (not expected once loaded) fall back to a plain note, never the bare id.
// scopeKey identifies a filter panel scope: the tab base query, or the
// unscoped main view. It keys the filter values cache.
function scopeKey(baseQuery) {
  if (!baseQuery) return 'unscoped'
  return JSON.stringify([baseQuery.field, baseQuery.op, baseQuery.value])
}

// sortForDisplay orders each field's values as the panel always has: import
// ids numerically, every other field with the default text sort.
function sortForDisplay(result) {
  const values = {}
  for (const [field, list] of Object.entries(result)) {
    const copy = [...(list || [])]
    values[field] = field === 'import_id'
      ? copy.sort((a, b) => Number(a) - Number(b))
      : copy.sort()
  }
  return values
}

function importOptionLabel(id, records) {
  const rec = records[id]
  if (!rec) return `Import ${id} (record missing)`
  return rec.label ? `${rec.label} (${rec.filename})` : rec.filename
}

// disabled is set while advanced search mode is active. The controls keep
// their values so switching back to simple mode restores them unchanged.
function FilterPanel({ visible, onApply, onClear, dbInfo, activeFilters, filterVersion, baseQuery, disabled = false, importRecords = {} }) {
  const [filters, setFilters] = useState([])
  const [logic, setLogic] = useState('AND')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [distinctValues, setDistinctValues] = useState({})
  const [loading, setLoading] = useState(false)
  // Filter values by scope, in memory only. Values depend only on the data and
  // the tab base query, so a new database (dbInfo) or a data change
  // (filterVersion) starts a new cache; a filter Apply or Clear does not.
  const cacheRef = useRef({ entries: new Map(), dbInfo: null, version: null })

  // Sync local state when activeFilters change externally (e.g. timeline selection)
  useEffect(() => {
    if (!activeFilters) return
    if (activeFilters.dateFrom) setDateFrom(activeFilters.dateFrom)
    if (activeFilters.dateTo) setDateTo(activeFilters.dateTo)
    if (activeFilters.filters) setFilters(activeFilters.filters)
    if (activeFilters.logic) setLogic(activeFilters.logic)
  }, [activeFilters])

  // Filterable fields and their display names
  const filterFields = [
    { field: 'source', label: 'Source' },
    { field: 'sourcetype', label: 'Source Type' },
    { field: 'type', label: 'Type' },
    { field: 'user', label: 'User' },
    { field: 'host', label: 'Host' },
    { field: 'import_id', label: 'Import' },
  ]

  // Load the dropdown values when the panel becomes visible, the database or
  // its data changes, or the tab scope changes. Active filters are not a
  // dependency: the values never depend on them, only on the tab base query.
  // activeFilters is still read below, at run time, for the date range.
  useEffect(() => {
    if (!visible || !dbInfo) return

    if (cacheRef.current.dbInfo !== dbInfo || cacheRef.current.version !== filterVersion) {
      // Replaced, not cleared, so a request still in flight for the old data
      // writes into the discarded map.
      cacheRef.current = { entries: new Map(), dbInfo, version: filterVersion }
    }
    const entries = cacheRef.current.entries
    const key = scopeKey(baseQuery)

    let cancelled = false
    setLoading(true)

    async function loadValues() {
      let values = entries.get(key)
      if (!values) {
        values = {}
        try {
          const result = await GetFilterValues(baseQuery?.field || '', baseQuery?.op || '', baseQuery?.value || '')
          values = sortForDisplay(result || {})
          // A field that failed is missing; leave the scope uncached so the
          // next load retries it.
          if (filterFields.every(f => f.field in values)) {
            entries.set(key, values)
          }
        } catch (err) {
          console.error('Error loading filter values:', err)
        }
      }

      if (!cancelled) {
        setDistinctValues(values)

        // Only load the DB date range when no date range is currently active.
        // This preserves histogram-selected ranges and user-typed dates on all tabs.
        if (!activeFilters?.dateFrom && !activeFilters?.dateTo) {
          try {
            let dates
            if (baseQuery) {
              dates = await GetFilteredMinMaxDate(baseQuery.field, baseQuery.op, baseQuery.value)
            } else {
              dates = await GetMinMaxDate()
            }
            if (dates && dates.length === 2) {
              if (!cancelled) {
                setDateFrom(dates[0])
                setDateTo(dates[1])
              }
            }
          } catch (err) {
            console.error('Error loading date range:', err)
          }
        }

        setLoading(false)
      }
    }

    loadValues()
    return () => { cancelled = true }
  }, [visible, dbInfo, filterVersion, baseQuery]) // eslint-disable-line react-hooks/exhaustive-deps

  const addFilter = useCallback(() => {
    setFilters(prev => [...prev, { field: 'source', operator: '=', value: '' }])
  }, [])

  // Changing to the Import field forces "=": import ids support nothing else.
  const changeField = useCallback((index, field) => {
    setFilters(prev => {
      const updated = [...prev]
      updated[index] = field === 'import_id'
        ? { ...updated[index], field, operator: '=' }
        : { ...updated[index], field }
      return updated
    })
  }, [])

  const updateFilter = useCallback((index, key, val) => {
    setFilters(prev => {
      const updated = [...prev]
      updated[index] = { ...updated[index], [key]: val }
      return updated
    })
  }, [])

  const removeFilter = useCallback((index) => {
    setFilters(prev => prev.filter((_, i) => i !== index))
  }, [])

  const handleApply = useCallback(() => {
    // Build filter list, excluding empty values
    const activeFilters = filters.filter(f => f.value.trim() !== '')
    onApply({
      filters: activeFilters,
      logic,
      dateFrom,
      dateTo,
    })
  }, [filters, logic, dateFrom, dateTo, onApply])

  const handleClear = useCallback(() => {
    setFilters([])
    setLogic('AND')
    // Reset dates to full range
    if (dbInfo) {
      setDateFrom(dbInfo.minDate || '')
      setDateTo(dbInfo.maxDate || '')
    }
    onClear()
  }, [dbInfo, onClear])

  const handleKeyDown = useCallback((e) => {
    if (e.key === 'Enter' && !disabled) {
      handleApply()
    }
  }, [handleApply, disabled])

  if (!visible) return null

  // Shared props for every control while advanced mode is active.
  const tip = disabled ? ADVANCED_MODE_TIP : undefined
  const sectionStyle = disabled ? { opacity: 0.5 } : undefined

  return (
    <div className="filter-panel" onKeyDown={handleKeyDown} title={tip}>
      <div className="filter-header">
        <span className="filter-title">Filters</span>
        <div className="filter-logic" style={sectionStyle}>
          <button
            className={logic === 'AND' ? 'active' : ''}
            onClick={() => setLogic('AND')}
            disabled={disabled}
            title={tip}
          >AND</button>
          <button
            className={logic === 'OR' ? 'active' : ''}
            onClick={() => setLogic('OR')}
            disabled={disabled}
            title={tip}
          >OR</button>
        </div>
      </div>

      {disabled && (
        <div className="filter-loading">{ADVANCED_MODE_TIP}</div>
      )}

      <div className="filter-date-range" style={sectionStyle}>
        <label>Date Range</label>
        <div className="date-inputs">
          <input
            type="text"
            placeholder="YYYY-MM-DD HH:MM:SS"
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
            disabled={disabled}
            title={tip}
          />
          <span className="date-separator">to</span>
          <input
            type="text"
            placeholder="YYYY-MM-DD HH:MM:SS"
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
            disabled={disabled}
            title={tip}
          />
        </div>
      </div>

      <div className="filter-list" style={sectionStyle}>
        {filters.map((filter, i) => (
          <div key={i} className="filter-row">
            <select
              value={filter.field}
              onChange={(e) => changeField(i, e.target.value)}
              disabled={disabled}
              title={tip}
            >
              {filterFields.map(f => (
                <option key={f.field} value={f.field}>{f.label}</option>
              ))}
              <option value="desc">Description</option>
              <option value="filename">Filename</option>
              <option value="tag">Tag</option>
              <option value="notes">Notes</option>
              <option value="extra">Extra</option>
            </select>

            <select
              value={filter.operator}
              onChange={(e) => updateFilter(i, 'operator', e.target.value)}
              disabled={disabled}
              title={tip}
            >
              {(filter.field === 'import_id' ? ['='] : OPERATORS).map(op => (
                <option key={op} value={op}>{op}</option>
              ))}
            </select>

            {/* Import: a dropdown of imports with events in this tab's scope */}
            {filter.field === 'import_id' ? (
              <select
                value={filter.value}
                onChange={(e) => updateFilter(i, 'value', e.target.value)}
                disabled={disabled}
                title={tip}
              >
                <option value="">-- select import --</option>
                {(distinctValues.import_id || []).map(id => (
                  <option key={id} value={id}>{importOptionLabel(id, importRecords)}</option>
                ))}
              </select>
            ) : distinctValues[filter.field] && filter.operator === '=' ? (
              /* Show dropdown if we have distinct values for this field, text input otherwise */
              <select
                value={filter.value}
                onChange={(e) => updateFilter(i, 'value', e.target.value)}
                disabled={disabled}
                title={tip}
              >
                <option value="">-- select --</option>
                {distinctValues[filter.field].map(v => (
                  <option key={v} value={v}>{v}</option>
                ))}
              </select>
            ) : (
              <input
                type="text"
                value={filter.value}
                placeholder={filter.operator.includes('LIKE') ? '%pattern%' : 'value'}
                onChange={(e) => updateFilter(i, 'value', e.target.value)}
                disabled={disabled}
                title={tip}
              />
            )}

            <button className="filter-remove" onClick={() => removeFilter(i)} disabled={disabled} title={tip}>x</button>
          </div>
        ))}
      </div>

      <div className="filter-actions" style={sectionStyle}>
        <button onClick={addFilter} disabled={disabled} title={tip}>+ Add Filter</button>
        <button className="filter-apply" onClick={handleApply} disabled={disabled} title={tip}>Apply</button>
        <button onClick={handleClear} disabled={disabled} title={tip}>Clear</button>
      </div>

      {loading && <div className="filter-loading">Loading filter values...</div>}
    </div>
  )
}

export default FilterPanel
