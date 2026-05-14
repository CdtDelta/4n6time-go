import { useState } from 'react'

export default function RecursiveImportSummaryDialog({ visible, summary, onClose }) {
  const [skippedExpanded, setSkippedExpanded] = useState(false)

  if (!visible || !summary) return null

  const perTool = summary.perTool || {}
  const skipped = summary.skippedFiles || []

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="recursive-summary-dialog" onClick={e => e.stopPropagation()}>
        <div className="recursive-summary-header">
          <h2>Import Complete</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>

        <div className="recursive-summary-content">
          <div className="recursive-summary-stats">
            <div className="recursive-stat">
              <span className="recursive-stat-value">{(summary.totalEvents || 0).toLocaleString()}</span>
              <span className="recursive-stat-label">Events imported</span>
            </div>
            <div className="recursive-stat">
              <span className="recursive-stat-value">{summary.totalFilesProcessed || 0}</span>
              <span className="recursive-stat-label">Files processed</span>
            </div>
            <div className="recursive-stat">
              <span className="recursive-stat-value">{summary.directoriesWalked || 0}</span>
              <span className="recursive-stat-label">Directories walked</span>
            </div>
            <div className="recursive-stat">
              <span className="recursive-stat-value">{summary.maxDepthReached || 0}</span>
              <span className="recursive-stat-label">Max depth reached (of 3)</span>
            </div>
          </div>

          {Object.keys(perTool).length > 0 && (
            <div className="recursive-summary-section">
              <h3>Per-Tool Breakdown</h3>
              <table className="recursive-summary-table">
                <thead>
                  <tr>
                    <th>Tool</th>
                    <th>Files</th>
                    <th>Events</th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(perTool).map(([tool, stats]) => (
                    <tr key={tool}>
                      <td>{tool}</td>
                      <td>{stats.fileCount}</td>
                      <td>{(stats.eventCount || 0).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {skipped.length > 0 && (
            <div className="recursive-summary-section">
              <button
                className="recursive-skipped-toggle"
                onClick={() => setSkippedExpanded(prev => !prev)}
              >
                {skippedExpanded ? '-' : '+'} Skipped Files ({skipped.length})
              </button>
              {skippedExpanded && (
                <table className="recursive-summary-table recursive-skipped-table">
                  <thead>
                    <tr>
                      <th>File</th>
                      <th>Reason</th>
                    </tr>
                  </thead>
                  <tbody>
                    {skipped.map((sf, i) => (
                      <tr key={i}>
                        <td className="recursive-skipped-path">{sf.relativePath}</td>
                        <td>{sf.reason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}
        </div>

        <div className="recursive-summary-footer">
          <button className="recursive-close-btn" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  )
}
