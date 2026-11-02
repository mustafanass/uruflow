import { Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { ArrowUpRight, Network, Server } from 'lucide-react'
import { segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { Project } from '@/shared/types'
import { Panel } from '@/shared/ui'
import { AgentResourceList } from '@/shared/ui/AgentResourceList'
import { services } from '@/shared/utils'
import { projectHealth } from './project-health'
import { ProjectReleaseSummary } from './ProjectReleaseSummary'
import { ServiceRuntime } from './ServiceRuntime'
import { ProjectMetrics } from './ProjectMetrics'

export function ProjectOverview({
  project,
  showMetrics = false,
}: {
  project: Project
  showMetrics?: boolean
}) {
  const [metricsOpen, setMetricsOpen] = useState(showMetrics)
  useEffect(() => {
    if (showMetrics) setMetricsOpen(true)
  }, [showMetrics])
  const { snapshot } = useWorkspace()
  if (!snapshot) return null
  const health = projectHealth(project, snapshot, services(project))
  const latest = snapshot.releases.find(
    (release) => release.project === project.name,
  )
  const assigned = new Set(
    [project.builder, ...(project.runners || [])].filter(Boolean),
  )
  const agents = snapshot.agents.filter((agent) => assigned.has(agent.id))
  const missing = [...assigned].filter(
    (id) => !agents.some((agent) => agent.id === id),
  )
  return (
    <>
      <div className="project-overview-grid">
        <div className="stack">
          <Panel
            title="Service health"
            className="service-health-panel"
            detail={
              health.buildOnly
                ? 'Build-only workflow'
                : health.expected
                  ? `${health.ready} / ${health.expected} instances ready`
                  : 'No runners assigned'
            }
            actions={
              <Link className="button small" to="?tab=Graph">
                <Network size={15} aria-hidden="true" />
                Open graph
              </Link>
            }
          >
            {health.rows.map((row) => (
              <ServiceRuntime
                key={row.service.name}
                project={project}
                row={row}
              />
            ))}
            <Link className="service-health-footer" to="?tab=Graph">
              Inspect definitions and dependencies in the graph
              <ArrowUpRight size={14} aria-hidden="true" />
            </Link>
          </Panel>
        </div>
        <aside className="stack" aria-label="Release and placement">
          <ProjectReleaseSummary release={latest} project={project.name} />
          <Panel
            title="Assigned machines"
            detail={project.builder ? 'Builder and runners' : 'Runners'}
            actions={<Server size={16} aria-hidden="true" />}
          >
            <AgentResourceList agents={agents.slice(0, 4)} />
            {agents.length > 4 && (
              <Link className="overview-more" to="?tab=Graph">
                View all {assigned.size} assigned machines
                <ArrowUpRight size={14} aria-hidden="true" />
              </Link>
            )}
            {missing.slice(0, 4).map((id) => (
              <Link
                className="missing-assignment"
                key={id}
                to={'/agents/' + segment(id)}
              >
                <Server size={15} aria-hidden="true" />
                <span>{id}</span>Not observed
              </Link>
            ))}
            {!assigned.size && (
              <p className="panel-note">
                No machines assigned. Inspect the project configuration.
              </p>
            )}
          </Panel>
        </aside>
      </div>
      <details
        className="details-panel"
        open={metricsOpen}
        onToggle={(event) => setMetricsOpen(event.currentTarget.open)}
      >
        <summary>Resource trends</summary>
        {metricsOpen && (
          <div className="details-content">
            <ProjectMetrics p={project} />
          </div>
        )}
      </details>
    </>
  )
}
