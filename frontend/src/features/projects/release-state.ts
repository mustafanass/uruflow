import type { Release, Snapshot } from '@/shared/types'

export function currentRelease(snapshot: Snapshot | null, release: Release) {
  if (!snapshot) return false
  const running = snapshot.containers.filter(
    (container) =>
      container.project === release.project &&
      ['running', 'restarting', 'exited', 'dead'].includes(container.state),
  )
  return (
    running.length > 0 &&
    running.every((container) => container.details?.release === release.id)
  )
}
