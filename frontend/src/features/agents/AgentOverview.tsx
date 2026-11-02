import { Uptime } from '@/shared/ui/RelativeTime'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useWorkspace } from '@/shared/hooks'
import type { Agent } from '@/shared/types'
import { Code, Facts, HostResources, Panel } from '@/shared/ui'
import { time } from '@/shared/utils'

export function AgentOverview({ a }: { a: Agent }) {
  const { snapshot } = useWorkspace()
  if (!snapshot) {
    return null
  }
  const workloads = snapshot.containers.filter((c) => c.agent_id === a.id)
  const projects = snapshot.projects.filter(
    (p) => p.builder === a.id || p.runners?.includes(a.id),
  )
  return (
    <div className="two-columns">
      <Panel title="Machine identity">
        <Facts
          items={[
            ['Agent ID', <Code value={a.id} copy />],
            ['Hostname', a.hostname],
            ['Reported address', a.host],
            ['Platform', a.platform],
            ['Agent version', a.version],
            ['Roles', a.roles.join(', ')],
            ['Last contact', time(a.last_seen)],
            ['Latest sample', <RelativeTime value={a.metrics?.collected_at} />],
            [
              'Host uptime',
              a.metrics?.host_uptime ? (
                a.status === 'online' ? (
                  <Uptime
                    seconds={a.metrics.host_uptime}
                    observedAt={a.metrics.collected_at}
                  />
                ) : (
                  'Unavailable'
                )
              ) : (
                'Unavailable'
              ),
            ],
            [
              'Agent uptime',
              a.metrics?.uptime ? (
                a.status === 'online' ? (
                  <Uptime
                    seconds={a.metrics.uptime}
                    observedAt={a.metrics.collected_at}
                  />
                ) : (
                  'Unavailable'
                )
              ) : (
                'Unavailable'
              ),
            ],
            ['Registered', time(a.registered_at)],
          ]}
        />
      </Panel>
      <div className="stack">
        <Panel>
          <HostResources
            metrics={a.metrics}
            label="Last reported host resources"
          />
          {a.status !== 'online' && (
            <div className="inline-warning">
              The agent is disconnected. Its workloads may still be running;
              current state is unknown.
            </div>
          )}
        </Panel>
        <Panel title="Assignments">
          <div className="assignment-summary">
            <span>
              <strong>{workloads.length}</strong>
              {workloads.length === 1
                ? 'observed container'
                : 'observed containers'}
            </span>
            <span>
              <strong>{projects.length}</strong>
              {projects.length === 1
                ? 'project assignment'
                : 'project assignments'}
            </span>
          </div>
          {a.metrics?.errors?.map((error) => (
            <p className="collection-note" key={error}>
              {error}
            </p>
          ))}
        </Panel>
      </div>
    </div>
  )
}
