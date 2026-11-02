import { withTab } from '@/shared/utils/url'
import {
  Link,
  Navigate,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router-dom'
import { Activity, Trash2 } from 'lucide-react'
import { ContainerTable } from '@/features/containers/ContainerTable'
import { ReleaseTable } from '@/features/releases/ReleaseTable'
import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import {
  Badge,
  EmptyState,
  PageHeader,
  Panel,
  Spinner,
  Tabs,
  TabPanel,
} from '@/shared/ui'
import { AgentMetrics } from './AgentMetrics'
import { AgentOverview } from './AgentOverview'
import { AgentProjects } from './AgentProjects'

export default function AgentDetailPage() {
  const { id = '' } = useParams()
  const { snapshot, canAdmin, confirm } = useWorkspace()
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const tabs = ['Overview', 'Containers', 'Metrics']
  const requested = ['Projects', 'Workloads'].includes(params.get('tab') || '')
    ? 'Containers'
    : params.get('tab') || ''
  const tab = tabs.includes(requested) ? requested : 'Overview'
  if (!snapshot) return <Spinner />
  const agent = snapshot.agents.find((value) => value.id === id)
  if (!agent)
    return (
      <EmptyState
        title="Agent not found"
        detail="This agent is no longer enrolled."
      />
    )
  if (params.get('tab') === 'Activity')
    return <Navigate replace to={'/activity?source=' + segment(agent.name)} />
  const containers = snapshot.containers.filter(
    (container) => container.agent_id === id,
  )
  const projects = snapshot.projects.filter(
    (project) => project.builder === id || project.runners?.includes(id),
  )
  return (
    <>
      <PageHeader
        title={agent.name}
        eyebrow={
          <>
            <Link to="/agents">Agents</Link>
            <span>/</span>
            <span>{agent.name}</span>
          </>
        }
        description={`${agent.hostname || agent.host || 'Awaiting connection'} · ${agent.roles.join(' + ')} · ${agent.platform || 'Platform not reported'}`}
        actions={
          <>
            <Badge value={agent.status} />
            <Link
              className="button"
              to={'/activity?source=' + segment(agent.name)}
            >
              <Activity size={15} aria-hidden="true" />
              Activity
            </Link>
          </>
        }
      />
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
            <AgentOverview a={agent} />
            {canAdmin && (
              <Panel title="Remove agent" className="danger-section">
                <div className="danger-action">
                  <p>
                    Revoke this machine’s enrollment. Running containers remain
                    on the host.
                    {projects.length ? ' Unassign its projects first.' : ''}
                  </p>
                  <button
                    className="button danger-subtle"
                    disabled={projects.length > 0}
                    onClick={() =>
                      confirm({
                        title: 'Remove ' + agent.name,
                        description:
                          'Disconnect this agent and revoke its enrollment. Running containers remain.',
                        label: 'Remove agent',
                        danger: true,
                        run: async () => {
                          await api('/api/agents/' + segment(agent.name), {
                            method: 'DELETE',
                          })
                          navigate('/agents')
                        },
                      })
                    }
                  >
                    <Trash2 size={15} aria-hidden="true" />
                    Remove agent
                  </button>
                </div>
              </Panel>
            )}
          </>
        )}
        {tab === 'Containers' && (
          <>
            <AgentProjects projects={projects} a={agent} />
            <Panel title="Runtime containers">
              <ContainerTable containers={containers} />
            </Panel>
            {agent.roles.includes('builder') && (
              <Panel title="Build work">
                <ReleaseTable
                  releases={snapshot.releases.filter(
                    (release) => release.builder === id,
                  )}
                />
              </Panel>
            )}
          </>
        )}
        {tab === 'Metrics' && <AgentMetrics a={agent} />}
      </TabPanel>
    </>
  )
}
