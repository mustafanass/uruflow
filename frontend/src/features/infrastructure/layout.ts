import dagre from '@dagrejs/dagre'

import type { Graph } from '@/shared/types'

export const nodeSize = { width: 312, height: 148 }

export function layout(graph: Graph) {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({
    rankdir: 'LR',
    nodesep: 28,
    ranksep: 72,
    marginx: 30,
    marginy: 30,
  })
  graph.nodes.forEach((node) => g.setNode(node.id, { ...nodeSize }))
  graph.edges.forEach((edge) => g.setEdge(edge.source, edge.target))
  dagre.layout(g)
  return new Map(
    graph.nodes.map((node) => {
      const pos = g.node(node.id)
      return [
        node.id,
        { x: pos.x - nodeSize.width / 2, y: pos.y - nodeSize.height / 2 },
      ]
    }),
  )
}
