import { Navigate, useLocation } from 'react-router-dom'

export function LegacyRedirect({
  to,
  tab,
  anchor,
}: {
  to: string
  tab?: string
  anchor?: string
}) {
  const location = useLocation()
  const params = new URLSearchParams(location.search)
  if (tab) params.set('tab', tab)
  return (
    <Navigate
      replace
      to={{
        pathname: to,
        search: params.toString(),
        hash: anchor || location.hash,
      }}
    />
  )
}
