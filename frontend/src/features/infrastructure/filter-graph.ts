import type { Graph, GraphNode, Project } from '@/shared/types'

export function filterGraph(
  graph: Graph,
  project: Project | undefined,
  query: string,
  attachments: boolean,
  focused: boolean,
  selected: GraphNode | null,
  collapsed: ReadonlySet<string> = new Set(),
): Graph {
  let visible = graph.nodes
    .filter((n) => n.kind !== 'project' || !project)
    .filter(
      (n) =>
        attachments ||
        query ||
        !['network', 'mount', 'volume'].includes(n.kind),
    )
  const byID = new Map(graph.nodes.map((node) => [node.id, node]))
  if (query) {
    const included = new Set<string>()
    const include = (node: GraphNode | undefined) => {
      let current = node
      while (current && !included.has(current.id)) {
        included.add(current.id)
        current = byID.get(current.parent)
      }
    }
    for (const node of graph.nodes)
      if (
        (node.name + ' ' + node.kind + ' ' + node.state + ' ' + node.summary)
          .toLowerCase()
          .includes(query.toLowerCase())
      ) {
        include(node)
        if (!project && ['project', 'agent'].includes(node.kind)) {
          for (const edge of graph.edges) {
            if (!['assignment', 'placement'].includes(edge.kind)) continue
            if (edge.target === node.id) include(byID.get(edge.source))
            if (edge.source === node.id) include(byID.get(edge.target))
          }
        }
      }
    visible = visible.filter((n) => included.has(n.id))
  }
  if (focused && selected) {
    const neighborhood = new Set([selected.id])
    for (let depth = 0; depth < 2; depth++) {
      const frontier = new Set(neighborhood)
      for (const edge of graph.edges) {
        if (frontier.has(edge.source)) {
          neighborhood.add(edge.target)
        }
        if (frontier.has(edge.target)) {
          neighborhood.add(edge.source)
        }
      }
    }
    visible = visible.filter((n) => neighborhood.has(n.id))
  }
  if (!query && !focused && collapsed.size) {
    visible = visible.filter((node) => {
      let parent = byID.get(node.parent)
      const seen = new Set<string>()
      while (parent && !seen.has(parent.id)) {
        if (collapsed.has(parent.id)) return false
        seen.add(parent.id)
        parent = byID.get(parent.parent)
      }
      return true
    })
  }
  const ids = new Set(visible.map((n) => n.id))
  return {
    nodes: visible,
    edges: graph.edges.filter((e) => ids.has(e.source) && ids.has(e.target)),
  }
}
