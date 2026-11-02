import type { Graph } from '@/shared/types'

interface Options {
  selected?: string | null
  hovered?: string | null
  hoveredEdge?: string | null
  all?: boolean
  live?: boolean
}

const inactiveStates = new Set([
  'offline',
  'unknown',
  'missing',
  'undeployed',
  'failed',
  'unhealthy',
  'degraded',
  'stopped',
  'exited',
  'dead',
  'completed',
  'unavailable',
])

export function graphConnections(
  graph: Graph,
  { selected, hovered, hoveredEdge, all = false, live = false }: Options,
) {
  const nodes = new Map(graph.nodes.map((node) => [node.id, node]))
  return graph.edges.map((edge) => {
    const secondary = ['dependency', 'assignment'].includes(edge.kind)
    const inspected =
      !!selected && [edge.source, edge.target].includes(selected)
    const highlighted =
      inspected ||
      (!!hovered && [edge.source, edge.target].includes(hovered)) ||
      edge.id === hoveredEdge
    const inactive =
      !live ||
      [edge.source, edge.target].some((id) =>
        inactiveStates.has(nodes.get(id)?.state || 'unknown'),
      )
    const fleetPlacement =
      edge.kind === 'placement' &&
      nodes.get(edge.source)?.kind === 'agent' &&
      nodes.get(edge.target)?.kind === 'project'
    return {
      ...edge,
      label: highlighted ? edge.label : undefined,
      secondary,
      highlighted,
      hidden: secondary && !all && !highlighted,
      muted: !!selected && !highlighted,
      inactive,
      motion:
        inspected &&
        !secondary &&
        !inactive &&
        (!fleetPlacement || /(?:^| \/ )runs$/.test(edge.label || '')),
    }
  })
}
