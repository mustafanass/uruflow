import assert from 'node:assert/strict'
import { test } from 'node:test'

import { currentRelease } from '../src/features/projects/release-state.ts'
import type { Release } from '../src/shared/types/release.ts'
import type { Snapshot } from '../src/shared/types/observation.ts'
import { count, duration } from '../src/shared/utils/format.ts'

test('formats long uptime without an unbounded minute count', () => {
  assert.equal(duration(5252 * 60000 + 55000), '3d 15h')
  assert.equal(duration(3599000), '59m 59s')
  assert.equal(duration(3600000), '1h 0m')
  assert.equal(duration(86400000), '1d 0h')
  assert.equal(duration(Number.NaN), '—')
})

test('uses singular nouns for one resource and plural nouns for empty fleets', () => {
  assert.equal(count(1, 'service'), '1 service')
  assert.equal(count(1, 'target machine'), '1 target machine')
  assert.equal(count(0, 'runner'), '0 runners')
  assert.equal(count(2, 'loaded line'), '2 loaded lines')
})

test('identifies the current release from containers even when the project is degraded', () => {
  const release = { id: 'r1', project: 'commerce' } as Release
  const snapshot = {
    project_states: { commerce: 'running' },
    containers: [
      { project: 'commerce', state: 'running', details: { release: 'r1' } },
      { project: 'commerce', state: 'running', details: { release: 'r1' } },
      { project: 'another', state: 'running', details: { release: 'r2' } },
    ],
  } as Snapshot
  assert.equal(currentRelease(snapshot, release), true)
  snapshot.containers[1].details!.release = 'r2'
  assert.equal(currentRelease(snapshot, release), false)
  snapshot.containers[1].details = undefined
  assert.equal(currentRelease(snapshot, release), false)
  snapshot.project_states.commerce = 'degraded'
  snapshot.containers[1].details = { release: 'r1' }
  assert.equal(currentRelease(snapshot, release), true)
  snapshot.containers = []
  assert.equal(currentRelease(snapshot, release), false)
  assert.equal(currentRelease(null, release), false)
})
