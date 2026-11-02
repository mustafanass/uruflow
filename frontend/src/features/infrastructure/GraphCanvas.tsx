import { useEffect, useRef } from 'react'

import {
  Background,
  MiniMap,
  ReactFlow,
  type Edge,
  type OnNodesChange,
  type ReactFlowInstance,
} from '@xyflow/react'

import { EmptyState } from '@/shared/ui'
import type { GraphNode } from '@/shared/types'
import { statusTone } from '@/shared/utils/status'

import { nodeTypes, type ResourceFlowNode } from './ResourceNode'
import { fitGraph, graphFit } from './graph-options'

interface Props {
  nodes: ResourceFlowNode[]
  edges: Edge[]
  onNodesChange: OnNodesChange<ResourceFlowNode>
  select: (id: string | null) => void
  close: () => void
  instance: ReactFlowInstance<ResourceFlowNode, Edge> | null
  initialize: (instance: ReactFlowInstance<ResourceFlowNode, Edge>) => void
  fitKey: string
  settled: boolean
}

export function GraphCanvas({
  nodes,
  edges,
  onNodesChange,
  select,
  close,
  instance,
  initialize,
  fitKey,
  settled,
}: Props) {
  const canvas = useRef<HTMLDivElement>(null)
  const fleet = nodes.some((node) => node.data.resource.kind === 'server')
  useEffect(() => {
    if (!instance || !settled) return
    const frame = requestAnimationFrame(
      () => void fitGraph(instance, canvas.current),
    )
    return () => cancelAnimationFrame(frame)
  }, [instance, fitKey, settled])

  return (
    <div className="graph-canvas" ref={canvas}>
      {nodes.length ? (
        <ReactFlow<ResourceFlowNode, Edge>
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onNodesChange={onNodesChange}
          onInit={initialize}
          onNodeClick={(_, node) => select(node.id)}
          onPaneClick={close}
          nodesConnectable={false}
          edgesReconnectable={false}
          multiSelectionKeyCode={null}
          selectionKeyCode={null}
          deleteKeyCode={null}
          panOnScroll
          panOnScrollSpeed={1}
          zoomOnScroll={false}
          zoomOnPinch
          zoomActivationKeyCode={['Meta', 'Control']}
          onlyRenderVisibleElements
          proOptions={{ hideAttribution: true }}
          minZoom={0.12}
          maxZoom={1.8}
          fitView
          fitViewOptions={graphFit}
        >
          <Background color="var(--graph-grid)" gap={22} size={1} />
          {nodes.length >= 12 && (
            <MiniMap
              pannable
              zoomable
              style={{ width: 150, height: 95 }}
              maskColor="color-mix(in srgb, var(--surface-subtle) 70%, transparent)"
              nodeColor={(node) => {
                const tone = statusTone((node.data.resource as GraphNode).state)
                return tone === 'good'
                  ? 'var(--good)'
                  : tone === 'bad'
                    ? 'var(--bad)'
                    : tone === 'warn'
                      ? 'var(--warn)'
                      : tone === 'active'
                        ? 'var(--blue)'
                        : 'var(--graph-node)'
              }}
            />
          )}
        </ReactFlow>
      ) : (
        <EmptyState
          title="No graph resources match"
          detail="Try a different search or show attachments."
        />
      )}
      <div className="graph-legend">
        <span>
          <i />
          Resource relationship
        </span>
        <span>
          <i className={fleet ? 'assignment' : 'dependency'} />
          {fleet ? 'Configured connection' : 'Deployment dependency'}
        </span>
      </div>
    </div>
  )
}
