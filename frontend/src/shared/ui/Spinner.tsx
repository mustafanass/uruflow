import { LoadingIndicator } from './LoadingIndicator'

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return (
    <div className="loading" role="status">
      <LoadingIndicator />
      <span>{label}</span>
    </div>
  )
}
