import themes from '../themes'

function ThemePicker({ visible, currentTheme, onSelect, onClose }) {
  if (!visible) return null

  return (
    <div className="theme-picker-overlay" onClick={onClose}>
      <div className="theme-picker" onClick={(e) => e.stopPropagation()}>
        <div className="tp-header">
          <span className="tp-title">Theme</span>
          <button className="tp-close" onClick={onClose}>x</button>
        </div>
        <div className="tp-list">
          {Object.entries(themes).map(([id, theme]) => (
            <div
              key={id}
              className={`tp-item ${currentTheme === id ? 'active' : ''}`}
              onClick={() => onSelect(id)}
            >
              <div className="tp-swatch">
                <span style={{ background: theme.vars['--bg-primary'] }} />
                <span style={{ background: theme.vars['--bg-secondary'] }} />
                <span style={{ background: theme.vars['--bg-accent-active'] }} />
                <span style={{ background: theme.vars['--text-primary'], width: '2px' }} />
              </div>
              <span className="tp-name">{theme.name}</span>
              {currentTheme === id && <span className="tp-check">*</span>}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export default ThemePicker
