import type { Container, Project, Service, Snapshot } from '@/shared/types'

interface Placement {
  agent: string
  state: string
  containers: Container[]
}

const readyStates = new Set(['healthy', 'running', 'completed'])
const badStates = new Set([
  'unhealthy',
  'failed',
  'dead',
  'exited',
  'restarting',
])
const key = (service: string, agent: string) => JSON.stringify([service, agent])

export function projectHealth(
  project: Project,
  snapshot: Snapshot,
  definitions: Service[],
) {
  const nodes = snapshot.graphs[project.name]?.runtime.nodes || []
  const containers = snapshot.containers.filter(
    (container) => container.project === project.name,
  )
  const buildOnly = project.workflow === 'build_only'
  const serviceStates = new Map(
    nodes
      .filter((node) => ['service', 'job'].includes(node.kind))
      .map((node) => [node.service, node.state]),
  )
  const observedStates = new Map(
    nodes
      .filter((node) => node.kind === 'container')
      .map((node) => [key(node.container_id, node.agent_id), node.state]),
  )
  const expectedStates = new Map(
    nodes
      .filter((node) => node.kind === 'instance')
      .map((node) => [key(node.service, node.agent_id), node.state]),
  )
  const byPlacement = new Map<string, Container[]>()
  for (const container of containers) {
    const address = key(container.service || '', container.agent_id)
    const entries = byPlacement.get(address) || []
    entries.push(container)
    byPlacement.set(address, entries)
  }
  const rows = definitions.map((service) => {
    const placements: Placement[] = (
      buildOnly ? [] : project.runners || []
    ).map((agent) => {
      const instances = byPlacement.get(key(service.name, agent)) || []
      const states = instances.map(
        (container) =>
          observedStates.get(key(container.id, agent)) || 'unknown',
      )
      let state = expectedStates.get(key(service.name, agent)) || 'unknown'
      if (states.length)
        state =
          states.find((value) => badStates.has(value)) ||
          states.find((value) => !readyStates.has(value)) ||
          (states.every((value) => value === 'completed')
            ? 'completed'
            : 'running')
      return { agent, state, containers: instances }
    })
    return {
      service,
      state: serviceStates.get(service.name) || 'unknown',
      placements,
      ready: placements.filter((placement) => readyStates.has(placement.state))
        .length,
      expected: placements.length,
    }
  })
  return {
    rows,
    containers,
    buildOnly,
    ready: rows.reduce((sum, row) => sum + row.ready, 0),
    expected: rows.reduce((sum, row) => sum + row.expected, 0),
  }
}

export function projectPriority(state: string) {
  if (['failed', 'degraded', 'unhealthy', 'unknown'].includes(state)) return 0
  if (
    ['building', 'releasing', 'deploying', 'pending', 'starting'].includes(
      state,
    )
  )
    return 1
  if (state === 'undeployed') return 2
  return 3
}
