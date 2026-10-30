import { useState, useEffect, useRef } from 'react'
import { ConnectPostgres, CreatePostgresDatabase, CancelPostgresConnect, GetOpenFailure, GetPostgresHost } from '../../wailsjs/go/main/App'

// The fields as the dialog first shows them after startup. The first open
// then replaces the host with the default PostgreSQL hostname from Settings
// (loadDefaultHost). Reset restores these, the same way. Later opens keep
// whatever the fields last held.
const INITIAL_FIELDS = {
  host: 'localhost',
  port: '5432',
  dbName: '',
  user: '',
  password: '',
  sslMode: 'disable',
}

function PostgresDialog({ visible, mode = 'connect', onConnect, onPush, onFailure, onClose }) {
  const [host, setHost] = useState(INITIAL_FIELDS.host)
  const [port, setPort] = useState(INITIAL_FIELDS.port)
  const [dbName, setDbName] = useState(INITIAL_FIELDS.dbName)
  const [user, setUser] = useState(INITIAL_FIELDS.user)
  const [password, setPassword] = useState(INITIAL_FIELDS.password)
  const [sslMode, setSslMode] = useState(INITIAL_FIELDS.sslMode)
  // busy is the attempt in progress: 'connect', 'create', or null. Only that
  // button shows its busy label; the other is disabled with its normal label.
  const [busy, setBusy] = useState(null)
  const loading = busy !== null
  // error is the main inline error line. errorDetail is the raw error text
  // shown beneath it, muted, when error holds the plain-language message.
  const [error, setError] = useState('')
  const [errorDetail, setErrorDetail] = useState('')

  const showError = (main, detail = '') => {
    setError(main)
    setErrorDetail(detail)
  }

  const loadDefaultHost = () => {
    GetPostgresHost()
      .then(h => setHost(h || INITIAL_FIELDS.host))
      .catch(() => {})
  }

  // Only the first open loads the Settings default host; later opens keep the
  // host last in the field, like the other fields.
  const hostLoadedRef = useRef(false)
  useEffect(() => {
    if (visible && !hostLoadedRef.current) {
      hostLoadedRef.current = true
      loadDefaultHost()
    }
  }, [visible])

  if (!visible) return null

  // A failed connect or create: the backend recorded it (GetOpenFailure), so
  // its plain-language message is the main line and the raw error goes
  // beneath it. If no failure is recorded or the call fails, the raw error is
  // shown alone, as before.
  const showFailure = async (err) => {
    const raw = String(err)
    let failure = null
    try {
      failure = await GetOpenFailure()
    } catch { /* fall back to the raw error */ }
    if (failure?.message) {
      showError(failure.message, raw)
    } else {
      showError(raw)
    }
  }

  const handleConnect = async () => {
    if (!dbName || !user) {
      showError('Database name and username are required')
      return
    }
    setBusy('connect')
    showError('')
    try {
      // null when the attempt was cancelled (handleCancelAttempt).
      const info = await ConnectPostgres(host, port, dbName, user, password, sslMode)
      if (info) {
        // A successful connect closes the dialog; the password is not kept.
        setPassword(INITIAL_FIELDS.password)
        onConnect(info)
      }
    } catch (err) {
      await showFailure(err)
      // The error shows inline here; the parent shows only the status bar
      // message (no modal), since the settings can be fixed in this dialog.
      if (onFailure) onFailure(err)
    } finally {
      setBusy(null)
    }
  }

  const handleCreate = async () => {
    if (!dbName || !user) {
      showError('Database name and username are required')
      return
    }
    setBusy('create')
    showError('')
    try {
      const info = await CreatePostgresDatabase(host, port, dbName, user, password, sslMode)
      if (info) {
        // A successful connect closes the dialog; the password is not kept.
        setPassword(INITIAL_FIELDS.password)
        onConnect(info)
      }
    } catch (err) {
      await showFailure(err)
      // The error shows inline here; the parent shows only the status bar
      // message (no modal), since the settings can be fixed in this dialog.
      if (onFailure) onFailure(err)
    } finally {
      setBusy(null)
    }
  }

  const handlePush = () => {
    if (!dbName || !user) {
      showError('Database name and username are required')
      return
    }
    // Delegate to parent; dialog closes and ImportProgress shows status
    onPush(host, port, dbName, user, password, sslMode)
  }

  // Cancels the connect or create in progress. The backend returns null for
  // it: no switch, the current database stays open, and no failure dialog.
  const handleCancelAttempt = () => {
    CancelPostgresConnect().catch(() => {})
    showError('Connection attempt cancelled.')
  }

  // Closing the dialog (x, overlay, or Cancel when idle) cancels any attempt.
  // In connect mode it also clears the password; the other fields are kept.
  const handleClose = () => {
    if (loading) CancelPostgresConnect().catch(() => {})
    if (mode !== 'push') setPassword(INITIAL_FIELDS.password)
    onClose()
  }

  // Reset restores every field to its first-open value, host included (from
  // the Settings default, as on open), and clears the inline error.
  const handleReset = () => {
    setHost(INITIAL_FIELDS.host)
    setPort(INITIAL_FIELDS.port)
    setDbName(INITIAL_FIELDS.dbName)
    setUser(INITIAL_FIELDS.user)
    setPassword(INITIAL_FIELDS.password)
    setSslMode(INITIAL_FIELDS.sslMode)
    showError('')
    loadDefaultHost()
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && !loading) {
      if (mode === 'push') {
        handlePush()
      } else {
        handleConnect()
      }
    }
  }

  return (
    <div className="modal-overlay" onClick={handleClose}>
      <div className="pg-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="pg-header">
          <h2>{mode === 'push' ? 'Push to PostgreSQL' : 'Connect to PostgreSQL'}</h2>
          <button className="modal-close" onClick={handleClose}>x</button>
        </div>
        <div className="pg-content">
          <div className="pg-field">
            <label>Host</label>
            <input
              type="text"
              value={host}
              onChange={(e) => setHost(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="localhost"
              disabled={loading}
            />
          </div>
          <div className="pg-field">
            <label>Port</label>
            <input
              type="text"
              value={port}
              onChange={(e) => setPort(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="5432"
              disabled={loading}
            />
          </div>
          <div className="pg-field">
            <label>Database Name</label>
            <input
              type="text"
              value={dbName}
              onChange={(e) => setDbName(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="4n6time"
              disabled={loading}
              autoFocus
            />
          </div>
          <div className="pg-field">
            <label>Username</label>
            <input
              type="text"
              value={user}
              onChange={(e) => setUser(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="postgres"
              disabled={loading}
            />
          </div>
          <div className="pg-field">
            <label>Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onKeyDown={handleKeyDown}
              disabled={loading}
            />
          </div>
          <div className="pg-field">
            <label>SSL Mode</label>
            <select
              value={sslMode}
              onChange={(e) => setSslMode(e.target.value)}
              disabled={loading}
            >
              <option value="disable">disable</option>
              <option value="prefer">prefer</option>
              <option value="require">require</option>
            </select>
          </div>
          {mode !== 'push' && (
            <div className="pg-reset-row">
              <button type="button" className="pg-reset" onClick={handleReset} disabled={loading}>
                Reset
              </button>
            </div>
          )}
          {error && (
            <div className="pg-error">
              {error}
              {errorDetail && <div className="pg-error-detail">{errorDetail}</div>}
            </div>
          )}
          <div className="pg-actions">
            {mode === 'push' ? (
              <button onClick={handlePush} disabled={loading}>
                Push Data
              </button>
            ) : (
              <>
                <button onClick={handleConnect} disabled={loading}>
                  {busy === 'connect' ? 'Connecting...' : 'Connect'}
                </button>
                <button onClick={handleCreate} disabled={loading}>
                  {busy === 'create' ? 'Creating...' : 'Create & Connect'}
                </button>
              </>
            )}
            {/* During an attempt Cancel stops it and keeps the dialog open
                so the settings can be corrected; otherwise it closes. */}
            <button className="pg-cancel" onClick={loading ? handleCancelAttempt : handleClose}>
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default PostgresDialog
