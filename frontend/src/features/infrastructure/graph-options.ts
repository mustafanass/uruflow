import type { Edge, ReactFlowInstance } from '@xyflow/react'
import { getViewportForBounds } from '@xyflow/react'
import type { ResourceFlowNode } from './ResourceNode'

export const graphFit = { padding: 0.12, minZoom: 0.3, maxZoom: 1.4 }

export async function fitGraph(
  instance: ReactFlowInstance<ResourceFlowNode, Edge> | null,
  canvas: Element | null | undefined,
) {
  if (!instance || !canvas) return
  const nodes = instance.getNodes()
  if (!nodes.length) return
  const bounds = instance.getNodesBounds(nodes)
  const width = canvas.clientWidth
  const readable = nodes.some((node) => node.data.resource.kind === 'runner')
  const fleet = nodes.some((node) => node.data.resource.kind === 'server')
  const minZoom =
    width < 600
      ? fleet && canvas.clientHeight < 500
        ? graphFit.minZoom
        : 0.7
      : Math.max(
          graphFit.minZoom,
          Math.min(
            readable ? 0.7 : graphFit.minZoom,
            (width - 48) / Math.max(1, bounds.width),
          ),
        )
  const viewport = getViewportForBounds(
    bounds,
    width,
    canvas.clientHeight,
    minZoom,
    graphFit.maxZoom,
    graphFit.padding,
  )
  if (bounds.height * viewport.zoom > canvas.clientHeight - 48)
    viewport.y = 24 - bounds.y * viewport.zoom
  if (bounds.width * viewport.zoom > width - 48)
    viewport.x = 24 - bounds.x * viewport.zoom
  await instance.setViewport(viewport)
}
