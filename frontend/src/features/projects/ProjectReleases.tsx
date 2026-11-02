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
  const merged = [...(data || [])]
  for (const r of snapshot?.releases || [])
    if (r.project === name) {
      const index = merged.findIndex((old) => old.id === r.id)
      if (index < 0) {
        merged.unshift(r)
      } else {
        merged[index] = r
      }
    }
  return error ? (
    <ErrorState message={error} retry={reload} />
  ) : loading && !merged.length ? (
    <Spinner />
  ) : (
    <Panel>
      <ReleaseTable releases={merged} />
    </Panel>
  )
}
