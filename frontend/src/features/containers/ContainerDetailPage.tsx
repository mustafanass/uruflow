import { withTab } from '@/shared/utils/url'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'

import { ArrowUpRight, RotateCw, Server } from 'lucide-react'

import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { type Container } from '@/shared/types'
import {
  Badge,
  CopyButton,
  EmptyState,
  LogViewer,
  PageHeader,
  Panel,
  Spinner,
  Tabs,
  TabPanel,
} from '@/shared/ui'
import { containerState, fresh } from '@/shared/utils'

import { ContainerHealth } from './ContainerHealth'
import { ContainerMetrics } from './ContainerMetrics'
import { ContainerNetwork } from './ContainerNetwork'
import { ContainerOverview } from './ContainerOverview'
import { ContainerStorage } from './ContainerStorage'
import { ContainerTerminal } from './ContainerTerminal'

export default function ContainerDetailPage() {
  const { agent = '', id = '' } = useParams()
  const { snapshot, canOperate, confirm } = useWorkspace()
  const [params, setParams] = useSearchParams()
  const observed = snapshot?.containers.find(
    (container) => container.agent_id === agent && container.id === id,
  )
  const [lastKnown, setLastKnown] = useState<Container>()
  useEffect(() => {
    if (observed) setLastKnown(observed)
  }, [observed])
  if (!snapshot) {
    return <Spinner />
  }
  const c =
    observed ||
    (lastKnown?.id === id && lastKnown.agent_id === agent
      ? lastKnown
      : undefined)
  if (!c) {
    return (
      <EmptyState
        title="Container is no longer in the current inventory"
        detail="A replacement may have a new identity. Return to the container list to inspect its current instance."
        action={
          <Link to="/agents?tab=Containers" className="button">
            View containers
          </Link>
        }
      />
    )
  }
  const tabs = [
    'Overview',
    'Logs',
    ...(canOperate && c.project ? ['Terminal'] : []),
    'Metrics',
    'Inspect',
  ]
  const tab = tabs.includes(params.get('tab') || '')
    ? params.get('tab')!
    : 'Overview'
  const host = snapshot.agents.find((a) => a.id === agent)
  const current = !!observed && fresh(host, c.observed_at)
  const valid = c.stats_available && current
  const details = c.details
  const observation = JSON.stringify(c, null, 2)
  const releasing = snapshot.releases.some(
    (r) =>
      r.project === c.project &&
      !['succeeded', 'failed', 'skipped'].includes(r.status),
  )
  return (
    <>
      <PageHeader
        title={c.name}
        eyebrow={
          <>
            <Link to="/agents?tab=Containers">Containers</Link>
            <span>/</span>
            <span>{host?.name || agent}</span>
          </>
        }
        description={
          <>
            {c.project || 'System workload'}
            {c.service ? ' / ' + c.service : ''} · Sample{' '}
            <RelativeTime value={c.observed_at} />
          </>
        }
        actions={
          <>
            <Badge value={containerState(c, snapshot)} />
            {c.project && (
              <Link className="button" to={'/projects/' + segment(c.project)}>
                Open project
                <ArrowUpRight size={15} />
              </Link>
            )}
            <Link className="button" to={'/agents/' + segment(agent)}>
              <Server size={15} />
              Open host
            </Link>
            {canOperate && c.project && observed && (
              <button
                className="button"
                disabled={releasing || host?.status !== 'online'}
                title={
                  releasing
                    ? 'A release is in progress for this project'
                    : undefined
                }
                onClick={() =>
                  confirm({
                    title: 'Restart ' + c.name,
                    description:
                      'Docker stops this container gracefully, then starts it again with the same configuration and data. Open connections and terminal sessions end.',
                    label: 'Restart container',
                    success: 'Container restarted',
                    run: () =>
                      api(
                        `/api/containers/${segment(agent)}/${segment(id)}/restart`,
                        { method: 'POST' },
                      ),
                  })
                }
              >
                <RotateCw size={15} />
                Restart
              </button>
            )}
          </>
        }
      />
      {!current && (
        <div className="inline-warning">
          {observed
            ? 'This is the last reported observation. The current workload state is unknown.'
            : 'This container left the inventory. Its last observation and loaded logs remain available here.'}
        </div>
      )}
      <Tabs
        items={tabs}
        value={tab}
        onChange={(value) =>
          setParams((current) => withTab(current, value, 'Overview'))
        }
      />
      <TabPanel value={tab}>
        {tab === 'Overview' && (
          <>
            <ContainerOverview
              host={host}
              c={c}
              valid={valid}
              details={details}
            />
            <div className="two-columns">
              <div className="stack">
                <ContainerNetwork details={details} />
              </div>
              <ContainerStorage details={details} />
            </div>
            <ContainerHealth details={details} />
          </>
        )}
        {tab === 'Terminal' &&
          (observed && c.state === 'running' ? (
            <ContainerTerminal agent={agent} id={id} />
          ) : (
            <EmptyState
              title="Terminal unavailable"
              detail="A shell opens only in a running container from the current inventory."
            />
          ))}
        {tab === 'Metrics' && <ContainerMetrics c={c} host={host} />}
        {tab === 'Logs' && (
          <LogViewer
            url={`/api/containers/${segment(agent)}/${segment(id)}/logs`}
          />
        )}
        {tab === 'Inspect' && (
          <Panel
            title="Collected observation"
            detail="Credential-free runtime data reported by the agent."
            actions={
              <CopyButton value={observation} label="Copy observation JSON" />
            }
          >
            <pre className="json-view">{observation}</pre>
          </Panel>
        )}
      </TabPanel>
    </>
  )
}
