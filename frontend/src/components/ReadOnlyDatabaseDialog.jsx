// ReadOnlyDatabaseDialog explains why a database did not open: it was created
// by an older version of 4n6time, needs a one-time upgrade, and is on
// read-only storage, so the upgrade could not run. failure is the
// main.ReadOnlyOpenFailure from GetReadOnlyOpenFailure.
export default function ReadOnlyDatabaseDialog({ failure, onClose }) {
  if (!failure) return null

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="recursive-summary-dialog" onClick={e => e.stopPropagation()}>
        <div className="recursive-summary-header">
          <h2>Database Is Read-Only</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="recursive-summary-content">
          <p><strong>File:</strong> <span className="import-failure-file">{failure.path}</span></p>
          <p>
            This database is on read-only storage. It was created by an older
            version of 4n6time and needs a one-time upgrade before it can be
            opened, and the upgrade cannot be written to read-only storage.
          </p>
          <p>
            To open it, copy the database file to a writable location and
            open the copy.
          </p>
          <p>The original file was not changed.</p>
        </div>
        <div className="recursive-summary-footer">
          <button className="recursive-close-btn" onClick={onClose}>OK</button>
        </div>
      </div>
    </div>
  )
}
