import type { Agent, Metrics, Snapshot } from '@/shared/types'
import { projectHealth } from '@/features/projects/project-health'
import { services } from '@/shared/utils'
import type { ResourceObservation } from './resource-store'

export function resourceObservations(snapshot: Snapshot, sourceLive: boolean) {
  const result = new Map<string, ResourceObservation>()
  const at = Date.parse(snapshot.at)
  const agents = new Map(snapshot.agents.map((agent) => [agent.id, agent]))
  const projects = new Map(
    snapshot.projects.map((project) => [project.name, project]),
  )
  const definitions = new Map(
    snapshot.projects.map((project) => [project.name, services(project)]),
  )
  const states = new Map(
    Object.values(snapshot.graphs)
      .flatMap((graph) => graph.runtime.nodes)
      .filter((node) => node.kind === 'container')
      .map((node) => [node.id, node.state]),
  )
  const recent = (collected: string | undefined, cadence = 10) => {
    const age = at - Date.parse(collected || '')
    return (
      sourceLive &&
      Number.isFinite(age) &&
      age >= -5000 &&
      age < Math.max(30, cadence * 3) * 1000
    )
  }
  const fresh = (agent: Agent | undefined, collected: string) =>
    agent?.status === 'online' && recent(collected, agent.metrics?.metrics_sec)
  const metrics = (
    value: Metrics | undefined,
    live: boolean,
  ): Pick<
    ResourceObservation,
    'cpu' | 'cpuCapacity' | 'memory' | 'memoryCapacity' | 'memoryPercent'
  > => ({
    cpu: live && value?.available?.includes('cpu') ? value.cpu_percent : null,
    cpuCapacity: 100,
    memory:
      live && value?.available?.includes('memory') ? value.memory_used : null,
    memoryCapacity: value?.memory_total || undefined,
    memoryPercent:
      live && value?.available?.includes('memory')
        ? value.memory_percent
        : null,
  })
  const active = snapshot.releases.filter(
    (release) => !['succeeded', 'failed', 'skipped'].includes(release.status),
  )
  for (const agent of snapshot.agents) {
    const live = fresh(agent, agent.metrics?.collected_at || '')
    result.set('agent/' + agent.id, {
      state: sourceLive ? agent.status : 'unknown',
      role: agent.roles.join(' / '),
      ...metrics(agent.metrics, live),
      live,
      containers: snapshot.containers.filter(
        (container) => container.agent_id === agent.id,
      ).length,
      active: active.filter(
        (release) =>
          release.builder === agent.id ||
          release.targets?.some((target) => target.agent_id === agent.id),
      ).length,
    })
  }
  result.set('server', {
    state: sourceLive ? 'online' : 'unknown',
    live: sourceLive,
    ...metrics(snapshot.host, recent(snapshot.host?.collected_at)),
    active: active.length,
    online: snapshot.agents.filter((agent) => agent.status === 'online').length,
  })
  for (const container of snapshot.containers) {
    const agent = agents.get(container.agent_id)
    const live = fresh(agent, container.observed_at)
    const service = definitions
      .get(container.project)
      ?.find((value) => value.name === (container.service || ''))
    const fallback =
      container.state === 'exited' &&
      service?.mode === 'job' &&
      container.details?.exit_code === 0
        ? 'completed'
        : container.state === 'running' &&
            ['healthy', 'unhealthy', 'starting'].includes(container.health)
          ? container.health
          : container.state
    const memoryCapacity =
      container.memory_limit || agent?.metrics?.memory_total || undefined
    result.set(`container/${container.agent_id}/${container.id}`, {
      state: live
        ? states.get(`container/${container.agent_id}/${container.id}`) ||
          fallback
        : 'unknown',
      cpu: live && container.stats_available ? container.cpu_percent : null,
      cpuCapacity:
        Math.max(
          service?.resources?.cpus || agent?.metrics?.cpu_cores || 1,
          0.01,
        ) * 100,
      memory: live && container.stats_available ? container.memory_usage : null,
      memoryCapacity,
      memoryPercent:
        live && container.stats_available && memoryCapacity
          ? (container.memory_usage / memoryCapacity) * 100
          : null,
      started: container.started_at,
      restarts: container.restart_count,
      live: live && container.state === 'running',
    })
  }
  for (const [name, project] of projects) {
    const health = projectHealth(project, snapshot, definitions.get(name) || [])
    const latest = snapshot.releases.find((release) => release.project === name)
    result.set('project/' + name, {
      services: definitions.get(name)?.length || 0,
      runners: project.runners?.length || 0,
      state: sourceLive ? snapshot.project_states[name] : 'unknown',
      ready: health.ready,
      expected: health.expected,
      latest: latest?.started_at,
      latestState: latest?.status,
      active: active.filter((release) => release.project === name).length,
    })
    for (const row of health.rows)
      result.set(`service/${name}/${row.service.name}`, {
        dependencies: row.service.depends_on?.length || 0,
        state: sourceLive ? row.state : 'unknown',
        live: sourceLive,
        ready: row.ready,
        expected: row.expected,
      })
  }
  return result
}
