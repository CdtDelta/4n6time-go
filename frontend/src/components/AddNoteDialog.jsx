import { useState } from 'react'
import { AddExaminerNote } from '../../wailsjs/go/main/App'

function AddNoteDialog({ visible, onClose, onAdded }) {
  const [datetime, setDatetime] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  if (!visible) return null

  const handleSubmit = async () => {
    if (!datetime.trim()) {
      setError('Date/time is required')
      return
    }
    if (!description.trim()) {
      setError('Description is required')
      return
    }

    setSaving(true)
    setError('')
    try {
      await AddExaminerNote(datetime.trim(), description.trim())
      setDatetime('')
      setDescription('')
      if (onAdded) onAdded()
      onClose()
    } catch (err) {
      setError('Error adding note: ' + err)
    } finally {
      setSaving(false)
    }
  }

  const handleCancel = () => {
    setDatetime('')
    setDescription('')
    setError('')
    onClose()
  }

  return (
    <div className="modal-overlay" onClick={handleCancel}>
      <div className="add-note-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="add-note-header">
          <h2>Add Examiner Note</h2>
          <button className="modal-close" onClick={handleCancel}>x</button>
        </div>
        <div className="add-note-content">
          <div className="add-note-field">
            <label>Date/Time</label>
            <input
              type="text"
              value={datetime}
              onChange={(e) => setDatetime(e.target.value)}
              placeholder="YYYY-MM-DD HH:MM:SS"
            />
          </div>
          <div className="add-note-field">
            <label>Description</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Examiner note..."
              rows={4}
            />
          </div>
          {error && <div className="add-note-error">{error}</div>}
          <div className="add-note-actions">
            <button className="add-note-cancel" onClick={handleCancel}>Cancel</button>
            <button onClick={handleSubmit} disabled={saving}>
              {saving ? 'Adding...' : 'Add Note'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default AddNoteDialog
