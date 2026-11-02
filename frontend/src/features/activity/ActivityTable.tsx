import { type Activity } from '@/shared/types'
import { Badge, DataTable } from '@/shared/ui'
import { time } from '@/shared/utils'

export function ActivityTable({ entries }: { entries: Activity[] }) {
  return (
    <DataTable
      scope="activity-table"
      rows={[...entries].reverse()}
      rowKey={(e) => String(e.sequence)}
      searchText={(e) =>
        e.source + ' ' + e.message + ' ' + e.operation + ' ' + e.level
      }
      empty="No recent activity"
      emptyDetail="New deployment, connectivity and alert events will appear here."
      columns={[
        {
          key: 'time',
          title: 'Time',
          render: (e) => <span className="nowrap">{time(e.time)}</span>,
          sort: (e) => e.time,
        },
        {
          key: 'source',
          title: 'Source',
          render: (e) => <strong>{e.source || 'Control plane'}</strong>,
          sort: (e) => e.source,
        },
        {
          key: 'event',
          title: 'Event',
          render: (e) => <span className="event-message">{e.message}</span>,
        },
        {
          key: 'level',
          title: 'Level',
          render: (e) => <Badge value={e.level} />,
          sort: (e) => e.level,
        },
      ]}
    />
  )
}
