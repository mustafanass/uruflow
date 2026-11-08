import { RelativeTime } from '@/shared/ui/RelativeTime'
import { type ReactNode } from 'react'
import { Link } from 'react-router-dom'

import { ArrowUpRight, Boxes, X } from 'lucide-react'

import { ServiceDetails } from './ServiceDetails'
import { resourceLabel } from './resource-labels'
import { icons } from '@/features/infrastructure/ResourceNode'
import { segment } from '@/shared/api'
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
    snapshot.projects.find(
      (value) =>
        value.name ===
        (node.kind === 'project' ? node.name : container?.project),
    )
  const definition = ['service', 'job'].includes(node.kind)
  const service =
    owner && definition
      ? services(owner).find((s) => s.name === node.service)
      : undefined
  const live =
    !!container?.stats_available && fresh(agent, container.observed_at)
  const facts: [string, ReactNode][] = [
    ['Purpose', resourceLabel(node.relation)],
  ]
  if (node.summary && !container && !definition) {
    facts.push(['Details', resourceLabel(node.summary)])
  }
  if (agent && node.kind !== 'agent') {
    facts.push(['Agent', agent.name])
  }
  if (node.service && node.service !== node.name && !definition) {
    facts.push(['Service', node.service])
  }
  const target = container
    ? {
        to: `/containers/${segment(container.agent_id)}/${segment(container.id)}`,
        label: 'Open container',
      }
    : node.kind === 'project'
      ? { to: '/projects/' + segment(node.name), label: 'Open project' }
      : agent
        ? { to: '/agents/' + segment(agent.id), label: 'Open agent' }
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
        <Facts items={facts} />
        {container && (
          <>
            <h4>Observed container</h4>
            <Facts
              items={[
                [
                  'CPU',
                  live ? `${container.cpu_percent.toFixed(1)}%` : 'Unavailable',
                ],
                [
                  'Memory',
                  live ? bytes(container.memory_usage) : 'Unavailable',
                ],
                ['Restarts', container.restart_count],
                ['Image', <Code value={container.image} />],
                ['Release', short(container.details?.release)],
                ['Sample', <RelativeTime value={container.observed_at} />],
              ]}
            />
          </>
        )}
        {agent && !container && (
          <HostResources
            metrics={agent.metrics}
            compact
            label="Last reported resources"
          />
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
        {node.kind === 'project' && owner && (
          <>
            <h4>Configured agents</h4>
            <Facts
              items={[
                [
                  'Builder',
                  snapshot.agents.find((value) => value.id === owner.builder)
                    ?.name ||
                    owner.builder ||
                    'None',
                ],
                [
                  'Runners',
                  owner.runners
                    .map(
                      (id) =>
                        snapshot.agents.find((value) => value.id === id)
                          ?.name || id,
                    )
                    .join(', ') || 'None',
                ],
              ]}
            />
          </>
        )}
        {node.kind === 'network' && (
          <p className="inspector-note">Attachment does not imply traffic.</p>
        )}
        {['mount', 'volume'].includes(node.kind) && (
          <p className="inspector-note">
            Storage is local to its host unless the driver shares it.
          </p>
        )}
        {target && (
          <Link className="button full" to={target.to}>
            {target.label}
            <ArrowUpRight size={15} />
          </Link>
        )}
      </div>
    </aside>
  )
}
