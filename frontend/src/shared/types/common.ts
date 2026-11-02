import { Agent } from '@/shared/types'

export interface Document {
  content: string
  revision: string
}
export interface Enrollment {
  agent: Agent
  command: string
  trust_root: string
}
export interface Audit {
  id: number
  actor: string
  action: string
  resource: string
  outcome: string
  created_at: string
}
