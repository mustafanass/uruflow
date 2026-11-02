export function StateMark({ state, label }: { state: string; label?: string }) {
  const tone = [
    'healthy',
    'running',
    'online',
    'completed',
    'succeeded',
    'built',
  ].includes(state)
    ? 'good'
    : ['failed', 'unhealthy', 'degraded', 'dead', 'offline', 'exited'].includes(
          state,
        )
      ? 'bad'
      : [
            'starting',
            'restarting',
            'building',
            'releasing',
            'deploying',
            'pending',
            'missing',
          ].includes(state)
        ? 'warn'
        : 'neutral'
  return (
    <span
      className={'state-mark ' + tone}
      aria-label={label || state}
      title={label || state}
      role="img"
    >
      <i aria-hidden="true" />
    </span>
  )
}
