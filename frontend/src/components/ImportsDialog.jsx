import { useEffect, useState } from 'react'
import { GetImports } from '../../wailsjs/go/main/App'

// ImportsDialog lists every l2t_imports row of the open database, read-only:
// ID, label, file, status, event count, import time (UTC), and error text.
// Failed imports are highlighted. It reloads each time it opens, so it shows
// records written since it was last shown.
export default function ImportsDialog({ visible, hasDatabase, onClose }) {
  const [records, setRecords] = useState([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!visible || !hasDatabase) return
    let cancelled = false
    setLoading(true)
    setError('')
    GetImports()
      .then(r => { if (!cancelled) setRecords(r || []) })
      .catch(err => { if (!cancelled) { setRecords([]); setError(String(err)) } })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [visible, hasDatabase])

  if (!visible) return null

  const failedCount = records.filter(r => r.status === 'failed').length

  let body
  if (!hasDatabase) {
    body = <p>No database is open.</p>
  } else if (loading) {
    body = <p>Loading imports...</p>
  } else if (error) {
    body = <p className="import-failure-error">Could not load imports: {error}</p>
  } else if (records.length === 0) {
    body = <p>No imports are recorded in this database. Events imported before v0.16.0 have no import record.</p>
  } else {
    body = (
      <>
        <p className="imports-dialog-count">
          {records.length} import{records.length === 1 ? '' : 's'}
          {failedCount > 0 ? `, ${failedCount} failed` : ''}. Times are UTC.
        </p>
        <table className="recursive-summary-table imports-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Label</th>
              <th>File</th>
              <th>Status</th>
              <th className="imports-num">Events</th>
              <th>Imported</th>
              <th>Error</th>
            </tr>
          </thead>
          <tbody>
            {records.map(r => (
              <tr key={r.id} className={r.status === 'failed' ? 'imports-row-failed' : undefined}>
                <td className="imports-num">{r.id}</td>
                <td>{r.label}</td>
                <td className="imports-file">{r.filename}</td>
                <td className={`imports-status imports-status-${r.status}`}>{r.status}</td>
                <td className="imports-num">{(r.event_count || 0).toLocaleString()}</td>
                <td className="imports-time">{r.imported_at}</td>
                <td className="imports-error">{r.error_message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </>
    )
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="recursive-summary-dialog imports-dialog" onClick={e => e.stopPropagation()}>
        <div className="recursive-summary-header">
          <h2>Imports</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="recursive-summary-content">
          {body}
        </div>
        <div className="recursive-summary-footer">
          <button className="recursive-close-btn" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  )
}
