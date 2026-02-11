import { useState, useEffect, useCallback } from 'react'

function ColumnChooser({ visible, columns, onToggle, onClose }) {
  if (!visible) return null

  // Group columns: visible first, hidden second
  const visibleCols = columns.filter(c => !c.hide)
  const hiddenCols = columns.filter(c => c.hide)

  return (
    <div className="column-chooser-overlay" onClick={onClose}>
      <div className="column-chooser" onClick={(e) => e.stopPropagation()}>
        <div className="cc-header">
          <span className="cc-title">Columns</span>
          <button className="cc-close" onClick={onClose}>x</button>
        </div>
        <div className="cc-list">
          <div className="cc-group-label">Visible</div>
          {visibleCols.map(col => (
            <label key={col.field} className="cc-item">
              <input
                type="checkbox"
                checked={true}
                onChange={() => onToggle(col.field)}
              />
              <span>{col.headerName}</span>
            </label>
          ))}
          {hiddenCols.length > 0 && (
            <>
              <div className="cc-group-label">Hidden</div>
              {hiddenCols.map(col => (
                <label key={col.field} className="cc-item">
                  <input
                    type="checkbox"
                    checked={false}
                    onChange={() => onToggle(col.field)}
                  />
                  <span>{col.headerName}</span>
                </label>
              ))}
            </>
          )}
        </div>
      </div>
    </div>
  )
}

export default ColumnChooser
