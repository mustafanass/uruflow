import { useContext } from 'react'

import { WorkspaceContext } from '@/shared/workspace/context'

export function useWorkspace() {
  const value = useContext(WorkspaceContext)
  if (!value) {
    throw new Error('Workspace provider is missing')
  }
  return value
}
