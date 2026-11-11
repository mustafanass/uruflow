import { useEffect } from 'react'

import { useLoad } from '@/shared/hooks'
import { useVisible } from '@/shared/hooks/useVisible'
import type { WebhookDelivery } from '@/shared/types'
import { Badge, Code, DataTable, ErrorState, Panel, Spinner } from '@/shared/ui'
import { short, time } from '@/shared/utils'

export function WebhookDeliveries() {
  const visible = useVisible()
  const { data, error, loading, reload } = useLoad<WebhookDelivery[]>(
    '/api/settings/webhook/deliveries',
    visible,
  )
  useEffect(() => {
    if (!visible) return
    const timer = window.setInterval(reload, 10000)
    return () => clearInterval(timer)
  }, [visible, reload])
  if (error) return <ErrorState message={error} retry={reload} />
  if (loading && !data) return <Spinner />
  return (
    <Panel
      title="Recent deliveries"
      detail="Signed pushes from the last 30 days and what each one started."
    >
      <DataTable
        scope="webhook-deliveries"
        rows={data || []}
        rowKey={(d) => d.provider + '/' + d.delivery_id}
        searchText={(d) =>
          d.provider +
          ' ' +
          d.repository +
          ' ' +
          d.branch +
          ' ' +
          d.commit +
          ' ' +
          d.result +
          ' ' +
          d.detail
        }
        empty="No push deliveries yet"
        emptyDetail="Deliveries appear after a repository webhook sends a signed push."
        columns={[
          {
            key: 'received',
            title: 'Received',
            render: (d) => time(d.received_at),
            sort: (d) => d.received_at,
          },
          {
            key: 'provider',
            title: 'Provider',
            render: (d) => d.provider,
            sort: (d) => d.provider,
          },
          {
            key: 'repository',
            title: 'Repository',
            render: (d) => <Code value={d.repository} />,
          },
          {
            key: 'branch',
            title: 'Branch',
            render: (d) => d.branch || '—',
          },
          {
            key: 'commit',
            title: 'Commit',
            render: (d) => <Code value={short(d.commit, 9)} />,
          },
          {
            key: 'result',
            title: 'Result',
            render: (d) => <Badge value={d.result || 'failed'} />,
            sort: (d) => d.result,
          },
          {
            key: 'detail',
            title: 'Detail',
            render: (d) => <span className="event-message">{d.detail}</span>,
          },
        ]}
      />
    </Panel>
  )
}
