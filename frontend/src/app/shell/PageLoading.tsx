import { useLocation } from 'react-router-dom'

import { pageTitle } from '@/app/navigation'
import { LoadingIndicator } from '@/shared/ui'

export function PageLoading() {
  const { pathname } = useLocation()
  return (
    <div className="page-loading" role="status" aria-live="polite">
      <div className="page-loading-status">
        <LoadingIndicator />
        <span>Loading {pageTitle(pathname).toLowerCase()}…</span>
      </div>
      <div className="page-loading-outline" aria-hidden="true">
        <div className="skeleton-heading">
          <i />
          <i />
        </div>
        <div className="skeleton-columns">
          <i />
          <i />
          <i />
        </div>
        {[0, 1, 2, 3].map((row) => (
          <div className="skeleton-row" key={row}>
            <i />
            <i />
            <i />
          </div>
        ))}
      </div>
    </div>
  )
}
