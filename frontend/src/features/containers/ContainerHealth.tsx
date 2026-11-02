import type { ContainerDetails } from '@/shared/types'
import { Badge, DataTable, Panel } from '@/shared/ui'
import { time } from '@/shared/utils'

export function ContainerHealth({
  details,
}: {
  details: ContainerDetails | undefined
}) {
  if (!details?.probes?.length) return null
  return (
    <>
      <Panel title="Recent Docker health probes">
        <DataTable
          scope="container-health"
          rows={details?.probes || []}
          rowKey={(p) => p.end}
          empty="No health probe results reported"
          emptyDetail="A running container without a continuous healthcheck does not verify application readiness."
          columns={[
            { key: 'time', title: 'Finished', render: (p) => time(p.end) },
            {
              key: 'result',
              title: 'Result',
              render: (p) => (
                <Badge value={p.exit_code === 0 ? 'succeeded' : 'failed'} />
              ),
            },
            {
              key: 'output',
              title: 'Probe output',
              render: (p) => (
                <code className="probe-output">{p.output || 'No output'}</code>
              ),
            },
          ]}
        />
      </Panel>
    </>
  )
}
