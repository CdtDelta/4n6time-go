import { useState, useEffect, useCallback, useRef } from 'react'
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer,
  Brush, CartesianGrid, ReferenceArea,
} from 'recharts'
import { GetTimelineHistogram } from '../../wailsjs/go/main/App'

function TimelineChart({ visible, filters, dbInfo, onSelectRange }) {
  const [data, setData] = useState([])
  const [loading, setLoading] = useState(false)
  const [refAreaLeft, setRefAreaLeft] = useState(null)
  const [refAreaRight, setRefAreaRight] = useState(null)
  const [selecting, setSelecting] = useState(false)

  // Load histogram data when visible, filters change, or db changes
  useEffect(() => {
    if (!visible || !dbInfo) return

    let cancelled = false
    setLoading(true)

    async function loadData() {
      try {
        const req = {
          filters: filters?.filters || [],
          logic: filters?.logic || 'AND',
          orderBy: 'datetime',
          page: 1,
          pageSize: 1000,
        }

        // Add date range filters if present
        if (filters?.dateFrom && filters?.dateTo) {
          req.filters = [
            ...req.filters,
            { field: 'datetime', operator: '>=', value: filters.dateFrom },
            { field: 'datetime', operator: '<=', value: filters.dateTo },
          ]
        }

        const result = await GetTimelineHistogram(req)
        if (!cancelled && result) {
          setData(result.map(b => ({
            timestamp: b.timestamp,
            count: b.count,
            // Short label for x-axis
            label: formatLabel(b.timestamp),
          })))
        }
      } catch (err) {
        console.error('Error loading histogram:', err)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadData()
    return () => { cancelled = true }
  }, [visible, filters, dbInfo])

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
    return (
      <div className="timeline-tooltip">
        <div className="timeline-tooltip-label">{label}</div>
        <div className="timeline-tooltip-value">{payload[0].value.toLocaleString()} events</div>
      </div>
    )
  }

  if (!visible) return null

  // Read theme colors from CSS variables for recharts
  const style = getComputedStyle(document.documentElement)
  const barFill = style.getPropertyValue('--color-bar-fill').trim() || '#533483'
  const borderColor = style.getPropertyValue('--border-primary').trim() || '#0f3460'
  const mutedColor = style.getPropertyValue('--text-muted').trim() || '#808080'

  return (
    <div className="timeline-chart">
      <div className="timeline-header">
        <span className="timeline-title">Timeline</span>
        {loading && <span className="timeline-loading">Loading...</span>}
        <span className="timeline-hint">Click and drag to select a time range</span>
      </div>
      <div className="timeline-body">
        {data.length === 0 && !loading ? (
          <div className="timeline-empty">No data to display</div>
        ) : (
          <ResponsiveContainer width="100%" height={140}>
            <BarChart
              data={data}
              onMouseDown={handleMouseDown}
              onMouseMove={handleMouseMove}
              onMouseUp={handleMouseUp}
            >
              <CartesianGrid strokeDasharray="3 3" stroke={borderColor} vertical={false} />
              <XAxis
                dataKey="label"
                tick={{ fill: mutedColor, fontSize: 10 }}
                tickLine={false}
                axisLine={{ stroke: borderColor }}
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
        )}
      </div>
    </div>
  )
}

export default TimelineChart
