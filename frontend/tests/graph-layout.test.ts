import assert from 'node:assert/strict'
import { afterEach, test } from 'node:test'

import {
  equalPositions,
  readPositions,
} from '../src/features/infrastructure/graph-layout.ts'

const original = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
afterEach(() => {
  if (original) {
    Object.defineProperty(globalThis, 'localStorage', original)
  } else {
    Reflect.deleteProperty(globalThis, 'localStorage')
  }
})

function storage(value: string | null) {
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: { getItem: () => value },
  })
}

test('restores finite saved coordinates including negative positions', () => {
  storage('{"service/api":{"x":-120,"y":46}}')
  assert.deepEqual(readPositions('account/project/runtime'), {
    'service/api': { x: -120, y: 46 },
  })
})

test('ignores corrupt, oversized, and missing coordinates', () => {
  storage(
    '{"valid":{"x":0,"y":0},"string":{"x":"10","y":2},"missing":{"x":1},"null":null,"huge":{"x":1e20,"y":1}}',
  )
  assert.deepEqual(readPositions('layout'), { valid: { x: 0, y: 0 } })
})

test('uses automatic layout for malformed JSON and non-object data', () => {
  for (const value of ['broken', 'null', '[]', 'true', '4', '"text"', null]) {
    storage(value)
    assert.deepEqual(readPositions('layout'), {})
  }
})

test('continues without persistence when browser storage is denied', () => {
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    get() {
      throw new Error('Storage denied')
    },
  })
  assert.deepEqual(readPositions('layout'), {})
})

test('reads only the requested account and graph scope', () => {
  let requested = ''
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: {
      getItem(key: string) {
        requested = key
        return null
      },
    },
  })
  readPositions('viewer/commerce/delivery')
  assert.equal(requested, 'viewer/commerce/delivery')
})

test('detects moved and newly arranged nodes independently of key order', () => {
  const saved = { a: { x: 1, y: 2 }, b: { x: 4, y: 5 } }
  assert.equal(
    equalPositions(saved, { b: { x: 4, y: 5 }, a: { x: 1, y: 2 } }),
    true,
  )
  assert.equal(equalPositions(saved, { ...saved, a: { x: 2, y: 2 } }), false)
  assert.equal(equalPositions(saved, { a: saved.a }), false)
})

test('migrates old saved coordinates to larger cards without discarding the layout', () => {
  storage('{"service/api":{"x":276,"y":124},"negative":{"x":-276,"y":-124}}')
  const migrated = readPositions('layout', { width: 312, height: 148 })
  assert.deepEqual(migrated, {
    'service/api': { x: 312, y: 148 },
    negative: { x: -312, y: -148 },
  })
  storage(
    JSON.stringify({
      version: 2,
      width: 312,
      height: 148,
      positions: migrated,
    }),
  )
  assert.deepEqual(
    readPositions('layout', { width: 312, height: 148 }),
    migrated,
  )
})

test('fitting uses the visible canvas size while the graph is expanding', async () => {
  const { fitGraph } =
    await import('../src/features/infrastructure/graph-options.ts')
  let viewport
  const instance = {
    getNodes: () => [{ data: { resource: { kind: 'agent' } } }],
    getNodesBounds: () => ({ x: 30, y: 30, width: 1080, height: 676 }),
    setViewport: async (value) => {
      viewport = value
    },
  }
  const canvas = { clientWidth: 1414, clientHeight: 834 }
  await fitGraph(instance, canvas)
  assert.ok(viewport.zoom > 1)
  assert.ok(
    Math.abs(viewport.x + (30 + 540) * viewport.zoom - canvas.clientWidth / 2) <
      1,
  )
  assert.ok(
    Math.abs(
      viewport.y + (30 + 338) * viewport.zoom - canvas.clientHeight / 2,
    ) < 1,
  )
})

test('phone fleet previews fit the map while project graphs retain a readable scale', async () => {
  const { fitGraph } =
    await import('../src/features/infrastructure/graph-options.ts')
  const fit = async (kind) => {
    let viewport
    const instance = {
      getNodes: () => [{ data: { resource: { kind } } }],
      getNodesBounds: () => ({ x: 30, y: 30, width: 1080, height: 676 }),
      setViewport: async (value) => {
        viewport = value
      },
    }
    await fitGraph(instance, { clientWidth: 358, clientHeight: 300 })
    return viewport.zoom
  }
  assert.equal(await fit('server'), 0.3)
  assert.equal(await fit('builder'), 0.7)
  assert.equal(await fit('runner'), 0.7)
})
