import { useState, useEffect } from 'react'

// Wails runtime for event listening
const EventsOn = window.runtime?.EventsOn || function() { return () => {} }

function ImportProgress({ visible }) {
  const [progress, setProgress] = useState({
    phase: '',
    message: 'Starting import...',
    count: 0,
    total: 0,
  })

  useEffect(() => {
    // Listen for import:progress events from the Go backend
    const cancel = EventsOn('import:progress', (data) => {
      setProgress(data)
    })
    return () => {
      if (typeof cancel === 'function') cancel()
    }
  }, [])

  if (!visible) return null

  const percent = progress.total > 0
    ? Math.round((progress.count / progress.total) * 100)
    : null

  return (
    <div className="import-overlay">
      <div className="import-modal">
        <h3>Importing Timeline Data</h3>

        <div className="import-phase">
          <span className={`phase-dot ${progress.phase === 'reading' ? 'active' : progress.phase !== 'reading' && progress.count > 0 ? 'done' : ''}`} />
          <span>Read CSV</span>
        </div>
        <div className="import-phase">
          <span className={`phase-dot ${progress.phase === 'inserting' ? 'active' : progress.phase === 'metadata' || progress.phase === 'done' ? 'done' : ''}`} />
          <span>Insert into database</span>
        </div>
        <div className="import-phase">
          <span className={`phase-dot ${progress.phase === 'metadata' ? 'active' : progress.phase === 'done' ? 'done' : ''}`} />
          <span>Build metadata</span>
        </div>

        {percent !== null && (
          <div className="progress-bar-container">
            <div className="progress-bar" style={{ width: `${percent}%` }} />
          </div>
        )}

        <p className="import-message">{progress.message}</p>

        {percent !== null && (
          <p className="import-percent">{percent}%</p>
        )}
      </div>
    </div>
  )
}

export default ImportProgress
