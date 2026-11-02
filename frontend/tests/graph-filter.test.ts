import assert from 'node:assert/strict'
import { test } from 'node:test'
import { filterGraph } from '../src/features/infrastructure/filter-graph.ts'
import type { Graph } from '../src/shared/types/observation.ts'

const graph = {
  nodes: [
    {
      id: 'server',
      kind: 'server',
      name: 'server',
      parent: '',
      state: 'online',
      summary: '',
    },
    {
      id: 'agent',
      kind: 'agent',
      name: 'runner',
      parent: 'server',
      state: 'online',
      summary: '',
    },
    {
      id: 'container',
      kind: 'container',
      name: 'api',
      parent: 'agent',
      state: 'healthy',
      summary: '',
    },
    {
      id: 'mount',
      kind: 'mount',
      name: 'data',
      parent: 'container',
      state: 'observed',
      summary: '',
    },
  ],
  edges: [
    { id: 'a', source: 'server', target: 'agent' },
    { id: 'b', source: 'agent', target: 'container' },
    { id: 'c', source: 'container', target: 'mount' },
  ],
} as Graph

test('collapsed hosts hide descendants while retaining their identity and connections', () => {
  const result = filterGraph(
    graph,
    undefined,
    '',
    true,
    false,
    null,
    new Set(['agent']),
  )
  assert.deepEqual(
    result.nodes.map((node) => node.id),
    ['server', 'agent'],
  )
  assert.deepEqual(
    result.edges.map((edge) => edge.id),
    ['a'],
  )
  assert.equal(graph.nodes.length, 4)
})

test('search finds a collapsed workload and retains its host ancestors', () => {
  const result = filterGraph(
    graph,
    undefined,
    'api',
    false,
    false,
    null,
    new Set(['agent']),
  )
  assert.deepEqual(
    result.nodes.map((node) => node.id),
    ['server', 'agent', 'container'],
  )
})

test('focus and attachment visibility remain independent of collapsed hosts', () => {
  const result = filterGraph(
    graph,
    undefined,
    '',
    true,
    true,
    graph.nodes[1],
    new Set(['agent']),
  )
  assert.deepEqual(
    result.nodes.map((node) => node.id),
    ['server', 'agent', 'container', 'mount'],
  )
  const withoutAttachments = filterGraph(
    graph,
    undefined,
    '',
    false,
    false,
    null,
  )
  assert.deepEqual(
    withoutAttachments.nodes.map((node) => node.id),
    ['server', 'agent', 'container'],
  )
})

test('each resource has its own non-overlapping layout bounds', async () => {
  const { layout, nodeSize } =
    await import('../src/features/infrastructure/layout.ts')
  const positions = layout(graph)
  assert.equal(positions.size, graph.nodes.length)
  const points = [...positions.values()]
  for (let i = 0; i < points.length; i++)
    for (let j = i + 1; j < points.length; j++)
      assert.ok(
        Math.abs(points[i].x - points[j].x) >= nodeSize.width ||
          Math.abs(points[i].y - points[j].y) >= nodeSize.height,
      )
  assert.deepEqual(nodeSize, { width: 312, height: 148 })
})

test('fleet project search retains its hosts through placement edges', () => {
  const fleet = {
    nodes: [
      ...graph.nodes,
      {
        id: 'project',
        name: 'billing',
        kind: 'project',
        parent: '',
        state: 'undeployed',
        summary: '',
      },
    ],
    edges: [
      ...graph.edges,
      {
        id: 'placement',
        source: 'agent',
        target: 'project',
        kind: 'assignment',
      },
    ],
  } as Graph
  const result = filterGraph(fleet, undefined, 'billing', false, false, null)
  assert.deepEqual(
    result.nodes.map((node) => node.id),
    ['server', 'agent', 'project'],
  )
  assert.deepEqual(
    result.edges.map((edge) => edge.id),
    ['a', 'placement'],
  )
})

test('fleet host search shows its projects without adding a direct server link', () => {
  const fleet = {
    nodes: [
      ...graph.nodes,
      {
        id: 'project',
        name: 'billing',
        kind: 'project',
        parent: '',
        state: 'undeployed',
        summary: '',
      },
    ],
    edges: [
      ...graph.edges,
      {
        id: 'placement',
        source: 'agent',
        target: 'project',
        kind: 'placement',
      },
    ],
  } as Graph
  const result = filterGraph(fleet, undefined, 'runner', false, false, null)
  assert.deepEqual(
    result.nodes.map((node) => node.id),
    ['server', 'agent', 'project'],
  )
  assert.equal(
    result.edges.some(
      (edge) => edge.source === 'server' && edge.target === 'project',
    ),
    false,
  )
})
