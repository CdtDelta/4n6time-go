// OpenFailureDialog explains a database open, connect, or create that failed
// for any reason other than read-only storage (ReadOnlyDatabaseDialog covers
// that). failure is the main.OpenFailure from GetOpenFailure: the backend
// writes the plain-language message, so the wording is tested in Go.
const TITLES = {
  open: 'Database Not Opened',
  connect: 'Could Not Connect',
  create: 'Could Not Create Database',
}

export default function OpenFailureDialog({ failure, onClose }) {
  if (!failure) return null

  const targetLabel = failure.backend === 'postgres' ? 'Connection' : 'File'

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="recursive-summary-dialog" onClick={e => e.stopPropagation()}>
        <div className="recursive-summary-header">
          <h2>{TITLES[failure.action] || TITLES.open}</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="recursive-summary-content">
          <p><strong>{targetLabel}:</strong> <span className="import-failure-file">{failure.target}</span></p>
          <p>{failure.message}</p>
          <p><strong>Error:</strong></p>
          <div className="import-failure-error">{failure.error}</div>
          <p>
            {failure.stillOpen
              ? <>Still open: <span className="import-failure-file">{failure.stillOpen}</span></>
              : 'No database is open.'}
          </p>
        </div>
        <div className="recursive-summary-footer">
          <button className="recursive-close-btn" onClick={onClose}>OK</button>
        </div>
      </div>
    </div>
  )
}
