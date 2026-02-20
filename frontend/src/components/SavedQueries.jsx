import { useState, useEffect, useCallback } from 'react'
import { GetSavedQueries, SaveQuery, DeleteSavedQuery } from '../../wailsjs/go/main/App'

function SavedQueries({ visible, onLoad, currentFilters, dbInfo }) {
  const [queries, setQueries] = useState([])
  const [saveName, setSaveName] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Load saved queries when panel becomes visible or db changes
  useEffect(() => {
    if (!visible || !dbInfo) return
    loadQueries()
  }, [visible, dbInfo])

  const loadQueries = useCallback(async () => {
    setLoading(true)
    try {
      const result = await GetSavedQueries()
      setQueries(result || [])
    } catch (err) {
      console.error('Error loading saved queries:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  const handleSave = useCallback(async () => {
    const name = saveName.trim()
    if (!name) {
      setError('Enter a name for this query')
      return
    }

    // Check for duplicate
    if (queries.some(q => q.Name === name)) {
      setError('A query with this name already exists')
      return
    }

    // Serialize current filters as JSON
    const queryData = JSON.stringify(currentFilters || {
      filters: [],
      logic: 'AND',
      dateFrom: '',
      dateTo: '',
    })

    try {
      await SaveQuery(name, queryData)
      setSaveName('')
      setError('')
      await loadQueries()
    } catch (err) {
      setError('Error saving: ' + err)
    }
  }, [saveName, queries, currentFilters, loadQueries])

  const handleLoad = useCallback((queryStr) => {
    try {
      const filterState = JSON.parse(queryStr)
      onLoad(filterState)
    } catch (err) {
      console.error('Error parsing saved query:', err)
      setError('Could not parse saved query')
    }
  }, [onLoad])

  const handleDelete = useCallback(async (name) => {
    try {
      await DeleteSavedQuery(name)
      await loadQueries()
    } catch (err) {
      setError('Error deleting: ' + err)
    }
  }, [loadQueries])

  const handleKeyDown = useCallback((e) => {
    if (e.key === 'Enter') {
      handleSave()
    }
  }, [handleSave])

  if (!visible) return null

  return (
    <div className="saved-queries-panel">
      <div className="sq-header">
        <span className="sq-title">Saved Queries</span>
      </div>

      {/* Save current filters */}
      <div className="sq-save-row">
        <input
          type="text"
          value={saveName}
          placeholder="Name for current filters..."
          onChange={(e) => { setSaveName(e.target.value); setError('') }}
          onKeyDown={handleKeyDown}
        />
        <button onClick={handleSave}>Save</button>
      </div>

      {error && <div className="sq-error">{error}</div>}

      {/* Query list */}
      <div className="sq-list">
        {loading && <div className="sq-loading">Loading...</div>}

        {!loading && queries.length === 0 && (
          <div className="sq-empty">No saved queries yet</div>
        )}

        {queries.map(q => {
          let summary = ''
          try {
            const parsed = JSON.parse(q.Query)
            if (parsed.advanced && parsed.whereClause) {
              const clause = parsed.whereClause
              summary = 'SQL: ' + (clause.length > 40 ? clause.substring(0, 40) + '...' : clause)
            } else {
              const parts = []
              if (parsed.filters && parsed.filters.length > 0) {
                parts.push(`${parsed.filters.length} filter${parsed.filters.length > 1 ? 's' : ''}`)
              }
              if (parsed.dateFrom && parsed.dateTo) {
                parts.push('date range')
              }
              if (parsed.logic === 'OR') {
                parts.push('OR logic')
              }
              summary = parts.join(', ') || 'no filters'
            }
          } catch {
            summary = 'raw query'
          }

          return (
            <div key={q.Name} className="sq-item">
              <div className="sq-item-info" onClick={() => handleLoad(q.Query)}>
                <span className="sq-item-name">{q.Name}</span>
                <span className="sq-item-summary">{summary}</span>
              </div>
              <button className="sq-item-delete" onClick={() => handleDelete(q.Name)}>x</button>
            </div>
          )
        })}
      </div>
    </div>
  )
}

export default SavedQueries
