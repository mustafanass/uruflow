import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useState } from 'react'
import type { Alert } from '@/shared/types'
import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { Badge, DataTable } from '@/shared/ui'

export function AlertTable({
  alerts,
  onResolved,
}: {
  alerts: Alert[]
  onResolved?: () => void
}) {
  const { canOperate, confirm } = useWorkspace()
  const [resolved, setResolved] = useState<Set<string>>(new Set())
  return (
    <>
      <DataTable
        scope="alerts-page"
        rows={alerts.map((alert) =>
          resolved.has(alert.id) ? { ...alert, resolved: true } : alert,
        )}
        rowKey={(a) => a.id}
        rowHref={(a) =>
          a.agent_id ? '/agents/' + segment(a.agent_id) : undefined
        }
        searchText={(a) =>
          a.message + ' ' + a.agent_name + ' ' + a.severity + ' ' + a.type
        }
        empty="No alerts to show"
        emptyDetail="New conditions appear here as the control plane receives observations."
        columns={[
          {
            key: 'severity',
            title: 'Severity',
            render: (a) => <Badge value={a.severity} />,
            sort: (a) => a.severity,
          },
          {
            key: 'state',
            title: 'State',
            render: (a) => <Badge value={a.resolved ? 'resolved' : 'active'} />,
          },
          {
            key: 'condition',
            title: 'Condition',
            render: (a) => (
              <div className="stacked">
                <strong className="event-message">{a.message}</strong>
                <small>{a.type}</small>
              </div>
            ),
          },
          {
            key: 'agent',
            title: 'Agent',
            render: (a) => a.agent_name || 'Control plane',
          },
          {
            key: 'age',
            title: 'Raised',
            render: (a) => <RelativeTime value={a.created_at} />,
            sort: (a) => a.created_at,
          },
          {
            key: 'resolved',
            title: 'Resolved',
            render: (a) =>
              a.resolved_at ? <RelativeTime value={a.resolved_at} /> : '—',
          },
          {
            key: 'actions',
            title: '',
            render: (a) =>
              canOperate &&
              !a.resolved && (
                <button
                  className="button small"
                  onClick={(e) => {
                    e.stopPropagation()
                    confirm({
                      title: 'Resolve alert',
                      description: `Resolve “${a.message}”. If its underlying condition persists, the control plane may raise it again.`,
                      label: 'Resolve alert',
                      success: 'Alert resolved',
                      run: async () => {
                        await api('/api/alerts/' + segment(a.id) + '/resolve', {
                          method: 'POST',
                          body: {},
                        })
                        setResolved((previous) => new Set([...previous, a.id]))
                        onResolved?.()
                      },
                    })
                  }}
                >
                  Resolve
                </button>
              ),
          },
        ]}
      />
    </>
  )
}
