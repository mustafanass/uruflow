import { memo } from 'react'

import { type Node, type NodeProps, Handle, Position } from '@xyflow/react'
import {
  type LucideIcon,
  Boxes,
  Container,
  Database,
  GitBranch,
  HardDrive,
  Layers,
  Network,
  Package,
  Server,
  Workflow,
} from 'lucide-react'

import type { GraphNode } from '@/shared/types'
import { Badge } from '@/shared/ui'
import { statusTone } from '@/shared/utils/status'
import { count } from '@/shared/utils'
import { useResourceObservation } from './ResourceLive'
import { NodeMetrics } from './NodeMetrics'

export type ResourceFlowNode = Node<
  {
    resource: GraphNode
    project?: string
    expanded?: boolean
    toggle?: (id: string) => void
    togglable?: boolean
  },
  'resource'
>
export const icons: Record<string, LucideIcon> = {
  server: Server,
  agent: Server,
  builder: Workflow,
  runner: Server,
  project: Boxes,
  service: Layers,
  job: Workflow,
  container: Container,
  instance: Container,
  image: Package,
  registry: Database,
  source: GitBranch,
  network: Network,
  volume: HardDrive,
  mount: HardDrive,
}
const ResourceNode = memo(function ResourceNode({
  data,
  selected,
}: NodeProps<ResourceFlowNode>) {
  const resource = data.resource
  const observation = useResourceObservation(resource, data.project)
  const state = observation?.state || resource.state
  const Icon = icons[resource.kind] || Boxes
  return (
    <div
      className={
        'infra-node tone-' +
        statusTone(state) +
        (selected ? ' is-selected' : '')
      }
    >
      <Handle type="target" position={Position.Left} />
      <div className="infra-node-top">
        <span className={'resource-icon ' + resource.kind}>
          <Icon size={16} aria-hidden="true" />
        </span>
        <span>
          {resource.kind === 'agent'
            ? observation?.role || resource.kind
            : resource.kind}
        </span>
        {!!observation?.dependencies && (
          <span
            className="node-dependencies"
            title={`Select to inspect ${count(observation.dependencies, 'dependency', 'dependencies')}`}
          >
            Needs {observation.dependencies}
          </span>
        )}
        {state && <Badge value={state} />}
      </div>
      <strong title={resource.name}>
        {resource.name || 'Unnamed resource'}
      </strong>
      <NodeMetrics resource={resource} observation={observation} />
      {data.toggle &&
        resource.kind === 'agent' &&
        !!observation?.containers && (
          <button
            className="node-expand nodrag nopan"
            aria-expanded={!!data.expanded}
            disabled={!data.togglable}
            aria-label={
              (data.expanded ? 'Hide' : 'Show') +
              ' containers on ' +
              resource.name
            }
            onClick={(event) => {
              event.stopPropagation()
              data.toggle?.(resource.id)
            }}
          >
            {data.expanded ? 'Hide containers' : 'Show containers'}
          </button>
        )}
      <Handle type="source" position={Position.Right} />
    </div>
  )
})
export const nodeTypes = { resource: ResourceNode }
