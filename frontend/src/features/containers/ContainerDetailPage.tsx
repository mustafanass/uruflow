import { withTab } from '@/shared/utils/url'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'

import { ArrowUpRight, Server } from 'lucide-react'

import { segment } from '@/shared/api'
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

export default function ContainerDetailPage() {
  const { agent = '', id = '' } = useParams()
  const { snapshot } = useWorkspace()
  const [params, setParams] = useSearchParams()
  const observed = snapshot?.containers.find(
    (container) => container.agent_id === agent && container.id === id,
  )
  const [lastKnown, setLastKnown] = useState<Container>()
  useEffect(() => {
    if (observed) setLastKnown(observed)
  }, [observed])
  const tabs = ['Overview', 'Logs', 'Metrics', 'Inspect']
  const tab = tabs.includes(params.get('tab') || '')
    ? params.get('tab')!
    : 'Overview'
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
  const host = snapshot.agents.find((a) => a.id === agent)
  const current = !!observed && fresh(host, c.observed_at)
  const valid = c.stats_available && current
  const details = c.details
  const observation = JSON.stringify(c, null, 2)
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
