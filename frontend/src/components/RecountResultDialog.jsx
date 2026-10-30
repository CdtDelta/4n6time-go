// RecountResultDialog confirms Tools > Recount Events. result is the
// main.RecountResult: the live count now stored, and the cached value it
// replaced (if there was one).
export default function RecountResultDialog({ result, onClose }) {
  if (!result) return null

  const count = (result.count || 0).toLocaleString()
  let detail
  if (!result.hadPrevious) {
    detail = 'There was no cached count before.'
  } else if (result.previous === result.count) {
    detail = 'The cached count was already correct.'
  } else {
    detail = `The cached count was ${(result.previous || 0).toLocaleString()}.`
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="recursive-summary-dialog" onClick={e => e.stopPropagation()}>
        <div className="recursive-summary-header">
          <h2>Recount Events</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="recursive-summary-content">
          <p>The database holds {count} events (examiner notes not included). {detail}</p>
        </div>
        <div className="recursive-summary-footer">
          <button className="recursive-close-btn" onClick={onClose}>OK</button>
        </div>
      </div>
    </div>
  )
}
