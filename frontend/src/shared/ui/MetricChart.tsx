import { useState } from 'react'

import { type Sample } from '@/shared/types'

import { Panel } from './Panel'

export function MetricChart({
  title,
  samples,
  field,
  format = String,
  color = 'var(--accent-fill)',
}: {
  title: string
  samples: Sample[]
  field: 'cpu' | 'memory'
  format?: (value: number) => string
  color?: string
}) {
  const [hover, setHover] = useState<number | null>(null)
  const values = samples.filter((s) => s[field] !== null)
  const width = 680,
    height = 170,
    pad = 12
  const max = Math.max(
    field === 'cpu' ? 100 : 1,
    ...values.map((s) => s[field] || 0),
  )
  const minTime = samples[0]?.at || 0,
    maxTime = samples[samples.length - 1]?.at || 0
  let path = '',
    previous = 0
  for (const sample of samples) {
    const value = sample[field]
    if (value === null) {
      previous = 0
      continue
    }
    const x =
        pad +
        (maxTime === minTime
          ? 0
          : (sample.at - minTime) / (maxTime - minTime)) *
          (width - pad * 2),
      y = height - pad - (value / max) * (height - pad * 2)
    path +=
      (previous && sample.at - previous <= 180000 ? ' L' : ' M') +
      x.toFixed(2) +
      ',' +
      y.toFixed(2)
    previous = sample.at
  }
  const selected =
    hover === null
      ? values[values.length - 1]
      : values[Math.min(hover, values.length - 1)]
  return (
    <Panel
      title={title}
      actions={
        <span className="chart-current">
          {selected && selected[field] !== null
            ? format(selected[field]!)
            : 'Awaiting samples'}
        </span>
      }
    >
      <div
        className="metric-chart"
        onMouseLeave={() => setHover(null)}
        onMouseMove={(event) => {
          const rect = event.currentTarget.getBoundingClientRect()
          setHover(
            Math.round(
              Math.max(
                0,
                Math.min(1, (event.clientX - rect.left) / rect.width),
              ) *
                (values.length - 1),
            ),
          )
        }}
      >
        <span className="chart-ceiling">{format(max)}</span>
        <svg
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-label={`${title}, ${values.length} samples during this session`}
          preserveAspectRatio="none"
        >
          <path
            d={`M${pad},${height / 2}H${width - pad} M${pad},${height - pad}H${width - pad}`}
            stroke="var(--border)"
            strokeDasharray="4 4"
            fill="none"
          />
          <path
            d={path}
            stroke={color}
            strokeWidth="2.5"
            fill="none"
            vectorEffect="non-scaling-stroke"
          />
        </svg>
        {values.length === 0 && (
          <span className="chart-empty">Waiting for a valid metric sample</span>
        )}
      </div>
      <div className="chart-axis">
        <span>
          {minTime ? new Date(minTime).toLocaleTimeString() : 'Session start'}
        </span>
        <span>
          {hover !== null && selected
            ? new Date(selected.at).toLocaleTimeString()
            : 'Session history · up to 120 samples'}
        </span>
        <span>{maxTime ? new Date(maxTime).toLocaleTimeString() : 'Now'}</span>
      </div>
    </Panel>
  )
}
