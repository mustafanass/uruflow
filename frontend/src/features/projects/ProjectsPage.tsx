import { withTab } from '@/shared/utils/url'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { ReleaseTable } from '@/features/releases/ReleaseTable'

import { Boxes, Plus, RotateCcw } from 'lucide-react'

import { CreateProject } from '@/features/projects/CreateProject'
import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import {
  Badge,
  Code,
  DataTable,
  LoadingIndicator,
  PageHeader,
  Panel,
  Spinner,
  Tabs,
  TabPanel,
} from '@/shared/ui'
import { count, errorText, services, short } from '@/shared/utils'

export default function ProjectsPage() {
  const { snapshot, canOperate, fleetAccess, notify } = useWorkspace()
  const [creating, setCreating] = useState(false)
  const [reloading, setReloading] = useState(false)
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'Releases' ? 'Releases' : 'Projects'
  return (
    <>
      <PageHeader
        title="Projects"
        actions={
          <>
            {canOperate && fleetAccess && (
              <>
                <button
                  className="button"
                  disabled={reloading}
                  aria-busy={reloading}
                  onClick={async () => {
                    setReloading(true)
                    try {
                      const result = await api<{
                        problems: string[] | null
                      }>('/api/projects/reload', { method: 'POST' })
                      notify(
                        result.problems?.length
                          ? `Reloaded with ${count(result.problems.length, 'configuration problem')}`
                          : 'Project definitions reloaded',
                        result.problems?.length ? 'error' : 'success',
                      )
                    } catch (e) {
                      notify(errorText(e), 'error')
                    } finally {
                      setReloading(false)
                    }
                  }}
                >
                  {reloading ? (
                    <LoadingIndicator small />
                  ) : (
                    <RotateCcw size={15} />
                  )}
                  {reloading ? 'Reloading…' : 'Reload definitions'}
                </button>
                <button
                  className="button primary"
                  onClick={() => setCreating(true)}
                >
                  <Plus size={17} />
                  Create project
                </button>
              </>
            )}
          </>
        }
      />
      <Tabs
        items={['Projects', 'Releases']}
        value={tab}
        onChange={(value) =>
          setParams((current) => withTab(current, value, 'Projects'))
        }
      />
      {snapshot ? (
        <TabPanel value={tab}>
          {tab === 'Releases' ? (
            <Panel title="Fleet releases">
              <ReleaseTable releases={snapshot.releases} />
            </Panel>
          ) : (
            <Panel>
              <DataTable
                scope="projects-page"
                rows={snapshot.projects}
                rowKey={(p) => p.name}
                rowHref={(p) => '/projects/' + segment(p.name)}
                searchText={(p) =>
                  p.name + ' ' + p.env + ' ' + snapshot.project_states[p.name]
                }
                empty={
                  canOperate && fleetAccess
                    ? 'Create your first project'
                    : 'No projects available'
                }
                emptyDetail={
                  canOperate && fleetAccess
                    ? 'Add services and choose the agents that will build or run them.'
                    : 'Ask an administrator for project access.'
                }
                columns={[
                  {
                    key: 'name',
                    title: 'Project environment',
                    render: (p) => (
                      <div className="resource-title">
                        <span className="resource-icon project">
                          <Boxes size={18} />
                        </span>
                        <div>
                          <strong>{p.name}</strong>
                          <small>{p.workflow || 'Automatic workflow'}</small>
                        </div>
                      </div>
                    ),
                    sort: (p) => p.name,
                  },
                  {
                    key: 'state',
                    title: 'Runtime',
                    render: (p) => (
                      <Badge value={snapshot.project_states[p.name]} />
                    ),
                    sort: (p) => snapshot.project_states[p.name],
                  },
                  {
                    key: 'env',
                    title: 'Environment',
                    render: (p) => (
                      <span className="environment-label">
                        {p.env || 'default'}
                      </span>
                    ),
                    sort: (p) => p.env,
                  },
                  {
                    key: 'services',
                    title: 'Services',
                    render: (p) => services(p).length,
                    sort: (p) => services(p).length,
                  },
                  {
                    key: 'runners',
                    title: 'Runners',
                    render: (p) => p.runners?.length || 0,
                    sort: (p) => p.runners?.length || 0,
                  },
                  {
                    key: 'latest',
                    title: 'Latest release attempt',
                    render: (p) => {
                      const r = snapshot.releases.find(
                        (r) => r.project === p.name,
                      )
                      return r ? (
                        <div className="stacked">
                          <Code value={short(r.id)} />
                          <small>
                            {r.status} · <RelativeTime value={r.started_at} />
                          </small>
                        </div>
                      ) : (
                        <span className="muted">No releases yet</span>
                      )
                    },
                  },
                ]}
              />
            </Panel>
          )}
        </TabPanel>
      ) : (
        <Spinner />
      )}
      {creating && <CreateProject onClose={() => setCreating(false)} />}
    </>
  )
}
