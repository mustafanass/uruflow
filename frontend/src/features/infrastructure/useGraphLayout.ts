import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import {
  MarkerType,
  useNodesState,
  type Edge,
  type OnNodesChange,
} from '@xyflow/react'

import { type Graph } from '@/shared/types'

import {
  equalPositions,
  readPositions,
  writePositions,
  type Positions,
} from './graph-layout'
import { layout, nodeSize } from './layout'
import { type ResourceFlowNode } from './ResourceNode'
import { graphConnections } from './graph-connections'

export function useGraphLayout({
  graph,
  base,
  view,
  storageKey,
  selected,
  select,
  projectName,
  expanded,
  toggle,
  togglable,
  filtered,
  hovered,
  hoveredEdge,
  allConnections,
  live,
}: {
  graph: Graph
  base: Graph
  view: Graph
  storageKey: string
  selected: string | null
  select: (id: string | null) => void
  projectName?: string
  expanded: ReadonlySet<string>
  toggle: (id: string) => void
  togglable: boolean
  filtered: boolean
  hovered: string | null
  hoveredEdge: string | null
  allConnections: boolean
  live: boolean
}) {
  const [nodes, setNodes, changeNodes] = useNodesState<ResourceFlowNode>([])
  const [initial] = useState(() => readPositions(storageKey, nodeSize))
  const saved = useRef<Positions>(initial)
  const draft = useRef<Positions>({ ...saved.current })
  const [dirty, setDirty] = useState(false)
  const layoutGraph = filtered ? view : base
  const signature = JSON.stringify([
    layoutGraph.nodes.map((node) => node.id),
    layoutGraph.edges.map((edge) => [edge.id, edge.source, edge.target]),
  ])
  const defaults = useMemo(() => layout(layoutGraph), [signature])
  useEffect(() => {
    setNodes((existing) => {
      const byID = new Map(existing.map((node) => [node.id, node]))
      return view.nodes.map((resource) => {
        const old = byID.get(resource.id)
        const position = draft.current[resource.id] ||
          defaults.get(resource.id) || { x: 0, y: 0 }
        const same =
          old &&
          Object.keys(resource).every((key) =>
            Object.is(
              resource[key as keyof typeof resource],
              old.data.resource[key as keyof typeof resource],
            ),
          )
        const data = {
          resource: same ? old.data.resource : resource,
          project: projectName,
          expanded: expanded.has(resource.id),
          toggle: resource.kind === 'agent' ? toggle : undefined,
          togglable,
        }
        if (
          old &&
          same &&
          old.position.x === position.x &&
          old.position.y === position.y &&
          old.selected === (resource.id === selected) &&
          old.data.expanded === data.expanded &&
          old.data.togglable === data.togglable
        )
          return old
        return {
          ...old,
          id: resource.id,
          type: 'resource',
          ariaLabel: resource.kind + ' ' + resource.name,
          position,
          data,
          selected: resource.id === selected,
          width: nodeSize.width,
          height: nodeSize.height,
          style: { width: nodeSize.width, height: nodeSize.height },
        }
      })
    })
  }, [
    view,
    defaults,
    selected,
    setNodes,
    projectName,
    expanded,
    toggle,
    togglable,
  ])
  const edgeSignature = JSON.stringify(view.edges)
  const stateSignature = JSON.stringify(
    view.nodes.map((node) => [node.id, node.state]),
  )
  const edges: Edge[] = useMemo(
    () =>
      graphConnections(view, {
        selected,
        hovered,
        hoveredEdge,
        all: allConnections,
        live,
      }).map((edge) => ({
        id: edge.id,
        source: edge.source,
        target: edge.target,
        type: 'resource',
        label: edge.label || undefined,
        hidden: edge.hidden,
        className:
          'graph-connection' +
          (edge.muted ? ' is-muted' : '') +
          (edge.inactive ? ' is-inactive' : ''),
        data: { motion: edge.motion },
        focusable: false,
        selectable: false,
        style: {
          stroke:
            edge.kind === 'dependency'
              ? 'var(--graph-dependency)'
              : edge.highlighted
                ? 'var(--blue)'
                : 'var(--graph-edge)',
          strokeWidth: edge.highlighted ? 2 : 1.4,
          strokeDasharray: ['dependency', 'assignment'].includes(edge.kind)
            ? '5 4'
            : undefined,
        },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color:
            edge.kind === 'dependency'
              ? 'var(--graph-dependency)'
              : edge.highlighted
                ? 'var(--blue)'
                : 'var(--graph-edge)',
        },
        labelStyle: { fill: 'var(--text)', fontSize: 11 },
        labelBgStyle: { fill: 'var(--surface-subtle)' },
      })),
    [
      edgeSignature,
      stateSignature,
      selected,
      hovered,
      hoveredEdge,
      allConnections,
      live,
    ],
  )

  const onNodesChange: OnNodesChange<ResourceFlowNode> = useCallback(
    (changes) => {
      changeNodes(changes)
      for (const change of changes) {
        if (change.type === 'select' && change.selected) {
          select(change.id)
        }
        if (change.type === 'position' && change.position) {
          draft.current[change.id] = change.position
          setDirty(!equalPositions(draft.current, saved.current))
        }
      }
    },
    [changeNodes, select],
  )

  const save = useCallback(() => {
    const ids = new Set(graph.nodes.map((node) => node.id))
    const positions = Object.fromEntries(
      Object.entries(draft.current).filter(([id]) => ids.has(id)),
    )
    writePositions(storageKey, positions, nodeSize)
    saved.current = positions
    draft.current = { ...positions }
    setDirty(false)
  }, [graph.nodes])

  function reset() {
    localStorage.removeItem(storageKey)
    saved.current = {}
    draft.current = {}
    setNodes((existing) =>
      existing.map((node) => ({
        ...node,
        position: defaults.get(node.id) || { x: 0, y: 0 },
      })),
    )
    setDirty(false)
  }

  const settled =
    nodes.length === view.nodes.length &&
    nodes.every((node, index) => {
      const expected = view.nodes[index]
      const position = draft.current[node.id] || defaults.get(node.id)
      return (
        node.id === expected.id &&
        position?.x === node.position.x &&
        position?.y === node.position.y
      )
    })

  return {
    nodes,
    edges,
    onNodesChange,
    dirty,
    save,
    reset,
    settled,
    customized: Object.keys(saved.current).length > 0,
  }
}
