import {
  createContext,
  useCallback,
  useContext,
  useLayoutEffect,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react'
import type { GraphNode, Snapshot } from '@/shared/types'
import { resourceObservations } from './resource-observations'
import { ResourceStore } from './resource-store'

const Context = createContext<ResourceStore | null>(null)

export function ResourceLive({
  snapshot,
  live,
  children,
}: {
  snapshot: Snapshot
  live: boolean
  children: ReactNode
}) {
  const [store] = useState(() => {
    const value = new ResourceStore()
    value.replace(resourceObservations(snapshot, live))
    return value
  })
  useLayoutEffect(
    () => store.replace(resourceObservations(snapshot, live)),
    [snapshot, live, store],
  )
  return <Context.Provider value={store}>{children}</Context.Provider>
}

export function useResourceObservation(resource: GraphNode, project?: string) {
  const store = useContext(Context)!
  const key =
    resource.kind === 'server'
      ? 'server'
      : ['agent', 'runner', 'builder'].includes(resource.kind)
        ? 'agent/' + resource.agent_id
        : resource.kind === 'container'
          ? `container/${resource.agent_id}/${resource.container_id}`
          : resource.kind === 'project'
            ? 'project/' + resource.name
            : ['service', 'job'].includes(resource.kind)
              ? `service/${project}/${resource.service}`
              : ''
  const subscribe = useCallback(
    (changed: () => void) => store.subscribe(key, changed),
    [store, key],
  )
  const read = useCallback(() => store.get(key), [store, key])
  return useSyncExternalStore(subscribe, read)
}
