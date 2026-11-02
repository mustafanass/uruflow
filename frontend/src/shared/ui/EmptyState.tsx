import { type ReactNode } from 'react'

import { Search } from 'lucide-react'

export function EmptyState({
  title,
  detail,
  action,
}: {
  title: string
  detail?: string
  action?: ReactNode
}) {
  return (
    <div className="empty">
      <span className="empty-mark">
        <Search size={24} />
      </span>
      <h3>{title}</h3>
      {detail && <p>{detail}</p>}
      {action}
    </div>
  )
}
