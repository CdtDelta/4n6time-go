function AboutDialog({ visible, version, onClose }) {
  if (!visible) return null

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="about-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="about-header">
          <h2>4n6time</h2>
          <button className="modal-close" onClick={onClose}>x</button>
        </div>
        <div className="about-content">
          <p className="about-version">Version {version || 'unknown'}</p>
          <p className="about-desc">Forensic Timeline Analysis Tool</p>
          <div className="about-credits">
            <p>Written by Tom Yarrish</p>
            <p className="about-original">Based on the original 4n6time by David Nides</p>
          </div>
          <div className="about-links">
            <p>github.com/CdtDelta/4n6time-go</p>
          </div>
        </div>
      </div>
    </div>
  )
}

export default AboutDialog
