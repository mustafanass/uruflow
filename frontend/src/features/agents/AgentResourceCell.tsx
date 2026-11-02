import { type Agent } from '@/shared/types'
import { available, bytes, fresh } from '@/shared/utils'

export function ResourceCell({
  agent,
  kind,
}: {
  agent: Agent
  kind: 'cpu' | 'memory'
}) {
  const metrics = agent.metrics
  if (
    !metrics ||
    !available(metrics, kind) ||
    !fresh(agent, metrics.collected_at)
  ) {
    return <span className="muted">Unavailable</span>
  }
  const percent = kind === 'cpu' ? metrics.cpu_percent : metrics.memory_percent
  return (
    <div className="table-meter">
      <span>
        {kind === 'cpu' ? percent.toFixed(1) + '%' : bytes(metrics.memory_used)}
      </span>
      <i>
        <b
          className={percent > 90 ? 'pressure' : ''}
          style={{ width: Math.min(100, Math.max(0, percent)) + '%' }}
        />
      </i>
    </div>
  )
}
