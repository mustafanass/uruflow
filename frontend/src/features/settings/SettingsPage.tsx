import { useEffect } from 'react'
import { Navigate, NavLink, useParams } from 'react-router-dom'

import { Gauge, History, Package, Webhook } from 'lucide-react'

import { useWorkspace } from '@/shared/hooks'
import { EmptyState, PageHeader } from '@/shared/ui'

import { AgentReporting } from './AgentReporting'
import { RegistrySettings } from './RegistrySettings'
import { WebhookDeliveries } from './WebhookDeliveries'
import { WebhookSettings } from './WebhookSettings'

const groups = [
  {
    label: 'Deployments',
    sections: [
      {
        path: 'webhooks',
        label: 'Webhooks',
        icon: Webhook,
        content: WebhookSettings,
      },
      {
        path: 'deliveries',
        label: 'Deliveries',
        icon: History,
        content: WebhookDeliveries,
      },
    ],
  },
  {
    label: 'Infrastructure',
    sections: [
      {
        path: 'registry',
        label: 'Registry',
        icon: Package,
        content: RegistrySettings,
      },
      {
        path: 'agents',
        label: 'Agent reporting',
        icon: Gauge,
        content: AgentReporting,
      },
    ],
  },
]
const sections = groups.flatMap((group) => group.sections)

export default function SettingsPage() {
  const { canAdmin } = useWorkspace()
  const { section } = useParams()
  const current = sections.find((item) => item.path === section)
  useEffect(() => {
    document
      .querySelector('.settings-nav a.active')
      ?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  }, [section])
  if (!canAdmin)
    return (
      <>
        <PageHeader title="Settings" />
        <EmptyState
          title="Administrator access required"
          detail="Webhooks, registry and agent settings are managed by administrators."
        />
      </>
    )
  if (!current) return <Navigate replace to={'/settings/' + sections[0].path} />
  return (
    <>
      <PageHeader title="Settings" />
      <div className="settings-layout">
        <nav className="settings-nav" aria-label="Settings sections">
          {groups.map((group) => (
            <div className="settings-group" key={group.label}>
              <h2>{group.label}</h2>
              {group.sections.map((item) => (
                <NavLink key={item.path} to={'/settings/' + item.path}>
                  <item.icon size={16} aria-hidden="true" />
                  {item.label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="settings-content">
          <current.content />
        </div>
      </div>
    </>
  )
}
