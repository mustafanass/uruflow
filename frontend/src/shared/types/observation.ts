import { Agent, Container, Metrics, Project, Release } from '@/shared/types'

export interface Alert {
  project?: string
  resolved?: boolean
  resolved_at?: string
  id: string
  agent_id: string
  agent_name: string
  type: string
  severity: string
  message: string
  created_at: string
}
export interface Activity {
  sequence: number
  time: string
  kind: string
  level: string
  operation: string
  source: string
  message: string
}
export interface GraphNode {
  id: string
  parent: string
  kind: string
  name: string
  state: string
  summary: string
  relation: string
  agent_id: string
  container_id: string
  service: string
}
interface GraphEdge {
  id: string
  source: string
  target: string
  kind: string
  label?: string
}
export interface Graph {
  nodes: GraphNode[]
  edges: GraphEdge[]
}
export interface Snapshot {
  at: string
  server_name: string
  version: string
  host: Metrics
  agents: Agent[]
  projects: Project[]
  containers: Container[]
  releases: Release[]
  alerts: Alert[]
  activity: Activity[]
  problems: string[]
  registry: string
  registry_address: string
  graphs: Record<
    string,
    {
      runtime: Graph
      delivery: Graph
    }
  >
  fleet_graph: Graph
  project_states: Record<string, string>
}
export interface Sample {
  at: number
  cpu: number | null
  memory: number | null
  rx?: number
  tx?: number
}
