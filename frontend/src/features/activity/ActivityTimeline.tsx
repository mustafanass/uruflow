import { useSearchParams } from 'react-router-dom'
import { useMemo } from 'react'
import { useWorkspace } from '@/shared/hooks'
import { Badge, DataTable } from '@/shared/ui'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import type { Alert } from '@/shared/types'
import { activityTimeline } from './activity-timeline'

export function ActivityTimeline({ alerts }: { alerts: Alert[] }) {
  const { snapshot } = useWorkspace()
  const [params, setParams] = useSearchParams()
  const agentNames = JSON.stringify(snapshot?.agents.map((agent) => agent.name))
  const entries = useMemo(
    () => (snapshot ? activityTimeline(snapshot, alerts) : []),
    [snapshot?.activity, snapshot?.releases, agentNames, alerts],
  )
  if (!snapshot) return null
  const type = params.get('type') || ''
  const source = params.get('source') || ''
  const update = (key: string, value: string) =>
    setParams(
      (current) => {
        const next = new URLSearchParams(current)
        if (value) next.set(key, value)
        else next.delete(key)
        next.delete('activity-timeline.page')
        return next
      },
      { replace: true },
    )
  return (
    <>
      <div className="context-filters">
        <label>
          Event type
          <select
            aria-label="Activity event type"
            name="type"
            value={type}
            onChange={(event) => update('type', event.target.value)}
          >
            <option value="">All events</option>
            {['Deployment', 'Agent', 'Alert', 'System'].map((value) => (
              <option key={value}>{value}</option>
            ))}
          </select>
        </label>
        <label>
          Resource
          <select
            aria-label="Activity resource"
            name="source"
            value={source}
            onChange={(event) => update('source', event.target.value)}
          >
            <option value="">All resources</option>
            {[
              ...new Set([
                ...entries.map((entry) => entry.source),
                ...(source ? [source] : []),
              ]),
            ]
              .sort()
              .map((value) => (
                <option key={value}>{value}</option>
              ))}
          </select>
        </label>
      </div>
      <DataTable
        scope="activity-timeline"
        rows={entries.filter(
          (entry) =>
            (!type || entry.type === type) &&
            (!source || entry.source === source),
        )}
        rowKey={(entry) => entry.id}
        rowHref={(entry) => entry.href}
        searchText={(entry) =>
          entry.message + ' ' + entry.source + ' ' + entry.state
        }
        empty="No matching events"
        columns={[
          {
            key: 'event',
            title: 'Event',
            render: (entry) => (
              <span className="event-message">{entry.message}</span>
            ),
          },
          {
            key: 'source',
            title: 'Resource',
            render: (entry) => <strong>{entry.source}</strong>,
          },
          { key: 'type', title: 'Type', render: (entry) => entry.type },
          {
            key: 'state',
            title: 'State',
            render: (entry) => <Badge value={entry.state} />,
          },
          {
            key: 'time',
            title: 'Time',
            render: (entry) => <RelativeTime value={entry.time} />,
            sort: (entry) => entry.time,
          },
        ]}
      />
    </>
  )
}
