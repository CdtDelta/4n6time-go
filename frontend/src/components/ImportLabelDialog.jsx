import { useState, useEffect, useRef } from 'react'

// ImportLabelDialog asks for an optional examiner label after the file or
// folder is chosen and before the import starts. The field is pre-filled with
// defaultLabel and may be left empty. Enter imports; Escape or Cancel aborts.
// It reuses the add-note dialog classes so it follows the active theme.
function ImportLabelDialog({ visible, title, defaultLabel, onImport, onCancel }) {
  const [label, setLabel] = useState(defaultLabel || '')
  const inputRef = useRef(null)

  useEffect(() => {
    if (visible) {
      setLabel(defaultLabel || '')
      if (inputRef.current) inputRef.current.select()
    }
  }, [visible, defaultLabel])

  if (!visible) return null

  const submit = () => onImport(label.trim())

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      submit()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onCancel()
    }
  }

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="add-note-dialog" onClick={(e) => e.stopPropagation()} onKeyDown={handleKeyDown}>
        <div className="add-note-header">
          <h2>{title}</h2>
          <button className="modal-close" onClick={onCancel}>x</button>
        </div>
        <div className="add-note-content">
          <div className="add-note-field">
            <label>Label (optional, e.g. hostname)</label>
            <input
              ref={inputRef}
              type="text"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              autoFocus
            />
          </div>
          <div className="add-note-actions">
            <button className="add-note-cancel" onClick={onCancel}>Cancel</button>
            <button onClick={submit}>Import</button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default ImportLabelDialog
