import { Link } from 'react-router-dom'
import { ArrowUpRight, Server } from 'lucide-react'
import { segment } from '@/shared/api'
import type { Agent } from '@/shared/types'
import { available, bytes, fresh } from '@/shared/utils'
import { Badge } from './Badge'

export function AgentResourceList({ agents }: { agents: Agent[] }) {
  return (
    <div className="agent-resource-list">
      {agents.map((agent) => {
        const metrics = agent.metrics
        const current = fresh(agent, metrics?.collected_at)
        const cpu =
          current && available(metrics, 'cpu')
            ? metrics?.cpu_percent
            : undefined
        const memory =
          current && available(metrics, 'memory')
            ? metrics?.memory_percent
            : undefined
        return (
          <Link
            className="agent-resource-row"
            key={agent.id}
            to={'/agents/' + segment(agent.id)}
          >
            <div className="agent-resource-name">
              <Server size={16} aria-hidden="true" />
              <strong>{agent.name}</strong>
              <Badge value={agent.status} />
              <ArrowUpRight size={13} aria-hidden="true" />
            </div>
            <div className="agent-resource-meters">
              <ResourceMeter
                label="CPU"
                value={cpu}
                text={cpu === undefined ? 'No sample' : cpu.toFixed(1) + '%'}
              />
              <ResourceMeter
                label="Memory"
                value={memory}
                text={
                  memory === undefined
                    ? 'No sample'
                    : bytes(metrics?.memory_used)
                }
              />
            </div>
          </Link>
        )
      })}
    </div>
  )
}

function ResourceMeter({
  label,
  value,
  text,
}: {
  label: string
  value: number | undefined
  text: string
}) {
  return (
    <div>
      <span>
        {label}
        <strong>{text}</strong>
      </span>
      <i
        className={
          'overview-resource-track ' +
          (value !== undefined && value > 90 ? 'pressure' : '')
        }
        aria-hidden="true"
      >
        <b style={{ width: Math.max(0, Math.min(100, value || 0)) + '%' }} />
      </i>
    </div>
  )
}
