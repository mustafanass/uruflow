import { withTab } from '@/shared/utils/url'
import { useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'

import { Play, RotateCcw, Square } from 'lucide-react'

import { ActivityTable } from '@/features/activity/ActivityTable'
import { ProjectEditor } from '@/features/projects/ProjectEditor'
import { ProjectReleases } from '@/features/projects/ProjectReleases'
import { RollbackProject } from '@/features/projects/RollbackProject'
import { api, segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { type Release } from '@/shared/types'
import {
  Badge,
  EmptyState,
  PageHeader,
  Panel,
  Spinner,
  Tabs,
  TabPanel,
} from '@/shared/ui'
import { count, services } from '@/shared/utils'

import { ProjectInfrastructure } from './ProjectInfrastructure'
import { ProjectOverview } from './ProjectOverview'
import { currentRelease } from './release-state'

export default function ProjectDetailPage() {
  const { name = '' } = useParams()
  const { snapshot, canOperate, confirm, dirty } = useWorkspace()
  const { data: history } = useLoad<Release[]>(
    '/api/projects/' + segment(name) + '/releases',
    canOperate,
  )
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const tabs = [
    'Overview',
    'Graph',
    'Releases',
    ...(canOperate ? ['Configuration'] : []),
    'Activity',
  ]
  const legacyTab = params.get('tab') || ''
  const requestedTab = ['Infrastructure', 'Services'].includes(legacyTab)
    ? 'Graph'
    : legacyTab === 'Variables'
      ? 'Configuration'
      : legacyTab === 'Metrics'
        ? 'Overview'
        : legacyTab
  const tab = tabs.includes(requestedTab) ? requestedTab : 'Overview'
  const [rolling, setRolling] = useState(false)
  const project = snapshot?.projects.find((p) => p.name === name)
  if (!snapshot) {
    return <Spinner />
  }
  if (!project) {
    return (
      <EmptyState
        title="Project not found"
        detail="This project is not loaded in the current control plane."
        action={
          <Link to="/projects" className="button">
            View projects
          </Link>
        }
      />
    )
  }
  const p = project
  const canRollback = [...(history || []), ...snapshot.releases].some(
    (release) =>
      release.project === p.name &&
      release.status === 'succeeded' &&
      (release.image || Object.keys(release.images || {}).length) &&
      !currentRelease(snapshot, release),
  )
  const hasRunning = snapshot.containers.some(
    (container) =>
      container.project === p.name && container.state === 'running',
  )
  const runners = (p.runners || []).map(
    (id) => snapshot.agents.find((a) => a.id === id)?.name || id,
  )
  const active = snapshot.releases.some(
    (r) =>
      r.project === p.name &&
      !['succeeded', 'failed', 'skipped'].includes(r.status),
  )
  const deploy = () =>
    confirm({
      title: 'Deploy ' + p.name,
      description: `${p.workflow === 'deploy_only' || services(p).every((service) => service.image) ? 'Release the saved images and configuration' : 'Build and release the saved configuration'} for ${p.env || 'this environment'} across ${runners.join(', ') || 'the configured builder'}. You can follow the release in the browser.`,
      label: 'Deploy',
      success: 'Deployment started',
      run: async () => {
        const r = await api<Release>(
          '/api/projects/' + segment(p.name) + '/deploy',
          { method: 'POST', body: {} },
        )
        navigate('/releases/' + segment(r.id) + '?tab=Logs')
      },
    })
  return (
    <>
      <PageHeader
        title={p.name}
        eyebrow={
          <>
            <Link to="/projects">Projects</Link>
            <span>/</span>
            <span>{p.name}</span>
          </>
        }
        description={`${count(services(p).length, 'service')} across ${count(runners.length, 'runner')}`}
        actions={
          <>
            <Badge value={snapshot.project_states[p.name]} />
            {canOperate && (
              <>
                <button
                  className="button"
                  disabled={active || dirty || !canRollback}
                  title={
                    !canRollback
                      ? 'No earlier successful release to restore'
                      : undefined
                  }
                  onClick={() => setRolling(true)}
                >
                  <RotateCcw size={15} />
                  Rollback
                </button>
                <button
                  className="button"
                  aria-label="Stop project"
                  title="Stop project"
                  disabled={active || dirty || !hasRunning}
                  onClick={() =>
                    confirm({
                      title: 'Stop ' + p.name,
                      description: `Stop this project's managed containers on ${runners.join(', ')}. Persistent volumes remain on their hosts.`,
                      label: 'Stop project',
                      success: 'Project stopped',
                      danger: true,
                      run: () =>
                        api('/api/projects/' + segment(p.name) + '/stop', {
                          method: 'POST',
                          body: {},
                        }),
                    })
                  }
                >
                  <Square size={15} />
                  Stop
                </button>
                <button
                  className="button primary"
                  disabled={active || dirty}
                  onClick={deploy}
                >
                  <Play size={15} />
                  {active ? 'Release in progress' : 'Deploy'}
                </button>
              </>
            )}
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
          <ProjectOverview project={p} showMetrics={legacyTab === 'Metrics'} />
        )}
        {tab === 'Graph' && <ProjectInfrastructure p={p} />}
        {tab === 'Releases' && <ProjectReleases name={p.name} />}
        {tab === 'Configuration' &&
          (canOperate ? (
            <div className="stack">
              <ProjectEditor
                key={p.name + '/configuration'}
                project={p}
                variables={false}
              />
              <ProjectEditor
                key={p.name + '/variables'}
                project={p}
                variables
              />
            </div>
          ) : (
            <EmptyState
              title="Operator access required"
              detail="Configuration and variable values are available to operators and administrators."
            />
          ))}
        {tab === 'Activity' && (
          <Panel
            title="Project activity"
            detail="Recent events retained by this control plane."
          >
            <ActivityTable
              entries={snapshot.activity.filter(
                (e) =>
                  e.source === p.name ||
                  snapshot.releases.some(
                    (r) => r.project === p.name && r.id === e.operation,
                  ),
              )}
            />
          </Panel>
        )}
        {rolling && (
          <RollbackProject project={p} onClose={() => setRolling(false)} />
        )}
      </TabPanel>
    </>
  )
}
