import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useWorkspace } from '@/shared/hooks'
import type { Agent, Container } from '@/shared/types'
import { Facts, MetricChart, Panel } from '@/shared/ui'
import { bytes, fresh } from '@/shared/utils'

export function ContainerMetrics({
  c,
  host,
}: {
  c: Container
  host: Agent | undefined
}) {
  const { samples } = useWorkspace()
  const valid = c.stats_available && fresh(host, c.observed_at)
  const history = samples('container/' + c.agent_id + '/' + c.id)
  const prev = history[history.length - 2],
    latest = history[history.length - 1]
  const elapsed = latest && prev ? (latest.at - prev.at) / 1000 : 0
  const rateValid =
    !!latest &&
    !!prev &&
    elapsed > 0 &&
    elapsed < Math.max(30, (host?.metrics?.metrics_sec || 10) * 3) &&
    latest.rx !== undefined &&
    prev.rx !== undefined &&
    latest.tx !== undefined &&
    prev.tx !== undefined &&
    latest.rx >= prev.rx &&
    latest.tx >= prev.tx
  return (
    <>
      <div className="two-columns">
        <MetricChart
          title="CPU usage"
          field="cpu"
          samples={history}
          format={(v) => v.toFixed(1) + '%'}
        />
        <MetricChart
          title="Memory usage"
          field="memory"
          samples={history}
          format={bytes}
        />
      </div>
      <Panel
        title="Latest resource counters"
        detail="Counters are cumulative for this container. Rates require two valid samples."
      >
        <Facts
          items={[
            ['Network received', valid ? bytes(c.network_rx) : 'Unavailable'],
            ['Network sent', valid ? bytes(c.network_tx) : 'Unavailable'],
            [
              'Receive / send rate',
              rateValid && valid
                ? `${bytes((latest.rx! - prev.rx!) / elapsed)}/s / ${bytes((latest.tx! - prev.tx!) / elapsed)}/s`
                : 'Waiting for valid consecutive samples',
            ],
            ['Block bytes read', valid ? bytes(c.block_read) : 'Unavailable'],
            [
              'Block bytes written',
              valid ? bytes(c.block_write) : 'Unavailable',
            ],
            ['Processes', valid ? c.pids : 'Unavailable'],
            ['Sample', <RelativeTime value={c.observed_at} />],
          ]}
        />
      </Panel>
      <p className="subtle-note">
        Trends begin when this workspace opens. Missing samples and counter
        resets do not produce utilization estimates.
      </p>
    </>
  )
}
