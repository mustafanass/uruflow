import { bytes, count } from '@/shared/utils'
import { ElapsedTime, RelativeTime } from '@/shared/ui/RelativeTime'
import type { GraphNode } from '@/shared/types'
import type { ResourceObservation } from './resource-store'
import { resourceLabel } from './resource-labels'

export function NodeMetrics({
  resource,
  observation,
}: {
  resource: GraphNode
  observation: ResourceObservation | null
}) {
  if (!observation)
    return (
      <span
        className="infra-node-summary"
        title={resourceLabel(resource.summary)}
      >
        {resourceLabel(resource.summary || resource.relation)}
      </span>
    )
  if (['service', 'job'].includes(resource.kind))
    return (
      <div className="infra-node-details">
        <span>
          {observation.live === false
            ? 'No live readiness'
            : observation.expected
              ? `${observation.ready} / ${observation.expected} ready`
              : resource.state === 'built'
                ? 'Artifacts built'
                : 'No runner assigned'}
        </span>
        <span className="infra-node-summary" title={resource.summary}>
          {resource.summary || 'Assigned service'}
        </span>
      </div>
    )
  if (resource.kind === 'project')
    return (
      <div className="infra-node-details">
        <span>
          {count(observation.services || 0, 'service')} /{' '}
          {count(observation.runners || 0, 'runner')}
        </span>
        <span>
          {observation.latest ? (
            <>
              {observation.latestState}{' '}
              <RelativeTime value={observation.latest} />
            </>
          ) : (
            'No recent release'
          )}
        </span>
      </div>
    )
  return (
    <div className="infra-node-details">
      <div className="node-gauges">
        <Gauge
          label="CPU"
          text={
            observation.cpu == null
              ? 'No sample'
              : observation.cpu.toFixed(1) + '%'
          }
          value={
            observation.cpu == null
              ? null
              : (observation.cpu / (observation.cpuCapacity || 100)) * 100
          }
          title={
            observation.cpuCapacity && observation.cpuCapacity !== 100
              ? `CPU capacity: ${observation.cpuCapacity / 100} cores`
              : 'CPU usage'
          }
        />
        <Gauge
          label="Memory"
          text={
            observation.memory == null ? 'No sample' : bytes(observation.memory)
          }
          value={observation.memoryPercent ?? null}
          title={
            observation.memoryCapacity
              ? 'Memory capacity: ' + bytes(observation.memoryCapacity)
              : 'Memory usage'
          }
        />
      </div>
      <div className="node-runtime-line">
        {resource.kind === 'container' ? (
          <>
            <span>
              {observation.live && Date.parse(observation.started || '') > 0 ? (
                <>
                  Up <ElapsedTime start={observation.started!} />
                </>
              ) : observation.state === 'completed' ? (
                'Job completed'
              ) : observation.state === 'unknown' ? (
                'No live observation'
              ) : (
                observation.state
              )}
            </span>
            {!!observation.restarts && (
              <span className="node-restarts">
                {count(observation.restarts, 'restart')}
              </span>
            )}
          </>
        ) : resource.kind === 'server' ? (
          observation.live ? (
            <>
              <span>{count(observation.online || 0, 'agent')} online</span>
              <span>{count(observation.active || 0, 'active release')}</span>
            </>
          ) : (
            <span>No live observation</span>
          )
        ) : (
          <>
            <span>{count(observation.containers || 0, 'container')}</span>
            {!!observation.active && (
              <span>{count(observation.active, 'active release')}</span>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function Gauge({
  label,
  text,
  value,
  title,
}: {
  label: string
  text: string
  value: number | null
  title: string
}) {
  return (
    <div className="node-gauge" title={title}>
      <span>
        {label}
        <strong>{text}</strong>
      </span>
      <i aria-hidden="true">
        <b
          style={{
            width:
              value == null ? '0%' : Math.max(0, Math.min(100, value)) + '%',
          }}
        />
      </i>
    </div>
  )
}
