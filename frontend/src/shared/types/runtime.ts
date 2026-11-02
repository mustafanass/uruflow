import { Mount } from '@/shared/types'

export interface Metrics {
  collected_at: string
  available?: string[]
  errors?: string[]
  cpu_cores?: number
  metrics_sec?: number
  cpu_percent: number
  memory_percent: number
  memory_used: number
  memory_total: number
  disk_percent: number
  disk_used: number
  disk_total: number
  load_avg?: number[]
  uptime: number
  host_uptime?: number
}
export interface Agent {
  id: string
  name: string
  roles: string[]
  host: string
  hostname: string
  version: string
  platform: string
  status: string
  metrics?: Metrics
  last_seen: string
  registered_at: string
}
export interface ContainerDetails {
  release?: string
  role?: string
  image_id?: string
  created?: string
  finished_at?: string
  exit_code: number
  oom_killed: boolean
  error?: string
  networks?: {
    name: string
    address?: string
    gateway?: string
    aliases?: string[]
  }[]
  ports?: string[]
  mounts?: Mount[]
  probes?: {
    end: string
    exit_code: number
    output: string
  }[]
}
export interface Container {
  id: string
  agent_id: string
  name: string
  project: string
  service?: string
  image: string
  state: string
  health: string
  observed_at: string
  stats_available: boolean
  cpu_percent: number
  memory_usage: number
  memory_limit: number
  network_rx: number
  network_tx: number
  pids: number
  block_read: number
  block_write: number
  restart_count: number
  started_at: string
  details?: ContainerDetails
}
