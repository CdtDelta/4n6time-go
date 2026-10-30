// ImportFailureDialog reports a failed single-file import. failure is the
// main.ImportFailure from GetLastImportFailure: the file, the error text, and
// an outcome sentence (built in Go) saying what the failure left in the
// database. The status bar still shows the error; this dialog stays until
// dismissed, so a later grid load cannot hide it.
export default function ImportFailureDialog({ failure, onClose, onShowImports }) {
  if (!failure) return null

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="recursive-summary-dialog" onClick={e => e.stopPropagation()}>
        <div className="recursive-summary-header">
          <h2>Import Failed</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="recursive-summary-content">
          <p><strong>File:</strong> <span className="import-failure-file">{failure.file}</span></p>
          <p>{failure.outcome}</p>
          <div className="recursive-summary-section">
            <h3>Error</h3>
            <div className="import-failure-error">{failure.error}</div>
          </div>
          {failure.inImportsList && (
            <p className="import-failure-hint">
              File &gt; Imports... lists every import with its status and error.
            </p>
          )}
        </div>
        <div className="recursive-summary-footer">
          {failure.inImportsList && onShowImports && (
            <button className="recursive-close-btn import-failure-show" onClick={onShowImports}>Show Imports</button>
          )}
          <button className="recursive-close-btn" onClick={onClose}>OK</button>
        </div>
      </div>
    </div>
  )
}
