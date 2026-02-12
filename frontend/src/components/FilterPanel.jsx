import { useState, useEffect, useCallback } from 'react'
import { GetDistinctValues, GetMinMaxDate } from '../../wailsjs/go/main/App'

function FilterPanel({ visible, onApply, onClear, dbInfo, activeFilters }) {
  const [filters, setFilters] = useState([])
  const [logic, setLogic] = useState('AND')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [distinctValues, setDistinctValues] = useState({})
  const [loading, setLoading] = useState(false)

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
  ]

  // Load distinct values when panel becomes visible or db changes
  useEffect(() => {
    if (!visible || !dbInfo) return

    let cancelled = false
    setLoading(true)

    async function loadValues() {
      const values = {}
      for (const f of filterFields) {
        try {
          const result = await GetDistinctValues(f.field)
          if (!cancelled && result) {
            // result is a map of value -> count, get sorted keys
            values[f.field] = Object.keys(result).sort()
          }
        } catch (err) {
          console.error(`Error loading ${f.field} values:`, err)
        }
      }

      if (!cancelled) {
        setDistinctValues(values)

        // Load date range
        try {
          const dates = await GetMinMaxDate()
          if (dates && dates.length === 2) {
            if (!dateFrom) setDateFrom(dates[0])
            if (!dateTo) setDateTo(dates[1])
          }
        } catch (err) {
          console.error('Error loading date range:', err)
        }

        setLoading(false)
      }
    }

    loadValues()
    return () => { cancelled = true }
  }, [visible, dbInfo])

  const addFilter = useCallback(() => {
    setFilters(prev => [...prev, { field: 'source', operator: '=', value: '' }])
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
    if (e.key === 'Enter') {
      handleApply()
    }
  }, [handleApply])

  if (!visible) return null

  return (
    <div className="filter-panel" onKeyDown={handleKeyDown}>
      <div className="filter-header">
        <span className="filter-title">Filters</span>
        <div className="filter-logic">
          <button
            className={logic === 'AND' ? 'active' : ''}
            onClick={() => setLogic('AND')}
          >AND</button>
          <button
            className={logic === 'OR' ? 'active' : ''}
            onClick={() => setLogic('OR')}
          >OR</button>
        </div>
      </div>

      <div className="filter-date-range">
        <label>Date Range</label>
        <div className="date-inputs">
          <input
            type="text"
            placeholder="YYYY-MM-DD HH:MM:SS"
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
          />
          <span className="date-separator">to</span>
          <input
            type="text"
            placeholder="YYYY-MM-DD HH:MM:SS"
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
          />
        </div>
      </div>

      <div className="filter-list">
        {filters.map((filter, i) => (
          <div key={i} className="filter-row">
            <select
              value={filter.field}
              onChange={(e) => updateFilter(i, 'field', e.target.value)}
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
            >
              <option value="=">=</option>
              <option value="!=">!=</option>
              <option value="LIKE">LIKE</option>
              <option value="NOT LIKE">NOT LIKE</option>
            </select>

            {/* Show dropdown if we have distinct values for this field, text input otherwise */}
            {distinctValues[filter.field] && filter.operator === '=' ? (
              <select
                value={filter.value}
                onChange={(e) => updateFilter(i, 'value', e.target.value)}
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
              />
            )}

            <button className="filter-remove" onClick={() => removeFilter(i)}>x</button>
          </div>
        ))}
      </div>

      <div className="filter-actions">
        <button onClick={addFilter}>+ Add Filter</button>
        <button className="filter-apply" onClick={handleApply}>Apply</button>
        <button onClick={handleClear}>Clear</button>
      </div>

      {loading && <div className="filter-loading">Loading filter values...</div>}
    </div>
  )
}

export default FilterPanel
