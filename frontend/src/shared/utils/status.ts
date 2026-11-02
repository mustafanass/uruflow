export function statusTone(state: string) {
  if (
    [
      'healthy',
      'online',
      'running',
      'succeeded',
      'completed',
      'built',
      'success',
      'resolved',
    ].includes(state)
  )
    return 'good'
  if (
    [
      'unhealthy',
      'failed',
      'degraded',
      'critical',
      'dead',
      'error',
      'offline',
    ].includes(state)
  )
    return 'bad'
  if (['missing', 'warning', 'restarting'].includes(state)) return 'warn'
  if (
    ['building', 'releasing', 'pending', 'starting', 'requested'].includes(
      state,
    )
  )
    return 'active'
  return 'neutral'
}
