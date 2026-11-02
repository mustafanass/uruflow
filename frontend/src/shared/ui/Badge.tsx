import { statusTone } from '@/shared/utils/status'

export function Badge({ value }: { value?: string }) {
  const state = value || 'unknown'
  const tone = statusTone(state)
  return (
    <span className={'badge ' + tone}>
      <span className="status-dot" />
      {state.replaceAll('_', ' ')}
    </span>
  )
}
