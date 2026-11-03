import type { Release, Snapshot } from '@/shared/types'

export function currentRelease(snapshot: Snapshot | null, release: Release) {
  if (!snapshot) return false
  const running = snapshot.containers.filter(
    (container) =>
      container.project === release.project &&
      ['running', 'restarting', 'exited', 'dead'].includes(container.state),
  )
  if (!running.length) return false
  const matches = (candidate: Release) => {
    const reported =
      candidate.targets?.flatMap((target) =>
        Object.entries(target.containers || {}).map(([service, id]) => ({
          agent: target.agent_id,
          service,
          id,
        })),
      ) || []
    if (!reported.length)
      return running.every(
        (container) => container.details?.release === candidate.id,
      )
    return (
      candidate.status === 'succeeded' &&
      reported.length === running.length &&
      running.every((container) =>
        reported.some(
          (entry) =>
            entry.agent === container.agent_id &&
            entry.id === container.id &&
            entry.service === (container.service || ''),
        ),
      )
    )
  }
  if (!matches(release)) return false
  const index = snapshot.releases.findIndex(
    (candidate) => candidate.id === release.id,
  )
  return !snapshot.releases.some(
    (candidate, position) =>
      candidate.project === release.project &&
      candidate.id !== release.id &&
      (Date.parse(candidate.started_at) > Date.parse(release.started_at) ||
        (candidate.started_at === release.started_at &&
          index >= 0 &&
          position < index)) &&
      matches(candidate),
  )
}
