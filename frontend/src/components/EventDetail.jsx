import { useState, useEffect, useCallback } from 'react'
import { UpdateEventFields } from '../../wailsjs/go/main/App'
import HighlightText from './HighlightText'

// Field groups for organized display
const fieldGroups = [
  {
    label: 'Timeline',
    fields: [
      { key: 'datetime', label: 'Date/Time' },
      { key: 'timezone', label: 'Timezone' },
      { key: 'macb', label: 'MACB' },
      { key: 'type', label: 'Type' },
    ],
  },
  {
    label: 'Source',
    fields: [
      { key: 'source', label: 'Source' },
      { key: 'sourcetype', label: 'Source Type' },
      { key: 'format', label: 'Format' },
      { key: 'source_name', label: 'Source Name' },
    ],
  },
  {
    label: 'System',
    fields: [
      { key: 'host', label: 'Host' },
      { key: 'user', label: 'User' },
      { key: 'computer_name', label: 'Computer' },
      { key: 'user_sid', label: 'User SID' },
    ],
  },
  {
    label: 'File',
    fields: [
      { key: 'filename', label: 'Filename' },
      { key: 'inode', label: 'Inode' },
      { key: 'url', label: 'URL' },
    ],
  },
  {
    label: 'Event',
    fields: [
      { key: 'event_identifier', label: 'Event ID' },
      { key: 'event_type', label: 'Event Type' },
      { key: 'record_number', label: 'Record #' },
      { key: 'offset', label: 'Offset' },
    ],
  },
]

// Colors available for marking events
const colorOptions = [
  '', 'RED', 'ORANGE', 'YELLOW', 'GREEN', 'BLUE', 'PURPLE', 'WHITE', 'BLACK',
]

const colorDisplayMap = {
  '': 'None',
  'RED': '#e74c3c',
  'ORANGE': '#e67e22',
  'YELLOW': '#f1c40f',
  'GREEN': '#2ecc71',
  'BLUE': '#3498db',
  'PURPLE': '#9b59b6',
  'WHITE': '#ecf0f1',
  'BLACK': '#2c3e50',
}

function EventDetail({ event, onUpdate, onClose, height, searchText, onToggleBookmark }) {
  const [tag, setTag] = useState('')
  const [color, setColor] = useState('')
  const [notes, setNotes] = useState('')
  const [reportNotes, setReportNotes] = useState('')
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)

  // Sync state when a new event is selected
  useEffect(() => {
    if (event) {
      setTag(event.tag || '')
      setColor(event.color || '')
      setNotes(event.notes || '')
      setReportNotes(event.reportnotes || '')
      setDirty(false)
    }
  }, [event?.id])

  const handleSave = useCallback(async () => {
    if (!event || !dirty) return

    setSaving(true)
    try {
      const fields = {
        tag: tag,
        color: color,
        notes: notes,
        reportnotes: reportNotes,
      }
      await UpdateEventFields(event.id, fields)
      setDirty(false)
      if (onUpdate) onUpdate(event.id, fields)
    } catch (err) {
      console.error('Error saving event:', err)
    } finally {
      setSaving(false)
    }
  }, [event, tag, color, notes, reportNotes, dirty, onUpdate])

  const handleKeyDown = useCallback((e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      handleSave()
    }
  }, [handleSave])

  if (!event) return null

  return (
    <div className="detail-panel" onKeyDown={handleKeyDown} style={{ height: height || 280 }}>
      <div className="detail-header">
        <span className="detail-title">Event Detail</span>
        <button
          className={`detail-bookmark ${event.bookmark ? 'active' : ''}`}
          onClick={() => onToggleBookmark && onToggleBookmark(event.id)}
          title={event.bookmark ? 'Remove bookmark' : 'Bookmark this event'}
        >
          {event.bookmark ? '\u2605' : '\u2606'}
        </button>
        <span className="detail-id">ID: {event.id}</span>
        <button className="detail-close" onClick={onClose}>x</button>
      </div>

      <div className="detail-body">
        {/* Description (full width) */}
        <div className="detail-desc-section">
          <label>Description</label>
          <div className="detail-desc-text">
            {event.desc ? <HighlightText text={event.desc} search={searchText} /> : '(empty)'}
          </div>
        </div>

        {/* Extra info (full width) */}
        {event.extra && (
          <div className="detail-desc-section">
            <label>Extra</label>
            <div className="detail-desc-text">
              <HighlightText text={event.extra} search={searchText} />
            </div>
          </div>
        )}

        {/* Field groups in columns */}
        <div className="detail-fields">
          {fieldGroups.map(group => (
            <div key={group.label} className="detail-group">
              <div className="detail-group-label">{group.label}</div>
              {group.fields.map(f => {
                const val = event[f.key]
                if (val === undefined || val === null || val === '' || val === -1) return null
                return (
                  <div key={f.key} className="detail-field">
                    <span className="detail-field-label">{f.label}</span>
                    <span className="detail-field-value">
                      <HighlightText text={String(val)} search={searchText} />
                    </span>
                  </div>
                )
              })}
            </div>
          ))}
        </div>

        {/* Editable fields */}
        <div className="detail-editable">
          <div className="detail-edit-row">
            <label>Tag</label>
            <input
              type="text"
              value={tag}
              placeholder="e.g. malware, lateral-movement"
              onChange={(e) => { setTag(e.target.value); setDirty(true) }}
            />
          </div>

          <div className="detail-edit-row">
            <label>Color</label>
            <div className="color-picker">
              {colorOptions.map(c => (
                <button
                  key={c || 'none'}
                  className={`color-swatch ${color === c ? 'selected' : ''}`}
                  style={{
                    background: colorDisplayMap[c] || 'transparent',
                    border: c === '' ? '1px dashed #808080' : '1px solid transparent',
                  }}
                  title={c || 'None'}
                  onClick={() => { setColor(c); setDirty(true) }}
                />
              ))}
            </div>
          </div>

          <div className="detail-edit-row">
            <label>Notes</label>
            <textarea
              value={notes}
              rows={2}
              placeholder="Investigator notes..."
              onChange={(e) => { setNotes(e.target.value); setDirty(true) }}
            />
          </div>

          <div className="detail-edit-row">
            <label>Report Notes</label>
            <textarea
              value={reportNotes}
              rows={2}
              placeholder="Notes for report..."
              onChange={(e) => { setReportNotes(e.target.value); setDirty(true) }}
            />
          </div>

          {dirty && (
            <div className="detail-save-bar">
              <button onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : 'Save Changes (Ctrl+Enter)'}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default EventDetail
