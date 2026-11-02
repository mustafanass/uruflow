import { useWorkspace } from '@/shared/hooks'
import type { Agent } from '@/shared/types'
import { HostResources, MetricChart } from '@/shared/ui'
import { bytes } from '@/shared/utils'

export function AgentMetrics({ a }: { a: Agent }) {
  const { samples } = useWorkspace()
  return (
    <>
      <HostResources metrics={a.metrics} label="Latest host sample" />
      <div className="two-columns">
        <MetricChart
          title="Host CPU"
          field="cpu"
          samples={samples('agent/' + a.id)}
          format={(v) => v.toFixed(1) + '%'}
        />
        <MetricChart
          title="Host memory"
          field="memory"
          samples={samples('agent/' + a.id)}
          format={bytes}
        />
      </div>
      <p className="subtle-note">
        Trends begin when this workspace opens. Host CPU is normalized to total
        machine capacity.
      </p>
    </>
  )
}
