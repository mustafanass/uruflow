import { Link } from 'react-router-dom'

import { segment } from '@/shared/api'
import type { Agent, Container, ContainerDetails } from '@/shared/types'
import { Code, Facts, Panel } from '@/shared/ui'
import { bytes, time } from '@/shared/utils'

export function ContainerOverview({
  c,
  valid,
  details,
  host,
}: {
  c: Container
  valid: boolean
  host?: Agent
  details: ContainerDetails | undefined
}) {
  const agent = c.agent_id
  return (
    <>
      <div className="metric-strip">
        <div>
          <span>CPU usage</span>
          <strong>
            {valid ? c.cpu_percent.toFixed(1) + '%' : 'Unavailable'}
          </strong>
          <small>100% equals one logical core</small>
        </div>
        <div>
          <span>Memory usage</span>
          <strong>{valid ? bytes(c.memory_usage) : 'Unavailable'}</strong>
          <small>Raw usage, including cache</small>
        </div>
        <div>
          <span>Memory limit</span>
          <strong>{valid ? bytes(c.memory_limit) : 'Unavailable'}</strong>
          <small>Reported by Docker</small>
        </div>
        <div>
          <span>Restarts</span>
          <strong>{c.restart_count}</strong>
          <small>For this container instance</small>
        </div>
      </div>
      <div className="two-columns">
        <Panel title="Container identity">
          <Facts
            items={[
              ['Container ID', <Code value={c.id} copy />],
              [
                'Project',
                c.project ? (
                  <Link to={'/projects/' + segment(c.project)}>
                    {c.project}
                  </Link>
                ) : (
                  'System workload'
                ),
              ],
              ['Service', c.service],
              [
                'Agent',
                <Link to={'/agents/' + segment(agent)}>
                  {host?.name || agent}
                </Link>,
              ],
              ['Runtime state', c.state],
              [
                'Health',
                c.health === 'none'
                  ? 'No continuous Docker healthcheck'
                  : c.health,
              ],
              [
                'Ports',
                details?.ports?.length
                  ? details.ports.join(', ')
                  : 'No published bindings',
              ],
              ['Started', time(c.started_at)],
              ['Finished', time(details?.finished_at)],
              ['Created', time(details?.created)],
              ['Last observation', time(c.observed_at)],
            ]}
          />
        </Panel>
        <Panel title="Artifact and release">
          <Facts
            items={[
              ['Image reference', <Code value={c.image} copy />],
              ['Image identity', <Code value={details?.image_id || ''} />],
              [
                'Release',
                details?.release ? (
                  <Link to={'/releases/' + segment(details.release)}>
                    {details.release}
                  </Link>
                ) : (
                  'Unavailable'
                ),
              ],
              ['Workload role', details?.role],
              [
                'Exit code',
                c.state === 'running'
                  ? 'Not exited'
                  : (details?.exit_code ?? 'Unavailable'),
              ],
              [
                'OOM killed',
                details ? String(details.oom_killed) : 'Unavailable',
              ],
              ['Runtime error', details?.error || 'None reported'],
            ]}
          />
          {!details && (
            <p className="collection-note">
              Detailed inspection is unavailable. Update this agent and wait for
              its next inventory.
            </p>
          )}
        </Panel>
      </div>
    </>
  )
}
