import { withTab } from '@/shared/utils/url'
import { useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'

import { Play, RotateCcw, Square } from 'lucide-react'

import { ActivityTable } from '@/features/activity/ActivityTable'
import { ProjectConfiguration } from './ProjectConfiguration'
import { ProjectPush } from '@/features/projects/ProjectPush'
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
import { currentRelease } from './release-state'
import { DeployProject } from './DeployProject'

export default function ProjectDetailPage() {
  const { name = '' } = useParams()
  const { snapshot, canOperate, confirm, dirty } = useWorkspace()
  const { data: history } = useLoad<Release[]>(
    '/api/projects/' + segment(name) + '/releases',
    canOperate,
  )
  const [params, setParams] = useSearchParams()
  const tabs = [
    'Overview',
    'Releases',
    ...(canOperate ? ['Configuration', 'Settings'] : []),
    'Activity',
  ]
  const legacyTab = params.get('tab') || ''
  const requestedTab = legacyTab === 'Variables' ? 'Configuration' : legacyTab
  const tab = tabs.includes(requestedTab) ? requestedTab : 'Overview'
  const [rolling, setRolling] = useState(false)
  const [deploying, setDeploying] = useState(false)
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
      !release.artifacts_expired &&
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
                  onClick={() => setDeploying(true)}
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
        {tab === 'Overview' && <ProjectInfrastructure p={p} />}
        {tab === 'Releases' && <ProjectReleases name={p.name} />}
        {tab === 'Configuration' &&
          (canOperate ? (
            <ProjectConfiguration
              key={p.name}
              project={p}
              initialSecrets={legacyTab === 'Variables'}
            />
          ) : (
            <EmptyState
              title="Operator access required"
              detail="Configuration and variable values are available to operators and administrators."
            />
          ))}
        {tab === 'Settings' && <ProjectPush key={p.name} name={p.name} />}
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
        {deploying && (
          <DeployProject project={p} onClose={() => setDeploying(false)} />
        )}
      </TabPanel>
    </>
  )
}
