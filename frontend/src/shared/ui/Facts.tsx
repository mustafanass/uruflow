import { type ReactNode } from 'react'

export function Facts({ items }: { items: [string, ReactNode][] }) {
  return (
    <dl className="facts">
      {items.map(([label, value]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>
            {value === undefined || value === null || value === ''
              ? '—'
              : value}
          </dd>
        </div>
      ))}
    </dl>
  )
}
