import { useState, useEffect, useCallback } from 'react'
import { EnableLogging, DisableLogging, GetLoggingStatus, SetLoggingPersist } from '../../wailsjs/go/main/App'

function LoggingDialog({ visible, onClose }) {
  const [status, setStatus] = useState({ enabled: false, filePath: '', persist: false })
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const refreshStatus = useCallback(async () => {
    try {
      const s = await GetLoggingStatus()
      if (s) setStatus(s)
    } catch (err) {
      setError(String(err))
    }
  }, [])

  useEffect(() => {
    if (visible) {
      setError('')
      refreshStatus()
    }
  }, [visible, refreshStatus])

  const handleEnable = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const path = await EnableLogging()
      if (path) {
        await refreshStatus()
      }
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }, [refreshStatus])

  const handleDisable = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      await DisableLogging()
      await refreshStatus()
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }, [refreshStatus])

  const handleTogglePersist = useCallback(async (e) => {
    const persist = e.target.checked
    try {
      await SetLoggingPersist(persist)
      await refreshStatus()
    } catch (err) {
      setError(String(err))
    }
  }, [refreshStatus])

  if (!visible) return null

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="logging-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="logging-header">
          <h2>Logging</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="logging-content">
          <div className="logging-status-row">
            <span className="logging-label">Status</span>
            <span className={`logging-badge ${status.enabled ? 'enabled' : 'disabled'}`}>
              {status.enabled ? 'Enabled' : 'Disabled'}
            </span>
          </div>

          {status.filePath && (
            <div className="logging-status-row">
              <span className="logging-label">Log file</span>
              <span className="logging-path">{status.filePath}</span>
            </div>
          )}

          <div className="logging-persist-row">
            <label>
              <input
                type="checkbox"
                checked={status.persist}
                onChange={handleTogglePersist}
              />
              Resume logging on next launch
            </label>
          </div>

          {error && <div className="logging-error">{error}</div>}

          <div className="logging-actions">
            {!status.enabled ? (
              <button onClick={handleEnable} disabled={loading}>
                {loading ? 'Opening...' : 'Enable Logging...'}
              </button>
            ) : (
              <button onClick={handleDisable} disabled={loading}>
                {loading ? 'Stopping...' : 'Disable Logging'}
              </button>
            )}
            <button className="logging-close-btn" onClick={onClose}>Close</button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default LoggingDialog
