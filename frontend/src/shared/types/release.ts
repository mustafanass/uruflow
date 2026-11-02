import { Project } from '@/shared/types'

interface ReleaseTarget {
  agent_id: string
  agent_name: string
  status: string
  message?: string
  ended_at?: string
}
export interface Release {
  id: string
  project: string
  status: string
  trigger: string
  builder: string
  builder_name: string
  commit: string
  commits?: Record<string, string>
  image: string
  images?: Record<string, string>
  digest: string
  message?: string
  started_at: string
  ended_at?: string
  duration: number
  targets?: ReleaseTarget[]
  spec?: Project
  workflow?: string
  runner_count?: number
}
export interface Image {
  repository: string
  tag: string
  digest: string
  size: number
  created_at: string
}
