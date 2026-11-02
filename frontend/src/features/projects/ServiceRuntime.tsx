import { Link } from 'react-router-dom'
import {
  ArrowUpRight,
  FileText,
  GitBranch,
  Layers,
  Network,
  Package,
  Server,
} from 'lucide-react'
import { segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { Project } from '@/shared/types'
import { Badge } from '@/shared/ui'
import { StateMark } from '@/shared/ui/StateMark'
import { bytes, fresh, ports } from '@/shared/utils'
import { projectPriority, type projectHealth } from './project-health'

type Row = ReturnType<typeof projectHealth>['rows'][number]

export function ServiceRuntime({
  project,
  row,
}: {
  project: Project
  row: Row
}) {
  const { snapshot } = useWorkspace()
  if (!snapshot) return null
  const service = row.service
  const source =
    service.image?.split('@')[0] ||
    (service.git_url || project.git_url || '')
      .replace(/^https?:\/\//, '')
      .replace(/\.git$/, '')
  return (
    <article
      className="service-runtime"
      aria-label={(service.name || project.name) + ' service health'}
    >
      <div className="service-runtime-heading">
        <Layers size={17} aria-hidden="true" />
        <h3>{service.name || project.name}</h3>
        {service.mode === 'job' && <span className="service-mode">Job</span>}
        <Badge value={row.state} />
        {row.expected > 0 && (
          <span className="service-runtime-count">
            {row.ready} / {row.expected} ready
          </span>
        )}
      </div>
      <div className="service-runtime-meta">
        <span title={source}>
          {service.image ? (
            <Package size={13} aria-hidden="true" />
          ) : (
            <GitBranch size={13} aria-hidden="true" />
          )}
          <span>{source || 'Source not configured'}</span>
          {!service.image && (
            <small>{service.branch || project.branch || 'main'}</small>
          )}
        </span>
        <span>
          <Network size={13} aria-hidden="true" />
          {service.ports?.length ? ports(service.ports) : 'No published ports'}
        </span>
      </div>
      {service.depends_on?.length ? (
        <p className="service-dependencies">
          Depends on{' '}
          {service.depends_on
            .map((dependency) => dependency.service)
            .join(', ')}
        </p>
      ) : null}
      <div className="service-placements">
        {[...row.placements]
          .sort((a, b) => projectPriority(a.state) - projectPriority(b.state))
          .slice(0, 4)
          .map((placement) => {
            const agent = snapshot.agents.find(
              (value) => value.id === placement.agent,
            )
            return placement.containers.length ? (
              placement.containers.map((container) => {
                const state =
                  snapshot.graphs[project.name]?.runtime.nodes.find(
                    (node) =>
                      node.kind === 'container' &&
                      node.agent_id === placement.agent &&
                      node.container_id === container.id,
                  )?.state || 'unknown'
                const current =
                  container.stats_available &&
                  container.state === 'running' &&
                  fresh(agent, container.observed_at)
                const address = `/containers/${segment(placement.agent)}/${segment(container.id)}`
                return (
                  <div
                    className="service-instance"
                    key={placement.agent + '/' + container.id}
                  >
                    <div className="service-instance-heading">
                      <StateMark
                        state={state}
                        label={container.name + ': ' + state}
                      />
                      <Link to={address} title={container.name}>
                        {agent?.name || placement.agent}
                      </Link>
                      {!['healthy', 'running'].includes(state) && (
                        <Badge value={state} />
                      )}
                      <Link
                        className="instance-logs"
                        to={address + '?tab=Logs'}
                        aria-label={
                          'Open logs for ' +
                          (service.name || project.name) +
                          ' on ' +
                          (agent?.name || placement.agent)
                        }
                      >
                        <FileText size={13} aria-hidden="true" />
                        Logs
                      </Link>
                    </div>
                    <div className="instance-resources">
                      <span>
                        CPU
                        <strong>
                          {current
                            ? container.cpu_percent.toFixed(1) + '%'
                            : 'No sample'}
                        </strong>
                      </span>
                      <span>
                        Memory
                        <strong>
                          {current
                            ? bytes(container.memory_usage)
                            : 'No sample'}
                        </strong>
                      </span>
                    </div>
                  </div>
                )
              })
            ) : (
              <div
                className="service-instance unobserved"
                key={placement.agent}
              >
                <div className="service-instance-heading">
                  <StateMark state={placement.state} />
                  <Link to={'/agents/' + segment(placement.agent)}>
                    {agent?.name || placement.agent}
                  </Link>
                  <Server size={13} aria-hidden="true" />
                </div>
                <span>
                  {placement.state === 'completed'
                    ? 'Job completed'
                    : placement.state === 'undeployed'
                      ? 'Awaiting first deployment'
                      : placement.state === 'missing'
                        ? 'Expected container is missing'
                        : 'Waiting for an observation'}
                </span>
              </div>
            )
          })}
      </div>
      {row.placements.length > 4 && (
        <Link className="service-placement-more" to="?tab=Graph">
          Inspect all {row.placements.length} runner placements
          <ArrowUpRight size={13} aria-hidden="true" />
        </Link>
      )}
      {!row.placements.length && (
        <p className="service-runtime-empty">
          {project.workflow === 'build_only'
            ? 'Build-only service. Artifacts are stored in the registry.'
            : 'No runner assigned.'}
          <Link to="?tab=Graph">
            Inspect placement
            <ArrowUpRight size={12} aria-hidden="true" />
          </Link>
        </p>
      )}
    </article>
  )
}
