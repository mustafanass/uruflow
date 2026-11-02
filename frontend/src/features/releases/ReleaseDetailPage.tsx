import { withTab } from '@/shared/utils/url'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useEffect } from 'react'

import { ArrowUpRight, RotateCcw } from 'lucide-react'

import { api, segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { type Release } from '@/shared/types'
import {
  Badge,
  EmptyState,
  ErrorState,
  LogViewer,
  PageHeader,
  Spinner,
  Tabs,
  TabPanel,
} from '@/shared/ui'
import { short, time } from '@/shared/utils'

import { ReleaseArtifacts } from './ReleaseArtifacts'
import { ReleaseOverview } from './ReleaseOverview'
import { currentRelease } from '@/features/projects/release-state'
import { ReleaseChanges } from './ReleaseChanges'

export default function ReleaseDetailPage() {
  const { id = '' } = useParams()
  const { snapshot, canOperate, confirm } = useWorkspace()
  const { data, error, loading, reload } = useLoad<Release>(
    '/api/releases/' + segment(id),
  )
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const tabs = ['Overview', 'Logs', 'Changes']
  const live = snapshot?.releases.find((r) => r.id === id)
  const r = data?.id === id ? { ...data, ...live, spec: data.spec } : undefined
  const active = !!r && !['succeeded', 'failed', 'skipped'].includes(r.status)
  const requested =
    params.get('tab') === 'Configuration'
      ? 'Changes'
      : ['Runner targets', 'Artifacts'].includes(params.get('tab') || '')
        ? 'Overview'
        : params.get('tab') || ''
  const tab = tabs.includes(requested)
    ? requested
    : active
      ? 'Logs'
      : 'Overview'
  useEffect(() => {
    if (!params.has('tab') && active)
      setParams({ tab: 'Logs' }, { replace: true })
  }, [active, params, setParams])
  if (!r) {
    return error ? (
      <ErrorState message={error} retry={reload} />
    ) : loading ? (
      <Spinner />
    ) : (
      <EmptyState title="Release not found" />
    )
  }
  return (
    <>
      <PageHeader
        title={'Release ' + short(r.id, 16)}
        eyebrow={
          <>
            <Link to={'/projects/' + segment(r.project)}>{r.project}</Link>
            <span>/</span>
            <Link to={'/projects/' + segment(r.project) + '?tab=Releases'}>
              Releases
            </Link>
          </>
        }
        description={`${r.trigger === 'rollback' ? 'Artifact restoration' : r.trigger === 'webhook' ? 'Webhook triggered' : 'Manually triggered'} · Started ${time(r.started_at)}`}
        actions={
          <>
            <Badge value={r.status} />
            {canOperate &&
              r.status === 'succeeded' &&
              !r.artifacts_expired &&
              !currentRelease(snapshot, r) && (
                <button
                  className="button"
                  onClick={() =>
                    confirm({
                      title: 'Restore release ' + short(r.id),
                      description: `Restore ${r.project} using this release’s saved configuration and exact artifacts. No build is performed.`,
                      label: 'Restore release',
                      success: 'Release restoration started',
                      run: async () => {
                        const restored = await api<Release>(
                          '/api/projects/' + segment(r.project) + '/rollback',
                          { method: 'POST', body: { release_id: r.id } },
                        )
                        navigate(
                          '/releases/' + segment(restored.id) + '?tab=Logs',
                        )
                      },
                    })
                  }
                >
                  <RotateCcw size={15} />
                  Restore release
                </button>
              )}
            <Link className="button" to={'/projects/' + segment(r.project)}>
              Open project
              <ArrowUpRight size={15} />
            </Link>
          </>
        }
      />
      {r.artifacts_expired && (
        <p className="subtle-note">
          Artifacts expired under registry retention. Release history and logs
          remain available.
        </p>
      )}
      <Tabs
        items={tabs}
        value={tab}
        onChange={(value) => setParams((current) => withTab(current, value))}
      />
      <TabPanel value={tab}>
        {tab === 'Overview' && (
          <>
            <ReleaseOverview r={r} />
            <ReleaseArtifacts r={r} />
          </>
        )}
        {tab === 'Logs' && (
          <LogViewer url={'/api/releases/' + segment(r.id) + '/logs'} />
        )}
        {tab === 'Changes' && <ReleaseChanges key={r.id} release={r} />}
      </TabPanel>
    </>
  )
}
