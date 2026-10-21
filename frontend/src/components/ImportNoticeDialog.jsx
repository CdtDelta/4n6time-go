// ImportNoticeDialog tells the examiner that a single-file import skipped
// timestamp values. The notice holds the file's base name, the number of
// events imported, and the number of unparseable timestamp values skipped.
// The parent sets it only when that skipped count is greater than zero.
export default function ImportNoticeDialog({ notice, onClose }) {
  if (!notice) return null

  const imported = (notice.imported || 0).toLocaleString()
  const skipped = (notice.unparseable || 0).toLocaleString()
  // With logging off, the examples were never written, so say how to get them.
  const hint = notice.logEnabled
    ? 'See log for details.'
    : 'Enable Help > Logging and re-import to record which values were skipped.'
  const message = `Imported ${imported} events from ${notice.file}; ${skipped} unparseable timestamp value(s) skipped. ${hint}`

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="recursive-summary-dialog" onClick={e => e.stopPropagation()}>
        <div className="recursive-summary-header">
          <h2>Import Complete</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="recursive-summary-content">
          <p>{message}</p>
        </div>
        <div className="recursive-summary-header">
          <span />
          <button onClick={onClose}>OK</button>
        </div>
      </div>
    </div>
  )
}
