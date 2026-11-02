import type {
  Agent,
  Container,
  Metrics,
  Project,
  Service,
  Snapshot,
} from '@/shared/types'

export function bytes(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) {
    return 'Unavailable'
  }
  if (value < 1024) {
    return `${value} B`
  }
  const unit = Math.min(4, Math.floor(Math.log(value) / Math.log(1024)))
  return `${(value / 1024 ** unit).toFixed(1)} ${['B', 'KiB', 'MiB', 'GiB', 'TiB'][unit]}`
}
export function time(value?: string): string {
  if (
    !value ||
    value.startsWith('0001') ||
    !Number.isFinite(Date.parse(value))
  ) {
    return 'Unavailable'
  }
  return new Date(value).toLocaleString()
}
export function age(value?: string): string {
  if (
    !value ||
    value.startsWith('0001') ||
    !Number.isFinite(Date.parse(value)) ||
    Date.parse(value) <= 0
  ) {
    return 'No sample'
  }
  const seconds = Math.round((Date.now() - Date.parse(value)) / 1000)
  if (seconds < -5) {
    return 'Clock skew'
  }
  if (seconds < 2) {
    return 'Just now'
  }
  if (seconds < 60) {
    return `${seconds}s ago`
  }
  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)}m ago`
  }
  if (seconds < 86400) {
    return `${Math.floor(seconds / 3600)}h ago`
  }
  return `${Math.floor(seconds / 86400)}d ago`
}
export function duration(ms: number): string {
  if (!ms || !Number.isFinite(ms) || ms < 0) {
    return '—'
  }
  const sec = Math.round(ms / 1000)
  if (sec < 60) return `${sec}s`
  const minutes = Math.floor(sec / 60)
  if (minutes < 60) return `${minutes}m ${sec % 60}s`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ${minutes % 60}m`
  return `${Math.floor(hours / 24)}d ${hours % 24}h`
}
export function count(
  value: number,
  singular: string,
  plural = singular + 's',
) {
  return `${value} ${value === 1 ? singular : plural}`
}
export function available(metrics: Metrics | undefined, kind: string): boolean {
  return (
    !!metrics?.available?.includes(kind) && Date.parse(metrics.collected_at) > 0
  )
}
export function fresh(agent: Agent | undefined, at?: string): boolean {
  const timestamp = Date.parse(at || '')
  return (
    agent?.status === 'online' &&
    timestamp > 0 &&
    Date.now() - timestamp >= -5000 &&
    Date.now() - timestamp <
      Math.max(30, (agent.metrics?.metrics_sec || 10) * 3) * 1000
  )
}
export function containerState(c: Container, snapshot: Snapshot): string {
  const agent = snapshot.agents.find((a) => a.id === c.agent_id)
  if (!fresh(agent, c.observed_at)) {
    return 'unknown'
  }
  const p = snapshot.projects.find((p) => p.name === c.project)
  const job = p && services(p).find((s) => s.name === c.service)?.mode === 'job'
  if (c.state === 'exited' && job && c.details?.exit_code === 0) {
    return 'completed'
  }
  return c.state === 'running' &&
    ['healthy', 'unhealthy', 'starting'].includes(c.health)
    ? c.health
    : c.state
}
export function services(project: Project): Service[] {
  return project.services?.length
    ? project.services
    : [
        {
          name: '',
          git_url: project.git_url,
          branch: project.branch,
          dockerfile: project.dockerfile,
          context: project.context,
          ...project.runtime,
        },
      ]
}
export function ports(values?: Service['ports']): string {
  return (
    values
      ?.map(
        (p) =>
          `${p.host_ip ? p.host_ip + ':' : ''}${p.host}:${p.container}${p.protocol && p.protocol !== 'tcp' ? '/' + p.protocol : ''}`,
      )
      .join(', ') || 'None published'
  )
}
export function short(value: string | undefined, max = 12): string {
  return value ? (value.length > max ? value.slice(0, max) + '…' : value) : '—'
}
export const errorText = (error: unknown) =>
  error instanceof Error ? error.message : 'The request failed.'
