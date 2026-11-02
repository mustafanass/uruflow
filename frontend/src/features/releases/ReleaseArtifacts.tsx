import { type Release } from '@/shared/types'
import { Code, DataTable, Panel } from '@/shared/ui'

export function ReleaseArtifacts({ r }: { r: Release }) {
  const artifacts = Object.entries(r.images || {})
  if (!artifacts.length && r.image) {
    artifacts.push([r.project, r.image])
  }
  return (
    <Panel
      title="Service artifacts"
      detail="Exact image references and resolved source commits for this release."
    >
      <DataTable
        scope="release-artifacts"
        rows={artifacts}
        rowKey={(a) => a[0]}
        empty="No artifacts recorded yet"
        columns={[
          {
            key: 'service',
            title: 'Service',
            render: (a) => <strong>{a[0] || r.project}</strong>,
          },
          {
            key: 'image',
            title: 'Immutable image',
            render: (a) => <Code value={a[1]} copy />,
          },
          {
            key: 'commit',
            title: 'Source commit',
            render: (a) => <Code value={r.commits?.[a[0]] || r.commit || ''} />,
          },
        ]}
      />
    </Panel>
  )
}
