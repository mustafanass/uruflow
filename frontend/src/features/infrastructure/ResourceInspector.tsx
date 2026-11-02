import { RelativeTime } from '@/shared/ui/RelativeTime'
import { Link } from 'react-router-dom'

import { ArrowUpRight, Boxes, X } from 'lucide-react'

import { ServiceDetails } from './ServiceDetails'
import { resourceLabel } from './resource-labels'
import { icons } from '@/features/infrastructure/ResourceNode'
import { useWorkspace } from '@/shared/hooks'
import { type GraphNode, type Project } from '@/shared/types'
import { Badge, Code, Facts, HostResources } from '@/shared/ui'
import { bytes, fresh, services, short } from '@/shared/utils'

export function ResourceInspector({
  node,
  project,
  close,
}: {
  node: GraphNode
  project?: Project
  close: () => void
}) {
  const { snapshot } = useWorkspace()
  if (!snapshot) {
    return null
  }
  const Icon = icons[node.kind] || Boxes
  const container = snapshot.containers.find(
    (c) => c.id === node.container_id && c.agent_id === node.agent_id,
  )
  const agent = snapshot.agents.find((a) => a.id === node.agent_id)
  const owner =
    project ||
    snapshot.projects.find((value) => value.name === container?.project)
  const service =
    owner && ['service', 'job', 'instance', 'container'].includes(node.kind)
      ? services(owner).find((s) => s.name === node.service)
      : undefined
  return (
    <aside className="graph-inspector" aria-label="Resource inspector">
      <div className="inspector-heading">
        <span>Resource inspector</span>
        <button
          className="icon-button"
          aria-label="Close resource inspector"
          onClick={close}
        >
          <X size={18} />
        </button>
      </div>
      <div className="inspector-content">
        <span className={'resource-icon large ' + node.kind}>
          <Icon size={24} />
        </span>
        <span className="resource-kind">{node.kind}</span>
        <h3>{node.name}</h3>
        <Badge value={node.state || 'declared'} />
        <Facts
          items={[
            ['Purpose', resourceLabel(node.relation)],
            ['Details', resourceLabel(node.summary)],
            ['Agent', agent?.name],
            ['Role', agent?.roles.join(', ')],
            ['Service', node.service],
          ]}
        />
        {container && (
          <>
            <h4>Observed container</h4>
            <Facts
              items={[
                [
                  'CPU',
                  container.stats_available &&
                  fresh(agent, container.observed_at)
                    ? `${container.cpu_percent.toFixed(1)}%`
                    : 'Unavailable',
                ],
                [
                  'Memory',
                  container.stats_available &&
                  fresh(agent, container.observed_at)
                    ? bytes(container.memory_usage)
                    : 'Unavailable',
                ],
                ['Restarts', container.restart_count],
                ['Sample', <RelativeTime value={container.observed_at} />],
                ['Image', <Code value={container.image} />],
                ['Release', short(container.details?.release)],
              ]}
            />
            <Link
              className="button full"
              to={`/containers/${encodeURIComponent(container.agent_id)}/${encodeURIComponent(container.id)}`}
            >
              Open container
              <ArrowUpRight size={15} />
            </Link>
          </>
        )}
        {agent && !container && (
          <>
            <HostResources
              metrics={agent.metrics}
              compact
              label="Last reported resources"
            />
            <Link
              className="button full"
              to={'/agents/' + encodeURIComponent(agent.id)}
            >
              Open agent
              <ArrowUpRight size={15} />
            </Link>
          </>
        )}
        {service && owner && (
          <ServiceDetails service={service} project={owner} />
        )}
        {node.kind === 'server' && (
          <HostResources
            metrics={snapshot.host}
            compact
            label="Control-plane host"
          />
        )}
        {node.kind === 'project' && (
          <Link
            className="button full"
            to={'/projects/' + encodeURIComponent(node.name)}
          >
            Open project
            <ArrowUpRight size={15} />
          </Link>
        )}
        {node.kind === 'network' && (
          <p className="inspector-note">Attachment does not imply traffic.</p>
        )}
        {['mount', 'volume'].includes(node.kind) && (
          <p className="inspector-note">
            Storage is local to its host unless the driver shares it.
          </p>
        )}
      </div>
    </aside>
  )
}
