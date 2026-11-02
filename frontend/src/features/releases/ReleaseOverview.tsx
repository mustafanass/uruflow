import { ElapsedTime } from '@/shared/ui/RelativeTime'
import { Link } from 'react-router-dom'

import { Check } from 'lucide-react'

import { segment } from '@/shared/api'
import { type Release } from '@/shared/types'
import { Badge, Code, Facts, Panel } from '@/shared/ui'
import { count, time } from '@/shared/utils'
import { ReleaseTargets } from './ReleaseTargets'

export function ReleaseOverview({ r }: { r: Release }) {
  const buildOnly =
    r.spec?.workflow === 'build_only' ||
    (!r.spec?.runners?.length && !r.runner_count && !r.targets?.length)
  const buildSkipped =
    r.trigger === 'rollback' || r.spec?.workflow === 'deploy_only'
  const hasArtifact = !!r.image || !!Object.keys(r.images || {}).length
  const completed = ['succeeded', 'failed', 'skipped'].includes(r.status)
  const stages = [
    {
      title: 'Build artifacts',
      state: buildSkipped
        ? 'skipped'
        : r.status === 'building'
          ? 'building'
          : hasArtifact
            ? 'succeeded'
            : r.status === 'failed'
              ? 'failed'
              : 'pending',
      detail: buildSkipped
        ? 'Uses existing immutable images'
        : r.builder_name || 'Configured builder',
    },
    {
      title: 'Release to runners',
      state: buildOnly
        ? 'skipped'
        : r.status === 'succeeded'
          ? 'succeeded'
          : r.status === 'releasing'
            ? 'releasing'
            : r.status === 'failed' && hasArtifact
              ? 'failed'
              : 'pending',
      detail: buildOnly
        ? 'Build-only workflow'
        : count(r.targets?.length || r.runner_count || 0, 'target machine'),
    },
    {
      title: 'Result',
      state: completed ? r.status : 'pending',
      detail: completed ? time(r.ended_at) : 'Waiting for all required work',
    },
  ]
  return (
    <>
      <div className="release-timeline">
        {stages.map((stage, index) => (
          <div className={'release-stage ' + stage.state} key={stage.title}>
            <span className="stage-number">
              {stage.state === 'succeeded' ? <Check size={18} /> : index + 1}
            </span>
            <div>
              <h3>{stage.title}</h3>
              <Badge value={stage.state} />
              <p>{stage.detail}</p>
            </div>
          </div>
        ))}
      </div>
      {r.message && (
        <div
          className={r.status === 'failed' ? 'inline-error' : 'inline-warning'}
        >
          {r.message}
        </div>
      )}
      <div className="two-columns">
        <Panel title="Release record">
          <Facts
            items={[
              ['Release ID', <Code value={r.id} copy />],
              [
                'Project',
                <Link to={'/projects/' + segment(r.project)}>{r.project}</Link>,
              ],
              ['State', <Badge value={r.status} />],
              ['Trigger', r.trigger],
              ['Builder', r.builder_name],
              ['Source commit', <Code value={r.commit} copy />],
              ['Workflow', r.spec?.workflow || r.workflow || 'Automatic'],
              ['Started', time(r.started_at)],
              ['Finished', time(r.ended_at)],
              [
                'Duration',
                <ElapsedTime
                  start={r.started_at}
                  elapsed={r.ended_at ? r.duration : undefined}
                />,
              ],
            ]}
          />
        </Panel>
        <ReleaseTargets r={r} />
      </div>
    </>
  )
}
