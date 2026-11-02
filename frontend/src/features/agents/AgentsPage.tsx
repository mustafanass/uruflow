import { withTab } from '@/shared/utils/url'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { ContainerTable } from '@/features/containers/ContainerTable'

import { Download, Plus, Server } from 'lucide-react'

import { ResourceCell } from '@/features/agents/AgentResourceCell'
import { EnrollAgent } from '@/features/agents/EnrollAgent'
import { segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import {
  Badge,
  DataTable,
  PageHeader,
  Panel,
  Spinner,
  Tabs,
  TabPanel,
} from '@/shared/ui'

export default function AgentsPage() {
  const { snapshot, canAdmin, canOperate } = useWorkspace()
  const [enrolling, setEnrolling] = useState(false)
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'Containers' ? 'Containers' : 'Agents'
  return (
    <>
      <PageHeader
        title="Agents"
        actions={
          <>
            {canOperate && (
              <a
                className="button"
                href="/api/trust-root"
                download="uruflow-ca.crt"
              >
                <Download size={15} />
                CA certificate
              </a>
            )}
            {canAdmin && (
              <button
                className="button primary"
                onClick={() => setEnrolling(true)}
              >
                <Plus size={17} />
                Enroll agent
              </button>
            )}
          </>
        }
      />
      <Tabs
        items={['Agents', 'Containers']}
        value={tab}
        onChange={(value) =>
          setParams((current) => withTab(current, value, 'Agents'))
        }
      />
      {snapshot ? (
        <TabPanel value={tab}>
          {tab === 'Containers' ? (
            <Panel title="Fleet containers">
              <ContainerTable containers={snapshot.containers} />
            </Panel>
          ) : (
            <Panel>
              <DataTable
                scope="agents-page"
                rows={snapshot.agents}
                rowKey={(a) => a.id}
                rowHref={(a) => '/agents/' + segment(a.id)}
                searchText={(a) =>
                  a.name +
                  ' ' +
                  a.hostname +
                  ' ' +
                  a.roles.join(' ') +
                  ' ' +
                  a.status
                }
                empty={
                  canAdmin
                    ? 'Connect your first machine'
                    : 'No agents available'
                }
                emptyDetail={
                  canAdmin
                    ? 'Enroll a builder to build images and a runner to host your services.'
                    : 'Your administrator can enroll agents for your projects.'
                }
                columns={[
                  {
                    key: 'name',
                    title: 'Agent',
                    render: (a) => (
                      <div className="resource-title">
                        <span className="resource-icon agent">
                          <Server size={18} />
                        </span>
                        <div>
                          <strong>{a.name}</strong>
                          <small>
                            {a.hostname ||
                              a.host ||
                              'Waiting for first connection'}
                          </small>
                        </div>
                      </div>
                    ),
                    sort: (a) => a.name,
                  },
                  {
                    key: 'role',
                    title: 'Roles',
                    render: (a) => (
                      <div className="role-list">
                        {a.roles.map((r) => (
                          <span className="role-tag" key={r}>
                            {r}
                          </span>
                        ))}
                      </div>
                    ),
                  },
                  {
                    key: 'connection',
                    title: 'Connection',
                    render: (a) => <Badge value={a.status} />,
                    sort: (a) => a.status,
                  },
                  {
                    key: 'cpu',
                    title: 'CPU',
                    render: (a) => <ResourceCell agent={a} kind="cpu" />,
                  },
                  {
                    key: 'memory',
                    title: 'Memory',
                    render: (a) => <ResourceCell agent={a} kind="memory" />,
                  },
                  {
                    key: 'workloads',
                    title: 'Containers',
                    render: (a) =>
                      snapshot.containers.filter((c) => c.agent_id === a.id)
                        .length,
                  },
                  {
                    key: 'sample',
                    title: 'Latest sample',
                    render: (a) => (
                      <span className="muted nowrap">
                        <RelativeTime value={a.metrics?.collected_at} />
                      </span>
                    ),
                  },
                ]}
              />
            </Panel>
          )}
        </TabPanel>
      ) : (
        <Spinner />
      )}
      {enrolling && <EnrollAgent onClose={() => setEnrolling(false)} />}
    </>
  )
}
