import { Link } from 'react-router-dom'
import {
  ArrowUpRight,
  FileText,
  GitCommitHorizontal,
  Rocket,
} from 'lucide-react'
import { segment } from '@/shared/api'
import { useLoad } from '@/shared/hooks'
import type { Release } from '@/shared/types'
import { Badge, Panel, Spinner } from '@/shared/ui'
import { ElapsedTime, RelativeTime } from '@/shared/ui/RelativeTime'
import { short } from '@/shared/utils'

export function ProjectReleaseSummary({
  release: observedRelease,
  project,
}: {
  release?: Release
  project: string
}) {
  const { data, error, loading } = useLoad<Release[]>(
    '/api/projects/' + segment(project) + '/releases',
    !observedRelease,
  )
  const release =
    observedRelease || data?.find((row) => row.project === project)
  return (
    <Panel
      title="Latest release"
      className="project-release-summary"
      actions={
        release && (
          <Link
            className="text-button"
            to={'/releases/' + segment(release.id)}
            aria-label="Inspect latest release"
          >
            <ArrowUpRight size={15} aria-hidden="true" />
          </Link>
        )
      }
    >
      {!release && loading ? (
        <div className="release-summary-loading">
          <Spinner label="Loading release history…" />
        </div>
      ) : !release && error ? (
        <div className="release-summary-empty">
          <strong>Release history unavailable</strong>
          <Link className="text-button" to="?tab=Releases">
            Open release history
            <ArrowUpRight size={13} aria-hidden="true" />
          </Link>
        </div>
      ) : release ? (
        <div className="release-summary-body">
          <div className="release-summary-state">
            <Badge value={release.status} />
            <RelativeTime value={release.started_at} />
          </div>
          <Link
            className="release-summary-id"
            to={'/releases/' + segment(release.id)}
          >
            {short(release.id, 26)}
          </Link>
          <div className="release-summary-details">
            <span>
              <GitCommitHorizontal size={14} aria-hidden="true" />
              {release.commit
                ? short(release.commit, 9)
                : release.trigger === 'rollback'
                  ? 'Artifact restoration'
                  : release.builder
                    ? 'Commit not reported'
                    : 'Prebuilt artifacts'}
            </span>
            <ElapsedTime
              start={release.started_at}
              elapsed={release.ended_at ? release.duration : undefined}
            />
          </div>
          {release.message && (
            <p className="release-summary-message">{release.message}</p>
          )}
          <Link
            className="button small release-summary-logs"
            to={'/releases/' + segment(release.id) + '?tab=Logs'}
          >
            <FileText size={14} aria-hidden="true" />
            {release.ended_at ? 'View release logs' : 'Follow release logs'}
          </Link>
        </div>
      ) : (
        <div className="release-summary-empty">
          <Rocket size={22} aria-hidden="true" />
          <strong>No releases yet</strong>
          <p>The first deployment will appear here.</p>
        </div>
      )}
    </Panel>
  )
}
