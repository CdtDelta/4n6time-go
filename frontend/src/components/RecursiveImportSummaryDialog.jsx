import { useEffect, useState } from 'react'
import { GetLoggingStatus } from '../../wailsjs/go/main/App'

const OTHER_FAMILY = 'Other'

// groupByFamily buckets per-tool stats under their tool family, with a
// subtotal per family. Tools without a family entry land under "Other",
// which sorts last; families and tools otherwise sort alphabetically.
function groupByFamily(perTool, toolFamilies) {
  const groups = {}
  for (const [tool, stats] of Object.entries(perTool)) {
    const family = toolFamilies[tool] || OTHER_FAMILY
    if (!groups[family]) {
      groups[family] = { family, tools: [], fileCount: 0, eventCount: 0, unparseableCount: 0 }
    }
    const g = groups[family]
    g.tools.push([tool, stats])
    g.fileCount += stats.fileCount || 0
    g.eventCount += stats.eventCount || 0
    g.unparseableCount += stats.unparseableCount || 0
  }
  for (const g of Object.values(groups)) {
    g.tools.sort(([a], [b]) => a.localeCompare(b))
  }
  return Object.values(groups).sort((a, b) => {
    if (a.family === OTHER_FAMILY) return 1
    if (b.family === OTHER_FAMILY) return -1
    return a.family.localeCompare(b.family)
  })
}

// unparseableNote returns the suffix shown after an event count, or nothing
// when the count is zero, so clean imports render exactly as before.
function unparseableNote(count) {
  if (!count) return null
  return ` (${count.toLocaleString()} unparseable)`
}

export default function RecursiveImportSummaryDialog({ visible, summary, onClose }) {
  const [skippedExpanded, setSkippedExpanded] = useState(false)
  const [unparseableExpanded, setUnparseableExpanded] = useState(false)
  // loggingOff is set only after GetLoggingStatus answers with logging disabled.
  // It stays false while the call is pending or if the call fails.
  const [loggingOff, setLoggingOff] = useState(false)

  const unparseableFiles = (summary && summary.unparseableFiles) || []
  const hasUnparseable = unparseableFiles.length > 0

  // Ask for the logging state once when the dialog opens with unparseable
  // files to report. Clean imports make no call.
  useEffect(() => {
    if (!visible || !summary || !hasUnparseable) return
    let cancelled = false
    const check = async () => {
      try {
        const status = await GetLoggingStatus()
        if (!cancelled) setLoggingOff(!status.enabled)
      } catch {
        if (!cancelled) setLoggingOff(false)
      }
    }
    check()
    return () => { cancelled = true }
  }, [visible, summary, hasUnparseable])

  if (!visible || !summary) return null

  const perTool = summary.perTool || {}
  const skipped = summary.skippedFiles || []
  const familyGroups = groupByFamily(perTool, summary.toolFamilies || {})

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
                {familyGroups.map(group => (
                  <tbody key={group.family}>
                    <tr>
                      <td colSpan={3} style={{ fontWeight: 'bold' }}>{group.family}</td>
                    </tr>
                    {group.tools.map(([tool, stats]) => (
                      <tr key={tool}>
                        <td style={{ paddingLeft: '1.5em' }}>{tool}</td>
                        <td>{stats.fileCount}</td>
                        <td>{(stats.eventCount || 0).toLocaleString()}{unparseableNote(stats.unparseableCount)}</td>
                      </tr>
                    ))}
                    <tr style={{ fontStyle: 'italic' }}>
                      <td style={{ paddingLeft: '1.5em' }}>{group.family} subtotal</td>
                      <td>{group.fileCount}</td>
                      <td>{group.eventCount.toLocaleString()}{unparseableNote(group.unparseableCount)}</td>
                    </tr>
                  </tbody>
                ))}
              </table>
            </div>
          )}

          {unparseableFiles.length > 0 && (
            <div className="recursive-summary-section">
              <button
                className="recursive-skipped-toggle"
                onClick={() => setUnparseableExpanded(prev => !prev)}
              >
                {unparseableExpanded ? '-' : '+'} Files with unparseable timestamps ({unparseableFiles.length})
              </button>
              {unparseableExpanded && (
                <table className="recursive-summary-table recursive-skipped-table">
                  <thead>
                    <tr>
                      <th>File</th>
                      <th>Unparseable</th>
                    </tr>
                  </thead>
                  <tbody>
                    {unparseableFiles.map((uf, i) => (
                      <tr key={i}>
                        <td className="recursive-skipped-path">{uf.relativePath}</td>
                        <td>{(uf.count || 0).toLocaleString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
              {unparseableExpanded && loggingOff && (
                <p style={{ fontStyle: 'italic' }}>
                  Enable Help &gt; Logging and re-import to record which values were skipped.
                </p>
              )}
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
