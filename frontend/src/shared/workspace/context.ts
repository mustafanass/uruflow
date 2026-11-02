import { createContext } from 'react'

import { type Sample, type Session, type Snapshot } from '@/shared/types'

export type Notice = {
  id: number
  message: string
  kind: 'success' | 'error'
}
export interface Confirmation {
  title: string
  description: string
  label: string
  danger?: boolean
  success?: string
  run: () => Promise<unknown>
}
export interface WorkspaceState {
  snapshot: Snapshot | null
  connection: 'connecting' | 'live' | 'reconnecting'
  stale: boolean
  session: Session
  canOperate: boolean
  canAdmin: boolean
  fleetAccess: boolean
  notice: Notice | null
  notify: (message: string, kind?: 'success' | 'error') => void
  confirm: (value: Confirmation | null) => void
  confirmation: Confirmation | null
  samples: (key: string) => Sample[]
  dirty: boolean
  setDirty: (value: boolean, key?: string) => void
}
export const WorkspaceContext = createContext<WorkspaceState | null>(null)
