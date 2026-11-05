import { Uptime } from '@/shared/ui/RelativeTime'
import { useWorkspace } from '@/shared/hooks'
import type { Agent } from '@/shared/types'
import { Code, Facts, HostResources, MetricChart, Panel } from '@/shared/ui'
import { bytes, time } from '@/shared/utils'

export function AgentOverview({ a }: { a: Agent }) {
  const { samples } = useWorkspace()
  return (
    <>
      <div className="two-columns">
        <Panel title="Machine identity">
          <Facts
            items={[
              ['Agent ID', <Code value={a.id} copy />],
              ['Reported address', a.host],
              ['Agent version', a.version],
              ['Last contact', time(a.last_seen)],
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
          {a.metrics?.errors?.map((error) => (
            <p className="collection-note" key={error}>
              {error}
            </p>
          ))}
        </Panel>
      </div>
      <div className="two-columns">
        <MetricChart
          title="Host CPU"
          field="cpu"
          samples={samples('agent/' + a.id)}
          format={(v) => v.toFixed(1) + '%'}
        />
        <MetricChart
          title="Host memory"
          field="memory"
          samples={samples('agent/' + a.id)}
          format={bytes}
        />
      </div>
      <p className="subtle-note">
        Trends begin when this workspace opens. Host CPU is normalized to total
        machine capacity.
      </p>
    </>
  )
}
