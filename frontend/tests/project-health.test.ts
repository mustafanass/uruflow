import assert from 'node:assert/strict'
import { test } from 'node:test'

import { projectHealth } from '../src/features/projects/project-health.ts'
import type { Project } from '../src/shared/types/project.ts'
import type { Snapshot } from '../src/shared/types/observation.ts'

function fixture() {
  const project = {
    name: 'commerce',
    runners: ['runner-a', 'runner-b'],
    workflow: 'build_deploy',
  } as Project
  const definitions = [{ name: 'api' }]
  const snapshot = {
    graphs: {
      commerce: {
        runtime: {
          nodes: [
            { kind: 'service', service: 'api', state: 'unknown' },
            {
              kind: 'container',
              service: 'api',
              agent_id: 'runner-a',
              container_id: 'a1',
              state: 'healthy',
            },
            {
              kind: 'container',
              service: 'api',
              agent_id: 'runner-b',
              container_id: 'b1',
              state: 'unknown',
            },
          ],
        },
      },
    },
    containers: [
      {
        id: 'a1',
        agent_id: 'runner-a',
        project: 'commerce',
        service: 'api',
        state: 'running',
      },
      {
        id: 'b1',
        agent_id: 'runner-b',
        project: 'commerce',
        service: 'api',
        state: 'running',
      },
    ],
  } as Snapshot
  return { project, definitions, snapshot }
}

test('uses graph observations for readiness instead of a cached running state', () => {
  const { project, definitions, snapshot } = fixture()
  const health = projectHealth(project, snapshot, definitions)
  assert.equal(health.ready, 1)
  assert.equal(health.expected, 2)
  assert.equal(health.rows[0].state, 'unknown')
  assert.equal(health.rows[0].placements[1].state, 'unknown')
})

test('completed jobs remain complete after their containers are removed', () => {
  const { project, snapshot } = fixture()
  snapshot.containers = []
  snapshot.graphs.commerce.runtime.nodes = [
    { kind: 'job', service: 'migrate', state: 'running' },
    {
      kind: 'instance',
      service: 'migrate',
      agent_id: 'runner-a',
      state: 'completed',
    },
    {
      kind: 'instance',
      service: 'migrate',
      agent_id: 'runner-b',
      state: 'completed',
    },
  ] as Snapshot['graphs'][string]['runtime']['nodes']
  const health = projectHealth(project, snapshot, [
    { name: 'migrate', mode: 'job' },
  ])
  assert.equal(health.ready, 2)
  assert.equal(health.rows[0].placements[0].state, 'completed')
})

test('counts placement once when old and replacement containers coexist', () => {
  const { project, definitions, snapshot } = fixture()
  snapshot.containers.push({ ...snapshot.containers[0], id: 'old' })
  snapshot.graphs.commerce.runtime.nodes.push({
    kind: 'container',
    service: 'api',
    agent_id: 'runner-a',
    container_id: 'old',
    state: 'unhealthy',
  } as Snapshot['graphs'][string]['runtime']['nodes'][number])
  const health = projectHealth(project, snapshot, definitions)
  assert.equal(health.ready, 0)
  assert.equal(health.expected, 2)
  assert.equal(health.rows[0].placements[0].state, 'unhealthy')
})

test('does not imply readiness without observations or for build-only workloads', () => {
  const { project, definitions, snapshot } = fixture()
  snapshot.graphs = {}
  assert.equal(projectHealth(project, snapshot, definitions).ready, 0)
  project.workflow = 'build_only'
  const health = projectHealth(project, snapshot, definitions)
  assert.equal(health.expected, 0)
  assert.equal(health.rows[0].placements.length, 0)
})
