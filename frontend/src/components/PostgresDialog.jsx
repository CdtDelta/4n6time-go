import { useState } from 'react'
import { ConnectPostgres, CreatePostgresDatabase } from '../../wailsjs/go/main/App'

function PostgresDialog({ visible, mode = 'connect', onConnect, onPush, onClose }) {
  const [host, setHost] = useState('localhost')
  const [port, setPort] = useState('5432')
  const [dbName, setDbName] = useState('')
  const [user, setUser] = useState('')
  const [password, setPassword] = useState('')
  const [sslMode, setSslMode] = useState('disable')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  if (!visible) return null

  const handleConnect = async () => {
    if (!dbName || !user) {
      setError('Database name and username are required')
      return
    }
    setLoading(true)
    setError('')
    try {
      const info = await ConnectPostgres(host, port, dbName, user, password, sslMode)
      if (info) {
        onConnect(info)
      }
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }

  const handleCreate = async () => {
    if (!dbName || !user) {
      setError('Database name and username are required')
      return
    }
    setLoading(true)
    setError('')
    try {
      const info = await CreatePostgresDatabase(host, port, dbName, user, password, sslMode)
      if (info) {
        onConnect(info)
      }
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }

  const handlePush = () => {
    if (!dbName || !user) {
      setError('Database name and username are required')
      return
    }
    // Delegate to parent; dialog closes and ImportProgress shows status
    onPush(host, port, dbName, user, password, sslMode)
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
    <div className="modal-overlay" onClick={onClose}>
      <div className="pg-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="pg-header">
          <h2>{mode === 'push' ? 'Push to PostgreSQL' : 'Connect to PostgreSQL'}</h2>
          <button className="modal-close" onClick={onClose}>x</button>
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
          {error && <div className="pg-error">{error}</div>}
          <div className="pg-actions">
            {mode === 'push' ? (
              <button onClick={handlePush} disabled={loading}>
                Push Data
              </button>
            ) : (
              <>
                <button onClick={handleConnect} disabled={loading}>
                  {loading ? 'Connecting...' : 'Connect'}
                </button>
                <button onClick={handleCreate} disabled={loading}>
                  {loading ? 'Creating...' : 'Create & Connect'}
                </button>
              </>
            )}
            <button className="pg-cancel" onClick={onClose} disabled={loading}>
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default PostgresDialog
