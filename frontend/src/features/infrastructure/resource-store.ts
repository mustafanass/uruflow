export interface ResourceObservation {
  services?: number
  runners?: number
  state: string
  role?: string
  dependencies?: number
  cpu?: number | null
  cpuCapacity?: number
  memory?: number | null
  memoryPercent?: number | null
  memoryCapacity?: number
  started?: string
  restarts?: number
  live?: boolean
  ready?: number
  expected?: number
  containers?: number
  active?: number
  online?: number
  latest?: string
  latestState?: string
}

export class ResourceStore {
  private values = new Map<string, ResourceObservation>()
  private listeners = new Map<string, Set<() => void>>()
  get(key: string) {
    return this.values.get(key) || null
  }
  subscribe(key: string, changed: () => void) {
    const entries = this.listeners.get(key) || new Set()
    entries.add(changed)
    this.listeners.set(key, entries)
    return () => {
      entries.delete(changed)
      if (!entries.size) this.listeners.delete(key)
    }
  }
  replace(next: Map<string, ResourceObservation>) {
    const changed = new Set<string>()
    for (const [key, value] of next) {
      const before = this.values.get(key)
      if (
        before &&
        Object.keys({ ...before, ...value }).every((field) =>
          Object.is(
            before[field as keyof ResourceObservation],
            value[field as keyof ResourceObservation],
          ),
        )
      )
        next.set(key, before)
      else changed.add(key)
    }
    for (const key of this.values.keys()) if (!next.has(key)) changed.add(key)
    this.values = next
    for (const key of changed)
      for (const notify of this.listeners.get(key) || []) notify()
  }
}
