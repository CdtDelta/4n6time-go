import { useState, useEffect, useCallback, useRef } from 'react'
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer,
  Brush, CartesianGrid, ReferenceArea,
} from 'recharts'
import { GetTimelineHistogram } from '../../wailsjs/go/main/App'
import { isSuperseded } from '../requestErrors'
import themes from '../themes'

const ADVANCED_MODE_TIP = 'Not applied in advanced search mode. Add conditions to your WHERE clause.'

// Examiner-note marker color per theme. Each is chosen to stand apart from that
// theme's --color-bar-fill and to read on its --bg-secondary; the marker's
// diamond shape carries the meaning when color alone would not.
const NOTE_MARKER_COLORS = {
  'forensic-dark': '#f0a500',
  'classic-dark': '#e5a33a',
  'high-contrast': '#ffff00',
  'light': '#e65100',
  'solarized-dark': '#cb4b16',
  'monokai': '#fd971f',
  'dracula': '#ffb86c',
  'nord': '#d08770',
  'gruvbox': '#83a598',
  'matrix': '#ffffff',
  'forensic-blue': '#f0ad4e',
}
const NOTE_MARKER_DEFAULT = '#f0a500'

// Height of the examiner-notes lane below the bars, in pixels. The chart grows
// by this much so the bars keep their earlier height. It leaves room for the
// two-line lane label below the y-axis 0 tick, which is centered on the lane's
// top edge and reaches about 4px into the lane.
const LANE_HEIGHT = 24

// Lane label layout: font size, and the baseline of the second line above the
// lane's bottom edge. The first line sits one font size above it.
const LANE_LABEL_FONT = 9
const LANE_LABEL_BOTTOM = 2

// formatCounts is the tooltip text for one bucket: "1,204 events · 2 examiner
// notes", with the notes part left off when there are none.
function formatCounts(count, notesCount) {
  const events = `${count.toLocaleString()} ${count === 1 ? 'event' : 'events'}`
  if (!notesCount) return events
  return `${events} · ${notesCount.toLocaleString()} examiner ${notesCount === 1 ? 'note' : 'notes'}`
}

function TimelineChart({ visible, filters, dbInfo, onSelectRange, theme, activeSearch, searchMode, bookmarkOnly, baseQuery, histogramSuppressRef, histogramVersion }) {
  const [data, setData] = useState([])
  const [loading, setLoading] = useState(false)
  const [refAreaLeft, setRefAreaLeft] = useState(null)
  const [refAreaRight, setRefAreaRight] = useState(null)
  const [selecting, setSelecting] = useState(false)
  // laneHover is the lane marker under the pointer: its bucket index and
  // position in chart coordinates. The lane sits outside the plot area, where
  // the Recharts tooltip does not fire, so it has its own tooltip.
  const [laneHover, setLaneHover] = useState(null)
  // Sequence number of the latest histogram load; older loads drop their
  // results. The backend also cancels the superseded query.
  const loadSeqRef = useRef(0)

  // Load histogram data when visible, filters change, or db changes.
  // histogramSuppressRef suppresses the fetch during user-initiated tab switches; histogramVersion
  // is bumped by the [activeTabId] effect once the switch completes and the flag is cleared.
  useEffect(() => {
    if (!visible || !dbInfo) return
    // histogramSuppressRef is intentionally not in deps: we read .current at call time.
    if (histogramSuppressRef?.current) return

    let cancelled = false
    const seq = ++loadSeqRef.current
    const isCurrent = () => !cancelled && seq === loadSeqRef.current
    setLoading(true)

    async function loadData() {
      try {
        const req = {
          filters: filters?.filters || [],
          logic: filters?.logic || 'AND',
          orderBy: 'datetime',
          page: 1,
          pageSize: 1000,
          searchText: activeSearch || '',
          searchMode: searchMode || 'simple',
          bookmarkOnly: bookmarkOnly || false,
          baseField: baseQuery?.field || '',
          baseOp: baseQuery?.op || '',
          baseValue: baseQuery?.value || '',
        }

        // The date range is sent as its own fields, ANDed outside the
        // filter group, as the grid sends it.
        if (filters?.dateFrom && filters?.dateTo) {
          req.dateFrom = filters.dateFrom
          req.dateTo = filters.dateTo
        }

        const result = await GetTimelineHistogram(req)
        if (isCurrent() && result) {
          setData(result.map(b => ({
            timestamp: b.timestamp,
            count: b.count,
            notesCount: b.notesCount || 0,
            // Short label for x-axis
            label: formatLabel(b.timestamp),
          })))
          setLaneHover(null)
        }
      } catch (err) {
        if (isCurrent() && !isSuperseded(err)) console.error('Error loading histogram:', err)
      } finally {
        if (isCurrent()) setLoading(false)
      }
    }

    loadData()
    return () => { cancelled = true }
  }, [visible, filters, dbInfo, activeSearch, searchMode, bookmarkOnly, baseQuery, histogramVersion])

  // Format timestamp for x-axis display
  const formatLabel = (ts) => {
    if (!ts) return ''
    // Monthly: "2024-01" -> "Jan 2024"
    if (ts.length === 7) {
      const [y, m] = ts.split('-')
      const months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']
      return `${months[parseInt(m) - 1]} ${y}`
    }
    // Daily: "2024-01-15" -> "Jan 15"
    if (ts.length === 10) {
      const [y, m, d] = ts.split('-')
      const months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']
      return `${months[parseInt(m) - 1]} ${parseInt(d)}`
    }
    // Hourly: "2024-01-15 14:00:00" -> "14:00"
    if (ts.length >= 16) {
      return ts.substring(11, 16)
    }
    return ts
  }

  const handleMouseDown = useCallback((e) => {
    if (e && e.activeLabel) {
      setRefAreaLeft(e.activeLabel)
      setSelecting(true)
    }
  }, [])

  const handleMouseMove = useCallback((e) => {
    if (selecting && e && e.activeLabel) {
      setRefAreaRight(e.activeLabel)
    }
  }, [selecting])

  const handleMouseUp = useCallback(() => {
    if (!selecting) {
      setRefAreaLeft(null)
      setRefAreaRight(null)
      return
    }

    setSelecting(false)

    // If no drag happened (single click), use refAreaLeft as a single bucket
    const isSingleClick = !refAreaRight || refAreaLeft === refAreaRight

    if (isSingleClick && refAreaLeft) {
      const idx = data.findIndex(d => d.label === refAreaLeft)
      if (idx !== -1 && onSelectRange) {
        const ts = data[idx].timestamp
        onSelectRange(ts, ts)
      }
      setRefAreaLeft(null)
      setRefAreaRight(null)
      return
    }

    if (!refAreaLeft || !refAreaRight) {
      setRefAreaLeft(null)
      setRefAreaRight(null)
      return
    }

    // Find the actual timestamps from the data
    const leftIdx = data.findIndex(d => d.label === refAreaLeft)
    const rightIdx = data.findIndex(d => d.label === refAreaRight)

    if (leftIdx === -1 || rightIdx === -1) {
      setRefAreaLeft(null)
      setRefAreaRight(null)
      return
    }

    const startIdx = Math.min(leftIdx, rightIdx)
    const endIdx = Math.max(leftIdx, rightIdx)

    const startTs = data[startIdx].timestamp
    const endTs = data[endIdx].timestamp

    setRefAreaLeft(null)
    setRefAreaRight(null)

    if (onSelectRange) {
      onSelectRange(startTs, endTs)
    }
  }, [selecting, refAreaLeft, refAreaRight, data, onSelectRange])

  const CustomTooltip = ({ active, payload, label }) => {
    if (!active || !payload || !payload.length) return null
    const bucket = payload[0].payload || {}
    return (
      <div className="timeline-tooltip">
        <div className="timeline-tooltip-label">{label}</div>
        <div className="timeline-tooltip-value">{formatCounts(bucket.count || 0, bucket.notesCount)}</div>
      </div>
    )
  }

  if (!visible) return null

  // Click-to-filter and drag-to-select set the filter panel date range, which
  // advanced mode does not apply, so they are off there. The chart still
  // renders and refetches for the active clause.
  const interactionsDisabled = searchMode === 'advanced'

  // Read theme colors directly from theme definitions
  const themeVars = themes[theme]?.vars || {}
  const barFill = themeVars['--color-bar-fill'] || '#533483'
  const borderColor = themeVars['--border-primary'] || '#0f3460'
  const mutedColor = themeVars['--text-muted'] || '#808080'
  const noteColor = NOTE_MARKER_COLORS[theme] || NOTE_MARKER_DEFAULT

  // A lane marker click does what a click on the bucket's bar does: set the
  // date range to that one bucket.
  const handleLaneClick = (idx) => {
    if (interactionsDisabled || !onSelectRange || !data[idx]) return
    const ts = data[idx].timestamp
    onSelectRange(ts, ts)
  }

  // renderLaneTick draws one lane cell. Recharts calls it once per bucket
  // (interval 0); x is the bucket's center and y the lane's top edge. Buckets
  // without notes draw nothing. The marker is a fixed-size diamond whatever the
  // count, so the lane shows where notes are, not how many.
  const renderLaneTick = ({ x, y, payload, index, visibleTicksCount }) => {
    const idx = payload?.index ?? index
    const bucket = data[idx]
    if (!bucket || !bucket.notesCount) return <g />
    const cy = y + LANE_HEIGHT / 2
    const r = 4.5
    return (
      <g
        onMouseEnter={() => setLaneHover({ idx, x, y: cy, flip: idx > visibleTicksCount / 2 })}
        onMouseLeave={() => setLaneHover(null)}
        onClick={interactionsDisabled ? undefined : () => handleLaneClick(idx)}
        style={{ cursor: interactionsDisabled ? 'not-allowed' : 'pointer' }}
      >
        <circle cx={x} cy={cy} r={r + 3} fill="transparent" />
        <path
          d={`M ${x} ${cy - r} L ${x + r} ${cy} L ${x} ${cy + r} L ${x - r} ${cy} Z`}
          fill={noteColor}
          stroke={noteColor}
          strokeWidth={1}
        />
      </g>
    )
  }

  // renderLaneLabel names the lane in the y-axis column to its left. The
  // lines are placed from the lane's bottom edge, so the first line's cap top
  // lands about 6px into the lane, below the 0 tick's baseline.
  const renderLaneLabel = ({ viewBox }) => {
    if (!viewBox) return null
    const lx = viewBox.x - 6
    const secondBaseline = viewBox.y + viewBox.height - LANE_LABEL_BOTTOM
    return (
      <text textAnchor="end" fill={mutedColor} fontSize={LANE_LABEL_FONT}>
        <tspan x={lx} y={secondBaseline - LANE_LABEL_FONT}>Examiner</tspan>
        <tspan x={lx} y={secondBaseline}>notes</tspan>
      </text>
    )
  }

  const hovered = laneHover && data[laneHover.idx]

  return (
    <div className="timeline-chart" title={interactionsDisabled ? ADVANCED_MODE_TIP : undefined}>
      <div className="timeline-header">
        <span className="timeline-title">Timeline</span>
        {loading && <span className="timeline-loading">Loading...</span>}
        <span className="timeline-hint">
          {interactionsDisabled ? 'Range selection not applied in advanced search mode' : 'Click and drag to select a time range'}
        </span>
      </div>
      <div className="timeline-body">
        {data.length === 0 && !loading ? (
          <div className="timeline-empty">No data to display</div>
        ) : (
          <div className="timeline-plot">
            <ResponsiveContainer width="100%" height={140 + LANE_HEIGHT}>
              <BarChart
                data={data}
                onMouseDown={interactionsDisabled ? undefined : handleMouseDown}
                onMouseMove={interactionsDisabled ? undefined : handleMouseMove}
                onMouseUp={interactionsDisabled ? undefined : handleMouseUp}
                style={interactionsDisabled ? { cursor: 'not-allowed' } : undefined}
              >
                <CartesianGrid strokeDasharray="3 3" stroke={borderColor} vertical={false} />
                {/* The notes lane is a second category x-axis over the same
                    buckets, so its cells line up with the bars. Recharts stacks
                    bottom axes by id with numeric ids first, so the lane keeps
                    the default id 0 (which the bars and ReferenceArea also use)
                    and sits directly under the bars, above the labels. It lies
                    outside the plot area, so chart mouse events report no
                    bucket there and drag-select stays on the bars. */}
                <XAxis
                  dataKey="label"
                  height={LANE_HEIGHT}
                  interval={0}
                  tick={renderLaneTick}
                  tickLine={false}
                  tickSize={0}
                  tickMargin={0}
                  axisLine={{ stroke: borderColor }}
                  label={renderLaneLabel}
                />
                <XAxis
                  xAxisId="labels"
                  dataKey="label"
                  tick={{ fill: mutedColor, fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  interval="preserveStartEnd"
                />
                <YAxis
                  tick={{ fill: mutedColor, fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  width={50}
                  tickFormatter={(v) => v >= 1000 ? `${(v / 1000).toFixed(0)}k` : v}
                />
                <Tooltip content={<CustomTooltip />} cursor={{ fill: barFill + '33' }} />
                <Bar
                  dataKey="count"
                  fill={barFill}
                  radius={[2, 2, 0, 0]}
                  isAnimationActive={false}
                />
                {refAreaLeft && refAreaRight && (
                  <ReferenceArea
                    x1={refAreaLeft}
                    x2={refAreaRight}
                    strokeOpacity={0.3}
                    fill={barFill + '4D'}
                  />
                )}
              </BarChart>
            </ResponsiveContainer>
            {hovered && (
              <div
                className="timeline-tooltip timeline-lane-tooltip"
                style={{
                  left: laneHover.flip ? undefined : laneHover.x + 10,
                  right: laneHover.flip ? `calc(100% - ${laneHover.x - 10}px)` : undefined,
                  bottom: `calc(100% - ${laneHover.y - 8}px)`,
                }}
              >
                <div className="timeline-tooltip-label">{hovered.label}</div>
                <div className="timeline-tooltip-value">{formatCounts(hovered.count, hovered.notesCount)}</div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default TimelineChart
