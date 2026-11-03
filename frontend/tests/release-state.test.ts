import assert from 'node:assert/strict'
import { test } from 'node:test'
import { currentRelease } from '../src/features/projects/release-state.ts'
import type { Release } from '../src/shared/types/release.ts'
import type { Snapshot } from '../src/shared/types/observation.ts'

const release = (id: string, at: string, containers: Record<string, string>) =>
  ({
    id,
    project: 'app',
    status: 'succeeded',
    started_at: at,
    targets: [{ agent_id: 'runner', status: 'succeeded', containers }],
  }) as Release

test('the latest applied release is live when unchanged containers keep their original labels', () => {
  const old = release('old', '2026-11-01T10:00:00Z', {
    rust: 'rust',
    console: 'console-old',
  })
  const next = release('next', '2026-11-02T10:00:00Z', {
    rust: 'rust',
    console: 'console-new',
  })
  const snapshot = {
    releases: [next, old],
    containers: [
      {
        id: 'rust',
        agent_id: 'runner',
        project: 'app',
        service: 'rust',
        state: 'running',
        details: { release: 'old' },
      },
      {
        id: 'console-new',
        agent_id: 'runner',
        project: 'app',
        service: 'console',
        state: 'running',
        details: { release: 'next' },
      },
    ],
  } as Snapshot
  assert.equal(currentRelease(snapshot, next), true)
  assert.equal(currentRelease(snapshot, old), false)
  const noop = release('noop', '2026-11-02T11:00:00Z', {
    rust: 'rust',
    console: 'console-new',
  })
  snapshot.releases.unshift(noop)
  assert.equal(currentRelease(snapshot, noop), true)
  assert.equal(currentRelease(snapshot, next), false)
  snapshot.containers[1].id = 'unexpected'
  assert.equal(currentRelease(snapshot, noop), false)
})

test('a partially applied or failed release is not marked live', () => {
  const item = release('partial', '2026-11-02T10:00:00Z', {
    rust: 'rust',
    console: 'console-new',
  })
  item.status = 'failed'
  const snapshot = {
    releases: [item],
    containers: [
      {
        id: 'rust',
        agent_id: 'runner',
        project: 'app',
        service: 'rust',
        state: 'running',
      },
    ],
  } as Snapshot
  assert.equal(currentRelease(snapshot, item), false)
  item.status = 'succeeded'
  assert.equal(currentRelease(snapshot, item), false)
})
