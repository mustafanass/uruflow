import { useSyncExternalStore } from 'react'

import { age, duration, time } from '@/shared/utils'

const listeners = new Set<() => void>()
let now = Date.now()
let timer = 0
function tick() {
  now = Date.now()
  for (const changed of listeners) changed()
}
function visibility() {
  clearInterval(timer)
  timer =
    document.visibilityState === 'visible' ? window.setInterval(tick, 1000) : 0
  tick()
}
function subscribe(changed: () => void) {
  listeners.add(changed)
  if (listeners.size === 1) {
    now = Date.now()
    document.addEventListener('visibilitychange', visibility)
    visibility()
  }
  return () => {
    listeners.delete(changed)
    if (!listeners.size) {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', visibility)
    }
  }
}
function useNow() {
  return useSyncExternalStore(subscribe, () => now)
}
export function RelativeTime({ value }: { value?: string }) {
  useNow()
  return (
    <time dateTime={value} title={time(value)}>
      {age(value)}
    </time>
  )
}
export function ElapsedTime({
  start,
  elapsed,
}: {
  start: string
  elapsed?: number
}) {
  const now = useNow()
  return (
    <span>
      {duration(elapsed === undefined ? now - Date.parse(start) : elapsed)}
    </span>
  )
}
export function Uptime({
  seconds,
  observedAt,
}: {
  seconds: number
  observedAt: string
}) {
  const now = useNow()
  const since = Date.parse(observedAt)
  return (
    <span>
      {duration(
        seconds * 1000 +
          (Number.isFinite(since) ? Math.max(0, now - since) : 0),
      )}
    </span>
  )
}
