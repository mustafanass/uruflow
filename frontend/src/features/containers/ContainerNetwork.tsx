import type { ContainerDetails } from '@/shared/types'
import { Code, DataTable, Panel } from '@/shared/ui'

export function ContainerNetwork({
  details,
}: {
  details: ContainerDetails | undefined
}) {
  return (
    <>
      <Panel
        title="Network attachments"
        detail="Shared attachment is not proof of application traffic."
      >
        <DataTable
          scope="container-network"
          rows={details?.networks || []}
          rowKey={(n) => n.name}
          empty={
            details
              ? 'No network attachments reported'
              : 'Network inspection unavailable'
          }
          columns={[
            {
              key: 'name',
              title: 'Network',
              render: (n) => <strong>{n.name}</strong>,
            },
            {
              key: 'address',
              title: 'Address',
              render: (n) => <Code value={n.address || ''} />,
            },
            {
              key: 'gateway',
              title: 'Gateway',
              render: (n) => <Code value={n.gateway || ''} />,
            },
            {
              key: 'aliases',
              title: 'Aliases',
              render: (n) => (n.aliases || []).join(', ') || '—',
            },
          ]}
        />
      </Panel>
    </>
  )
}
