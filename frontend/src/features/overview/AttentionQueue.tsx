import { Link } from 'react-router-dom'
import { AlertTriangle, ArrowUpRight, CheckCheck } from 'lucide-react'
import { useWorkspace } from '@/shared/hooks'
import { count } from '@/shared/utils'

export function AttentionQueue() {
  const { snapshot, fleetAccess, stale, connection } = useWorkspace()
  if (!snapshot) return null
  const offline = snapshot.agents.filter(
    (agent) => agent.status !== 'online',
  ).length
  const affected = snapshot.projects.filter((project) =>
    ['degraded', 'failed', 'unknown'].includes(
      snapshot.project_states[project.name],
    ),
  ).length
  const issues = [
    ...(snapshot.alerts.length
      ? [
          {
            text: count(snapshot.alerts.length, 'active alert'),
            to: '/?alerts=open',
          },
        ]
      : []),
    ...(offline
      ? [{ text: count(offline, 'disconnected agent'), to: '/agents' }]
      : []),
    ...(affected
      ? [
          {
            text:
              count(affected, 'project') +
              (affected === 1 ? ' needs attention' : ' need attention'),
            to: '/projects',
          },
        ]
      : []),
    ...(snapshot.problems.length
      ? [
          {
            text: count(snapshot.problems.length, 'configuration issue'),
            to: '/projects',
          },
        ]
      : []),
    ...(fleetAccess && snapshot.registry === 'unavailable'
      ? [{ text: 'Registry unavailable', to: '/system?tab=Server' }]
      : []),
  ]
  if (!issues.length)
    return snapshot.agents.length && !stale && connection === 'live' ? (
      <div className="attention-clear">
        <CheckCheck size={16} aria-hidden="true" />
        <span>No active issues</span>
      </div>
    ) : null
  return (
    <section className="attention-strip" aria-label="Needs attention">
      <AlertTriangle size={17} aria-hidden="true" />
      <div>
        {issues.map((issue) => (
          <Link key={issue.to + issue.text} to={issue.to}>
            {issue.text}
            <ArrowUpRight size={13} aria-hidden="true" />
          </Link>
        ))}
      </div>
    </section>
  )
}
