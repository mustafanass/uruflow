import { Uptime } from '@/shared/ui/RelativeTime'
import { useWorkspace } from '@/shared/hooks'
import {
  Badge,
  Code,
  Facts,
  HostResources,
  MetricChart,
  Panel,
  Spinner,
} from '@/shared/ui'
import { bytes, time } from '@/shared/utils'

export function SystemOverview() {
  const { snapshot, session, fleetAccess, samples } = useWorkspace()
  return snapshot ? (
    <>
      <div className="two-columns">
        <Panel title="Control plane">
          <Facts
            items={[
              ['Server', snapshot.server_name],
              ['Version', snapshot.version],
              ['Operator interface', window.location.origin],
              ['Agent transport', 'UFP over TLS'],
              [
                'Registry',
                fleetAccess ? (
                  <Code value={snapshot.registry_address} />
                ) : (
                  'Fleet access required'
                ),
              ],
              ['Registry reachability', <Badge value={snapshot.registry} />],
              ['Latest observation', time(snapshot.at)],
            ]}
          />
        </Panel>
        {fleetAccess ? (
          <Panel>
            <HostResources
              metrics={snapshot.host}
              label="Control-plane host resources"
            />
            <Facts
              items={[
                ['Logical CPUs', snapshot.host?.cpu_cores || 'Unavailable'],
                [
                  'Host uptime',
                  snapshot.host?.host_uptime ? (
                    <Uptime
                      seconds={snapshot.host.host_uptime}
                      observedAt={snapshot.host.collected_at}
                    />
                  ) : (
                    'Unavailable'
                  ),
                ],
                [
                  'Control-plane uptime',
                  snapshot.host?.uptime ? (
                    <Uptime
                      seconds={snapshot.host.uptime}
                      observedAt={snapshot.host.collected_at}
                    />
                  ) : (
                    'Unavailable'
                  ),
                ],
                [
                  'Load averages',
                  snapshot.host?.load_avg
                    ?.map((n) => n.toFixed(2))
                    .join(' / ') || 'Unavailable',
                ],
              ]}
            />
            {snapshot.host?.errors?.map((error) => (
              <p className="collection-note" key={error}>
                {error}
              </p>
            ))}
          </Panel>
        ) : (
          <Panel title="Project-scoped access">
            <p className="panel-note">
              Fleet access is required for server metrics and the registry.
            </p>
            <Facts
              items={[
                [
                  'Project assignments',
                  session.user?.projects?.join(', ') || 'None',
                ],
                [
                  'Session policy',
                  '30-minute idle timeout; eight-hour maximum',
                ],
              ]}
            />
          </Panel>
        )}
      </div>
      {fleetAccess && (
        <details className="details-panel">
          <summary>Control-plane resource trends</summary>
          <div className="two-columns">
            <MetricChart
              title="Control-plane CPU"
              samples={samples('server')}
              field="cpu"
              format={(v) => v.toFixed(1) + '%'}
            />
            <MetricChart
              title="Control-plane memory"
              samples={samples('server')}
              field="memory"
              format={bytes}
            />
          </div>
        </details>
      )}
    </>
  ) : (
    <Spinner />
  )
}
