import { useState, useEffect, useCallback } from 'react'
import { GetTabLimit, SetTabLimit, GetAutoRestoreTabs, SetAutoRestoreTabs, GetPostgresHost, SetPostgresHost } from '../../wailsjs/go/main/App'

function SettingsDialog({ visible, onClose, onTabLimitChange }) {
  const [limitInput, setLimitInput] = useState('5')
  const [autoRestore, setAutoRestore] = useState(false)
  const [postgresHost, setPostgresHost] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (visible) {
      setError('')
      GetTabLimit()
        .then(n => { if (n > 0) setLimitInput(String(n)) })
        .catch(() => {})
      GetAutoRestoreTabs()
        .then(v => setAutoRestore(!!v))
        .catch(() => {})
      GetPostgresHost()
        .then(h => setPostgresHost(h || ''))
        .catch(() => {})
    }
  }, [visible])

  const handleApply = useCallback(async () => {
    const n = parseInt(limitInput, 10)
    if (isNaN(n) || n < 1 || n > 20) {
      setError('Max tabs must be between 1 and 20.')
      return
    }
    setSaving(true)
    setError('')
    try {
      await SetTabLimit(n)
      await SetAutoRestoreTabs(autoRestore)
      await SetPostgresHost(postgresHost.trim())
      if (onTabLimitChange) onTabLimitChange(n)
      onClose()
    } catch (err) {
      setError(String(err))
    } finally {
      setSaving(false)
    }
  }, [limitInput, autoRestore, postgresHost, onClose, onTabLimitChange])

  if (!visible) return null

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="settings-dialog" onClick={e => e.stopPropagation()}>
        <div className="logging-header">
          <h2>Settings</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="logging-content">
          <div className="settings-field-row">
            <label className="settings-field-label">Max open tabs</label>
            <input
              className="settings-field-input"
              type="number"
              min="1"
              max="20"
              value={limitInput}
              onChange={e => { setLimitInput(e.target.value); setError('') }}
              onKeyDown={e => { if (e.key === 'Enter') handleApply() }}
            />
          </div>
          <p className="settings-field-hint">
            Maximum number of tabs that can be open at once (1–20).
          </p>
          <div className="settings-field-row">
            <label className="settings-field-label">Auto-restore tabs</label>
            <input
              type="checkbox"
              checked={autoRestore}
              onChange={e => setAutoRestore(e.target.checked)}
            />
          </div>
          <p className="settings-field-hint">
            Automatically restore saved tabs when opening a database.
          </p>
          <div className="settings-field-row">
            <label className="settings-field-label">Default PostgreSQL host</label>
            <input
              className="settings-field-input"
              style={{ flex: 1, textAlign: 'left' }}
              type="text"
              value={postgresHost}
              onChange={e => setPostgresHost(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') handleApply() }}
              placeholder="localhost"
            />
          </div>
          <p className="settings-field-hint">
            Pre-fills the host field when opening the PostgreSQL connection dialog.
          </p>
          {error && <div className="logging-error">{error}</div>}
          <div className="logging-actions">
            <button onClick={handleApply} disabled={saving}>
              {saving ? 'Saving...' : 'Save'}
            </button>
            <button className="logging-close-btn" onClick={onClose}>Cancel</button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default SettingsDialog
