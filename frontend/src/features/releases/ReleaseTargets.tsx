import { segment } from '@/shared/api'
import { type Release } from '@/shared/types'
import { Badge, DataTable, Panel } from '@/shared/ui'
import { time } from '@/shared/utils'

export function ReleaseTargets({ r }: { r: Release }) {
  const buildOnly =
    r.spec?.workflow === 'build_only' ||
    (!r.spec?.runners?.length && !r.runner_count && !r.targets?.length)
  return (
    <Panel
      title="Runner outcomes"
      detail="Each runner reports the result of the release procedure."
    >
      <DataTable
        scope="release-targets"
        rows={r.targets || []}
        rowKey={(t) => t.agent_id}
        rowHref={(t) => '/agents/' + segment(t.agent_id)}
        empty={
          buildOnly
            ? 'This release only builds artifacts'
            : 'No runner outcomes recorded'
        }
        columns={[
          {
            key: 'runner',
            title: 'Runner',
            render: (t) => <strong>{t.agent_name || t.agent_id}</strong>,
          },
          {
            key: 'state',
            title: 'State',
            render: (t) => <Badge value={t.status} />,
          },
          {
            key: 'result',
            title: 'Result',
            render: (t) => (
              <span className="event-message">
                {t.message || 'No additional result reported'}
              </span>
            ),
          },
          {
            key: 'finished',
            title: 'Finished',
            render: (t) => time(t.ended_at),
          },
        ]}
      />
    </Panel>
  )
}
