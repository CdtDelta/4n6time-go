import { useState, useEffect } from 'react'

// OffsetUpgradeDialog offers to widen the offset column of a PostgreSQL
// database created before v0.18.1, whose INT column overflows on the byte
// offsets Plaso reports for large evidence files.
//
// request is null (hidden) or { context, run }:
//   context 'connect': shown after connecting; buttons Upgrade now and Later.
//   context 'import' or 'push': shown before the operation; buttons Upgrade
//   now, Continue without upgrading, and Cancel.
//   run is an async function that performs the upgrade.
// onResolve receives 'upgraded', 'failed', 'later', 'continue', or 'cancel'.
// onRunningChange(true/false) brackets the upgrade so the parent can block
// other database actions, including native menu items.
export default function OffsetUpgradeDialog({ request, onResolve, onRunningChange }) {
  const [phase, setPhase] = useState('ask')
  const [error, setError] = useState('')

  // Each new request starts at the question.
  useEffect(() => {
    setPhase('ask')
    setError('')
  }, [request])

  if (!request) return null

  const beforeOperation = request.context !== 'connect'
  const subject = request.context === 'push'
    ? 'The PostgreSQL database you are pushing to'
    : 'This PostgreSQL database'

  const runUpgrade = async () => {
    setPhase('running')
    onRunningChange(true)
    try {
      await request.run()
      setPhase('done')
    } catch (err) {
      setError(String(err))
      setPhase('failed')
    } finally {
      onRunningChange(false)
    }
  }

  // While the upgrade runs, the overlay covers the whole window and has no
  // way out: the table is locked until PostgreSQL finishes.
  if (phase === 'running') {
    return (
      <div className="import-overlay">
        <div className="import-modal">
          <h3>Upgrading Database</h3>
          <div className="progress-bar-container">
            <div className="progress-bar indeterminate" />
          </div>
          <p className="import-message">
            Rewriting the event table so the offset column can hold large values.
            This can take a long time on a large database. Do not close 4n6time.
          </p>
        </div>
      </div>
    )
  }

  if (phase === 'done' || phase === 'failed') {
    const ok = phase === 'done'
    const resolve = () => onResolve(ok ? 'upgraded' : 'failed')
    let message
    if (ok) {
      message = beforeOperation
        ? 'The offset column now holds large values. Select OK to continue.'
        : 'The offset column now holds large values.'
    } else {
      // No claim that nothing changed: a failed ANALYZE follows a completed ALTER.
      message = beforeOperation
        ? 'The upgrade did not complete, so the operation was cancelled.'
        : 'The upgrade did not complete.'
    }
    return (
      <div className="modal-overlay">
        <div className="recursive-summary-dialog" onClick={e => e.stopPropagation()}>
          <div className="recursive-summary-header">
            <h2>{ok ? 'Upgrade Complete' : 'Upgrade Failed'}</h2>
          </div>
          <div className="recursive-summary-content">
            <p>{message}</p>
            {!ok && <p className="offset-upgrade-error">{error}</p>}
            <div className="add-note-actions">
              <button onClick={resolve}>OK</button>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="modal-overlay">
      <div className="recursive-summary-dialog" onClick={e => e.stopPropagation()}>
        <div className="recursive-summary-header">
          <h2>Database Upgrade Recommended</h2>
        </div>
        <div className="recursive-summary-content">
          <p>
            {subject} was created by an older version of 4n6time. Its offset
            column can't store values above 2,147,483,647, so imports of large
            evidence files can fail.
          </p>
          <p>
            The upgrade rewrites the event table. On a large database this can
            take a long time, the database is unavailable while it runs, and the
            PostgreSQL server needs free disk space roughly equal to the
            table's size.
          </p>
          <div className="add-note-actions">
            {beforeOperation ? (
              <>
                <button className="add-note-cancel" onClick={() => onResolve('cancel')}>Cancel</button>
                <button className="add-note-cancel" onClick={() => onResolve('continue')}>Continue without upgrading</button>
              </>
            ) : (
              <button className="add-note-cancel" onClick={() => onResolve('later')}>Later</button>
            )}
            <button onClick={runUpgrade}>Upgrade now</button>
          </div>
        </div>
      </div>
    </div>
  )
}
