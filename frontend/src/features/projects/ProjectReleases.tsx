import { ReleaseTable } from '@/features/releases/ReleaseTable'
import { segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { type Release } from '@/shared/types'
import { ErrorState, Panel, Spinner } from '@/shared/ui'

export function ProjectReleases({ name }: { name: string }) {
  const { snapshot } = useWorkspace()
  const { data, error, loading, reload } = useLoad<Release[]>(
    '/api/projects/' + segment(name) + '/releases',
  )
  const byID = new Map((data || []).map((r) => [r.id, r]))
  for (const r of snapshot?.releases || []) {
    if (r.project === name) {
      byID.set(r.id, r)
    }
  }
  const merged = [...byID.values()].sort(
    (a, b) => Date.parse(b.started_at) - Date.parse(a.started_at),
  )
  return error ? (
    <ErrorState message={error} retry={reload} />
  ) : loading && !data ? (
    <Spinner />
  ) : (
    <Panel>
      <ReleaseTable releases={merged} />
    </Panel>
  )
}
