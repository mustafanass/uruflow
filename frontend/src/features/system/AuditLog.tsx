import { useEffect } from 'react'
import { useLoad } from '@/shared/hooks'
import { useVisible } from '@/shared/hooks/useVisible'
import type { Audit } from '@/shared/types'
import { Badge, DataTable, ErrorState, Panel, Spinner } from '@/shared/ui'
import { time } from '@/shared/utils'

export function AuditLog() {
  const visible = useVisible()
  const { data, error, loading, reload } = useLoad<Audit[]>(
    '/api/audit',
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
    <Panel title="Audit log">
      <DataTable
        scope="audit-log"
        rows={data || []}
        rowKey={(entry) => String(entry.id)}
        searchText={(entry) =>
          entry.actor +
          ' ' +
          entry.action +
          ' ' +
          entry.resource +
          ' ' +
          entry.outcome
        }
        empty="No operator actions recorded"
        columns={[
          {
            key: 'time',
            title: 'Time',
            render: (entry) => time(entry.created_at),
            sort: (entry) => entry.created_at,
          },
          {
            key: 'actor',
            title: 'Actor',
            render: (entry) => <strong>{entry.actor}</strong>,
            sort: (entry) => entry.actor,
          },
          {
            key: 'action',
            title: 'Action',
            render: (entry) => <code>{entry.action}</code>,
          },
          {
            key: 'resource',
            title: 'Resource',
            render: (entry) => (
              <span className="event-message">{entry.resource}</span>
            ),
          },
          {
            key: 'outcome',
            title: 'Outcome',
            render: (entry) => <Badge value={entry.outcome} />,
            sort: (entry) => entry.outcome,
          },
        ]}
      />
    </Panel>
  )
}
