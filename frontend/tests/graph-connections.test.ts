import assert from 'node:assert/strict'
import { test } from 'node:test'

import { graphConnections } from '../src/features/infrastructure/graph-connections.ts'
import type { Graph } from '../src/shared/types/observation.ts'

const graph = {
  nodes: [
    { id: 'server', kind: 'server', state: 'online' },
    { id: 'agent', kind: 'agent', state: 'online' },
    { id: 'project', kind: 'project', state: 'running' },
    { id: 'planned', kind: 'project', state: 'undeployed' },
    { id: 'api', kind: 'service', state: 'running' },
    { id: 'db', kind: 'service', state: 'healthy' },
  ],
  edges: [
    { id: 'link', source: 'server', target: 'agent', kind: 'connection' },
    {
      id: 'placement',
      source: 'agent',
      target: 'project',
      kind: 'placement',
      label: 'runs',
    },
    {
      id: 'assignment',
      source: 'agent',
      target: 'planned',
      kind: 'assignment',
      label: 'assigned',
    },
    {
      id: 'dependency',
      source: 'api',
      target: 'db',
      kind: 'dependency',
      label: 'requires healthy',
    },
  ],
} as Graph

test('the quiet view retains relationships and reveals secondary connections on demand', () => {
  const before = structuredClone(graph)
  const quiet = graphConnections(graph, { live: true })
  assert.deepEqual(
    quiet.filter((edge) => !edge.hidden).map((edge) => edge.id),
    ['link', 'placement'],
  )
  assert.ok(quiet.every((edge) => !edge.label && !edge.motion))
  const all = graphConnections(graph, { all: true, live: true })
  assert.equal(all.filter((edge) => !edge.hidden).length, graph.edges.length)
  assert.ok(all.every((edge) => !edge.label))
  assert.deepEqual(graph, before)
})

test('selection and hover expose dependency conditions without implying active traffic', () => {
  for (const options of [{ selected: 'api' }, { hovered: 'api' }]) {
    const edges = graphConnections(graph, { ...options, live: true })
    const dependency = edges.find((edge) => edge.id === 'dependency')!
    assert.equal(dependency.hidden, false)
    assert.equal(dependency.label, 'requires healthy')
    assert.equal(dependency.motion, false)
  }
  const planned = graphConnections(graph, { selected: 'planned', live: true })
  assert.equal(planned.find((edge) => edge.id === 'assignment')?.hidden, false)
  assert.equal(planned.find((edge) => edge.id === 'assignment')?.motion, false)
})

test('selected live relationships receive motion while other paths are subdued', () => {
  const selected = graphConnections(graph, { selected: 'agent', live: true })
  assert.equal(selected.find((edge) => edge.id === 'placement')?.motion, true)
  assert.equal(selected.find((edge) => edge.id === 'assignment')?.motion, false)
  assert.equal(selected.find((edge) => edge.id === 'dependency')?.muted, true)
  assert.equal(selected.find((edge) => edge.id === 'placement')?.label, 'runs')
  const hovered = graphConnections(graph, {
    hoveredEdge: 'placement',
    live: true,
  })
  assert.equal(hovered.find((edge) => edge.id === 'placement')?.label, 'runs')
  assert.ok(hovered.every((edge) => !edge.motion))
})

test('observed, offline and stale relationships never animate as live placements', () => {
  for (const label of ['observed', 'builds / observed']) {
    const observed = {
      ...graph,
      edges: graph.edges.map((edge) =>
        edge.id === 'placement' ? { ...edge, label } : edge,
      ),
    }
    assert.equal(
      graphConnections(observed, { selected: 'agent', live: true }).find(
        (edge) => edge.id === 'placement',
      )?.motion,
      false,
    )
  }
  const offline = {
    ...graph,
    nodes: graph.nodes.map((node) =>
      node.id === 'agent' ? { ...node, state: 'offline' } : node,
    ),
  }
  assert.ok(
    graphConnections(offline, { selected: 'agent', live: true }).every(
      (edge) => !edge.motion,
    ),
  )
  assert.ok(
    graphConnections(graph, { selected: 'agent', live: false }).every(
      (edge) => !edge.motion,
    ),
  )
})
