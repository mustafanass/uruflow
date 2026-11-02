import { RelativeTime } from '@/shared/ui/RelativeTime'
import { Container as ContainerIcon } from 'lucide-react'

import { segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { type Container } from '@/shared/types'
import { Badge, DataTable, Spinner } from '@/shared/ui'
import { bytes, containerState, fresh } from '@/shared/utils'

export function ContainerTable({ containers }: { containers: Container[] }) {
  const { snapshot } = useWorkspace()
  if (!snapshot) {
    return <Spinner />
  }
  return (
    <DataTable
      scope="container-table"
      rows={containers}
      rowKey={(c) => c.agent_id + '/' + c.id}
      rowHref={(c) => `/containers/${segment(c.agent_id)}/${segment(c.id)}`}
      searchText={(c) =>
        c.name +
        ' ' +
        c.project +
        ' ' +
        c.service +
        ' ' +
        snapshot.agents.find((a) => a.id === c.agent_id)?.name +
        ' ' +
        containerState(c, snapshot)
      }
      empty="No managed containers reported"
      emptyDetail="Deployed containers appear after a runner sends a complete inventory."
      columns={[
        {
          key: 'name',
          title: 'Container',
          render: (c) => (
            <div className="resource-title">
              <span className="resource-icon container">
                <ContainerIcon size={18} />
              </span>
              <div>
                <strong>{c.name}</strong>
                <small>
                  {c.project || 'System workload'}
                  {c.service ? ' / ' + c.service : ''}
                </small>
              </div>
            </div>
          ),
          sort: (c) => c.name,
        },
        {
          key: 'state',
          title: 'Observed state',
          render: (c) => <Badge value={containerState(c, snapshot)} />,
          sort: (c) => containerState(c, snapshot),
        },
        {
          key: 'agent',
          title: 'Host',
          render: (c) =>
            snapshot.agents.find((a) => a.id === c.agent_id)?.name ||
            c.agent_id,
        },
        {
          key: 'cpu',
          title: 'CPU',
          render: (c) =>
            c.stats_available &&
            fresh(
              snapshot.agents.find((a) => a.id === c.agent_id),
              c.observed_at,
            ) ? (
              c.cpu_percent.toFixed(1) + '%'
            ) : (
              <span className="muted">Unavailable</span>
            ),
        },
        {
          key: 'memory',
          title: 'Memory',
          render: (c) =>
            c.stats_available &&
            fresh(
              snapshot.agents.find((a) => a.id === c.agent_id),
              c.observed_at,
            ) ? (
              bytes(c.memory_usage)
            ) : (
              <span className="muted">Unavailable</span>
            ),
        },
        {
          key: 'restarts',
          title: 'Restarts',
          render: (c) => (
            <span className={c.restart_count > 3 ? 'warning-text' : ''}>
              {c.restart_count}
            </span>
          ),
          sort: (c) => c.restart_count,
        },
        {
          key: 'sample',
          title: 'Sample',
          render: (c) => (
            <span className="muted nowrap">
              <RelativeTime value={c.observed_at} />
            </span>
          ),
        },
      ]}
    />
  )
}
