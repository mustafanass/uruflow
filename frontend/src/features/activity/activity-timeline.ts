import type { Alert, Snapshot } from '../../shared/types/observation.ts'

interface TimelineEntry {
  id: string
  time: string
  type: 'Deployment' | 'Agent' | 'Alert' | 'System'
  source: string
  message: string
  state: string
  href?: string
}

export function activityTimeline(
  snapshot: Snapshot,
  alerts: Alert[],
): TimelineEntry[] {
  const releases = new Map(
    snapshot.releases.map((release) => [release.id, release]),
  )
  const alertIDs = new Set(alerts.map((alert) => alert.id))
  const agents = new Set(snapshot.agents.map((agent) => agent.name))
  const recorded = new Set(snapshot.activity.map((entry) => entry.operation))
  const rows: TimelineEntry[] = snapshot.activity
    .filter((entry) => !alertIDs.has(entry.operation))
    .map((entry) => {
      const release = releases.get(entry.operation)
      const agent =
        agents.has(entry.source) ||
        /\bagent (?:connected|disconnected|enrolled)\b/i.test(entry.message)
      return {
        id: 'event/' + entry.sequence,
        time: entry.time,
        type: release ? 'Deployment' : agent ? 'Agent' : 'System',
        source: release?.project || entry.source || 'Control plane',
        message: entry.message,
        state: entry.level,
        href: release
          ? '/releases/' + encodeURIComponent(release.id)
          : undefined,
      }
    })
  for (const release of snapshot.releases) {
    if (recorded.has(release.id)) continue
    rows.push({
      id: 'release/' + release.id,
      time: release.ended_at || release.started_at,
      type: 'Deployment',
      source: release.project,
      message:
        release.trigger === 'rollback' ? 'Release restoration' : 'Deployment',
      state: release.status,
      href: '/releases/' + encodeURIComponent(release.id),
    })
  }
  for (const alert of alerts) {
    const href = alert.project
      ? '/projects/' + encodeURIComponent(alert.project)
      : alert.agent_id
        ? '/agents/' + encodeURIComponent(alert.agent_id)
        : undefined
    const event = {
      type: 'Alert' as const,
      source: alert.project || alert.agent_name || 'Control plane',
      message: alert.message,
      href,
    }
    rows.push({
      ...event,
      id: 'alert/' + alert.id,
      time: alert.created_at,
      state: alert.severity,
    })
    if (alert.resolved && alert.resolved_at)
      rows.push({
        ...event,
        id: 'resolved/' + alert.id,
        time: alert.resolved_at,
        message: 'Resolved: ' + alert.message,
        state: 'resolved',
      })
  }
  return rows
    .map((entry) => ({ entry, at: Date.parse(entry.time) }))
    .sort((a, b) => b.at - a.at || a.entry.id.localeCompare(b.entry.id))
    .map((value) => value.entry)
}
