import assert from 'node:assert/strict'
import { test } from 'node:test'
import { navigation, navigationSection } from '../src/app/navigation.ts'
import { activityTimeline } from '../src/features/activity/activity-timeline.ts'
import { releaseDefinition } from '../src/features/releases/release-definition.ts'
import type { Snapshot } from '../src/shared/types/observation.ts'
import type { Project } from '../src/shared/types/project.ts'

test('five primary destinations retain clear owners for direct resource links', () => {
  assert.deepEqual(
    navigation.map((item) => item.label),
    ['Overview', 'Projects', 'Agents', 'Activity', 'System'],
  )
  assert.equal(navigationSection('/releases/release-1'), '/projects')
  assert.equal(navigationSection('/containers/runner/container-1'), '/agents')
  assert.equal(navigationSection('/registry'), '/system')
  assert.equal(navigationSection('/alerts'), '/activity')
  assert.equal(navigationSection('/projects-other'), '/')
})

test('activity combines release and durable alert events without duplicating their feed records', () => {
  const snapshot = {
    agents: [{ name: 'runner' }],
    releases: [
      {
        id: 'r1',
        project: 'api',
        status: 'succeeded',
        started_at: '2026-11-02T10:00:00Z',
      },
    ],
    activity: [
      {
        sequence: 1,
        time: '2026-11-02T10:00:00Z',
        operation: 'r1',
        source: 'api',
        message: 'Deployed',
        level: 'success',
      },
      {
        sequence: 2,
        time: '2026-11-02T10:01:00Z',
        operation: 'alert1',
        source: 'runner',
        message: 'Raised alert',
        level: 'warning',
      },
      {
        sequence: 3,
        time: '2026-11-02T10:02:00Z',
        operation: '',
        source: 'runner',
        message: 'Agent connected',
        level: 'success',
      },
    ],
  } as Snapshot
  const entries = activityTimeline(snapshot, [
    {
      id: 'alert1',
      project: 'api',
      message: 'High memory',
      created_at: '2026-11-02T10:01:00Z',
      resolved: true,
      resolved_at: '2026-11-02T10:03:00Z',
      severity: 'warning',
      agent_id: 'runner',
      agent_name: 'runner',
      type: 'memory',
    },
  ])
  assert.equal(entries.length, 4)
  assert.equal(entries[0].state, 'resolved')
  assert.equal(entries.filter((entry) => entry.type === 'Deployment').length, 1)
  assert.equal(entries.filter((entry) => entry.type === 'Alert').length, 2)
  assert.equal(
    entries.find((entry) => entry.type === 'Agent')?.source,
    'runner',
  )
  assert.equal(
    entries.find((entry) => entry.type === 'Deployment')?.href,
    '/releases/r1',
  )
})

test('release diffs ignore storage metadata and object-key ordering while preserving service order', () => {
  const a = {
    name: 'api',
    env: 'prod',
    builder: 'b',
    runners: ['r'],
    source: '/a/config.yaml',
    created_at: 'today',
    services: [{ name: 'api', env: { B: 'two', A: '${secret:token}' } }],
  } as Project
  const b = {
    ...a,
    source: '/b/config.yaml',
    created_at: 'yesterday',
    services: [{ name: 'api', env: { A: '${secret:token}', B: 'two' } }],
  }
  assert.equal(releaseDefinition(a), releaseDefinition(b))
  assert.ok(releaseDefinition(a).includes('${secret:token}'))
  assert.notEqual(
    releaseDefinition(a),
    releaseDefinition({ ...b, runners: ['other'] }),
  )
})

test('switching views preserves independent table filters and the selected graph mode', async () => {
  const { withTab } = await import('../src/shared/utils/url.ts')
  const current = new URLSearchParams(
    'tab=Activity&view=delivery&release-table.q=api&projects-page.page=2',
  )
  const releases = withTab(current, 'Releases', 'Overview')
  assert.equal(releases.get('tab'), 'Releases')
  assert.equal(releases.get('view'), 'delivery')
  assert.equal(releases.get('release-table.q'), 'api')
  assert.equal(releases.get('projects-page.page'), '2')
  assert.equal(withTab(releases, 'Overview', 'Overview').has('tab'), false)
  assert.equal(current.get('tab'), 'Activity')
})
