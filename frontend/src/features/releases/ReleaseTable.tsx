import { ElapsedTime } from '@/shared/ui/RelativeTime'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { GitBranch } from 'lucide-react'

import { segment } from '@/shared/api'
import { type Release } from '@/shared/types'
import { Badge, Code, DataTable } from '@/shared/ui'
import { short } from '@/shared/utils'

export function ReleaseTable({ releases }: { releases: Release[] }) {
  return (
    <DataTable
      scope="release-table"
      rows={releases}
      rowKey={(r) => r.id}
      rowHref={(r) => '/releases/' + segment(r.id)}
      searchText={(r) =>
        r.id +
        ' ' +
        r.project +
        ' ' +
        r.status +
        ' ' +
        r.commit +
        ' ' +
        r.trigger
      }
      empty="No releases recorded"
      emptyDetail="Open a project and deploy its saved configuration."
      columns={[
        {
          key: 'release',
          title: 'Release',
          render: (r) => (
            <div className="resource-title">
              <span className="resource-icon release">
                <GitBranch size={18} />
              </span>
              <div>
                <strong>{short(r.id)}</strong>
                <small>{r.project}</small>
              </div>
            </div>
          ),
          sort: (r) => r.started_at,
        },
        {
          key: 'state',
          title: 'State',
          render: (r) => <Badge value={r.status} />,
          sort: (r) => r.status,
        },
        {
          key: 'trigger',
          title: 'Trigger',
          render: (r) => r.trigger,
          sort: (r) => r.trigger,
        },
        {
          key: 'revision',
          title: 'Revision',
          render: (r) => <Code value={short(r.commit, 10)} />,
        },
        {
          key: 'targets',
          title: 'Runners',
          render: (r) =>
            r.targets?.length ? (
              <div
                className="target-indicators"
                title={r.targets
                  .map((t) => t.agent_name + ': ' + t.status)
                  .join(', ')}
              >
                {r.targets.map((t) => (
                  <i key={t.agent_id} className={t.status} />
                ))}
                <span>
                  {r.targets.filter((t) => t.status === 'succeeded').length}/
                  {r.targets.length}
                </span>
              </div>
            ) : (
              <span className="muted">
                {r.runner_count || r.spec?.runners?.length
                  ? 'Not reported'
                  : 'Build only'}
              </span>
            ),
        },
        {
          key: 'duration',
          title: 'Duration',
          render: (r) => (
            <ElapsedTime
              start={r.started_at}
              elapsed={r.ended_at ? r.duration : undefined}
            />
          ),
        },
        {
          key: 'started',
          title: 'Started',
          render: (r) => (
            <span className="muted nowrap">
              <RelativeTime value={r.started_at} />
            </span>
          ),
          sort: (r) => r.started_at,
        },
      ]}
    />
  )
}
