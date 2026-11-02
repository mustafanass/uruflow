import type { ContainerDetails } from '@/shared/types'
import { Code, DataTable, Panel } from '@/shared/ui'

export function ContainerStorage({
  details,
}: {
  details: ContainerDetails | undefined
}) {
  return (
    <Panel
      title="Observed mounts"
      detail="Persistent data lives on this host unless the storage driver provides shared storage."
    >
      <DataTable
        scope="container-storage"
        rows={details?.mounts || []}
        rowKey={(m) => m.source + '/' + m.target}
        empty={details ? 'No mounts reported' : 'Mount inspection unavailable'}
        columns={[
          { key: 'type', title: 'Type', render: (m) => m.type || 'mount' },
          {
            key: 'source',
            title: 'Source',
            render: (m) => <Code value={m.source} />,
          },
          {
            key: 'target',
            title: 'Container path',
            render: (m) => <Code value={m.target} />,
          },
          {
            key: 'access',
            title: 'Access',
            render: (m) => (m.read_only ? 'Read only' : 'Read / write'),
          },
        ]}
      />
    </Panel>
  )
}
