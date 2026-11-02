import { Link } from 'react-router-dom'
import { ArrowUpRight } from 'lucide-react'
import type { Project, Service } from '@/shared/types'
import { useWorkspace } from '@/shared/hooks'
import { segment } from '@/shared/api'
import { Badge, Code, Facts } from '@/shared/ui'
import { bytes, containerState, ports } from '@/shared/utils'

export function ServiceDetails({
  service,
  project,
}: {
  service: Service
  project: Project
}) {
  const { snapshot } = useWorkspace()
  if (!snapshot) return null
  const instances = snapshot.containers.filter(
    (container) =>
      container.project === project.name &&
      (container.service || '') === service.name,
  )
  return (
    <>
      <h4>Service definition</h4>
      <Facts
        items={[
          ['Mode', service.mode || 'service'],
          ['Ports', ports(service.ports)],
          [
            'Source / image',
            <Code
              value={service.image || service.git_url || project.git_url || ''}
            />,
          ],
          [
            'CPU limit',
            service.resources?.cpus
              ? `${service.resources.cpus} cores`
              : 'Not configured',
          ],
          [
            'Memory limit',
            service.resources?.memory_bytes
              ? bytes(service.resources.memory_bytes)
              : 'Not configured',
          ],
          ['PID limit', service.resources?.pids || 'Not configured'],
          ['Restart policy', service.restart || 'unless-stopped'],
          ['Healthcheck', service.healthcheck?.type || 'Not configured'],
        ]}
      />
      {!!service.depends_on?.length && (
        <>
          <h4>Dependencies</h4>
          {service.depends_on.map((dependency) => (
            <p className="dependency-detail" key={dependency.service}>
              {dependency.service}
              <span>requires {dependency.condition}</span>
            </p>
          ))}
        </>
      )}
      {!!service.volumes?.length && (
        <>
          <h4>Storage</h4>
          {service.volumes.map((mount, index) => (
            <p className="inspector-note" key={index}>
              {mount.source} → {mount.target}
              {mount.read_only ? ' (read only)' : ''}
            </p>
          ))}
        </>
      )}
      {!!(service.networks?.length || service.network) && (
        <>
          <h4>Networks</h4>
          <p className="inspector-note">
            {[
              ...(service.networks || []).map((network) => network.name),
              service.network,
            ]
              .filter(Boolean)
              .join(', ')}
          </p>
        </>
      )}
      <h4>Instances</h4>
      {instances.length ? (
        instances.map((container) => (
          <Link
            className="inspector-instance"
            key={container.agent_id + '/' + container.id}
            to={`/containers/${segment(container.agent_id)}/${segment(container.id)}`}
          >
            <div>
              <strong>
                {snapshot.agents.find(
                  (agent) => agent.id === container.agent_id,
                )?.name || container.agent_id}
              </strong>
              <span>{container.name}</span>
            </div>
            <Badge value={containerState(container, snapshot)} />
            <ArrowUpRight size={13} aria-hidden="true" />
          </Link>
        ))
      ) : (
        <p className="inspector-note">
          No containers currently observed for this service.
        </p>
      )}
    </>
  )
}
