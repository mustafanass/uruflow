import { Link } from 'react-router-dom'

import { Boxes, Container, Radio, Server } from 'lucide-react'

import { useWorkspace } from '@/shared/hooks'
import { containerState } from '@/shared/utils'

export function FleetSummary() {
  const { snapshot } = useWorkspace()
  if (!snapshot) return null
  const online = snapshot.agents.filter(
    (agent) => agent.status === 'online',
  ).length
  const running = snapshot.containers.filter((container) =>
    ['running', 'healthy'].includes(containerState(container, snapshot)),
  ).length
  const active = snapshot.releases.filter(
    (release) => !['succeeded', 'failed', 'skipped'].includes(release.status),
  ).length
  const items = [
    {
      label: 'Projects',
      value: snapshot.projects.length,
      detail: snapshot.projects.length === 1 ? 'environment' : 'environments',
      to: '/projects',
      icon: Boxes,
    },
    {
      label: 'Agents',
      value: online + ' / ' + snapshot.agents.length,
      detail: snapshot.agents.length ? 'online' : 'No agents enrolled',
      to: '/agents',
      icon: Server,
    },
    {
      label: 'Containers',
      value: running,
      detail: 'running',
      to: '/agents?tab=Containers',
      icon: Container,
    },
    {
      label: 'Releases',
      value: active,
      detail: 'in progress',
      to: '/projects?tab=Releases',
      icon: Radio,
    },
  ]
  return (
    <div className="fleet-summary" aria-label="Fleet summary">
      {items.map((item) => (
        <Link key={item.to} to={item.to}>
          <item.icon size={17} aria-hidden="true" />
          <div>
            <span>{item.label}</span>
            <strong>
              {item.value}
              <small>{item.detail}</small>
            </strong>
          </div>
        </Link>
      ))}
    </div>
  )
}
