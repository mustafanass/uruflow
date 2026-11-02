import { Link } from 'react-router-dom'

import { segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { Project } from '@/shared/types'
import { EmptyState, MetricChart } from '@/shared/ui'
import { bytes } from '@/shared/utils'

export function ProjectMetrics({ p }: { p: Project }) {
  const { snapshot, samples } = useWorkspace()
  if (!snapshot) {
    return null
  }
  const containers = snapshot.containers.filter((c) => c.project === p.name)
  return (
    <>
      <div className="section-heading">
        <div>
          <h2>Workload resources</h2>
          <p>
            Session trends from distinct agent samples. Memory includes cache.
          </p>
        </div>
      </div>
      {containers.length ? (
        containers.map((c) => (
          <div className="service-metrics" key={c.agent_id + '/' + c.id}>
            <h3>
              <Link to={`/containers/${segment(c.agent_id)}/${segment(c.id)}`}>
                {c.name}
              </Link>
              <span>
                {snapshot.agents.find((a) => a.id === c.agent_id)?.name}
              </span>
            </h3>
            <div className="two-columns">
              <MetricChart
                title="CPU usage"
                samples={samples('container/' + c.agent_id + '/' + c.id)}
                field="cpu"
                format={(v) => v.toFixed(1) + '%'}
              />
              <MetricChart
                title="Memory usage"
                samples={samples('container/' + c.agent_id + '/' + c.id)}
                field="memory"
                format={bytes}
              />
            </div>
          </div>
        ))
      ) : (
        <EmptyState
          title="No container samples yet"
          detail="Metrics appear after your runners report deployed containers."
        />
      )}
    </>
  )
}
