import { useState, useEffect, useRef } from 'react'

function TabBar({
  tabs,
  activeTabId,
  onTabClick,
  onTabClose,
  isActiveStale,
  onRefresh,
  onSaveTab,
}) {
  const [saveTabId, setSaveTabId] = useState(null)
  const [saveName, setSaveName] = useState('')
  const saveInputRef = useRef(null)

  useEffect(() => {
    if (saveTabId && saveInputRef.current) {
      saveInputRef.current.focus()
    }
  }, [saveTabId])

  const handleSaveClick = (e, tabId) => {
    e.stopPropagation()
    setSaveTabId(tabId)
    setSaveName('')
  }

  const handleSaveConfirm = (e, tabId) => {
    e.stopPropagation()
    const name = saveName.trim()
    if (name && onSaveTab) {
      onSaveTab(tabId, name)
    }
    setSaveTabId(null)
    setSaveName('')
  }

  const handleSaveCancel = (e) => {
    e.stopPropagation()
    setSaveTabId(null)
    setSaveName('')
  }

  const handleSaveKeyDown = (e, tabId) => {
    if (e.key === 'Enter') {
      e.stopPropagation()
      handleSaveConfirm(e, tabId)
    } else if (e.key === 'Escape') {
      handleSaveCancel(e)
    }
  }

  return (
    <div className="tab-bar">
      <div className="tab-bar-tabs">
        {tabs.map(tab => (
          <div
            key={tab.id}
            className={`tab ${tab.id === activeTabId ? 'active' : ''}`}
            onClick={() => onTabClick(tab.id)}
            title={tab.label}
          >
            <span className="tab-label">{tab.label}</span>
            {tab.stale && (
              <span
                className="tab-stale-dot"
                title="Data changed in another tab. Click Refresh to reload."
              />
            )}

            {/* Inline save form shown in place of save button */}
            {!tab.isMain && saveTabId === tab.id && (
              <div className="tab-save-form" onClick={e => e.stopPropagation()}>
                <input
                  ref={saveInputRef}
                  type="text"
                  className="tab-save-input"
                  placeholder="Query name..."
                  value={saveName}
                  onChange={e => setSaveName(e.target.value)}
                  onKeyDown={e => handleSaveKeyDown(e, tab.id)}
                />
                <button
                  className="tab-save-confirm"
                  onClick={e => handleSaveConfirm(e, tab.id)}
                  title="Save query"
                >✓</button>
                <button
                  className="tab-save-cancel"
                  onClick={handleSaveCancel}
                  title="Cancel"
                >✕</button>
              </div>
            )}

            {!tab.isMain && saveTabId !== tab.id && (
              <button
                className="tab-save"
                title="Save tab query"
                onClick={e => handleSaveClick(e, tab.id)}
              >⊙</button>
            )}

            {!tab.isMain && (
              <button
                className="tab-close"
                title="Close tab"
                onClick={(e) => {
                  e.stopPropagation()
                  onTabClose(tab.id)
                }}
              >
                ×
              </button>
            )}
          </div>
        ))}
      </div>

      <div className="tab-bar-actions">
        {isActiveStale && (
          <button
            className="tab-refresh-btn"
            onClick={onRefresh}
            title="Reload this tab (data changed in another tab)"
          >
            ↻
          </button>
        )}
      </div>
    </div>
  )
}

export default TabBar
