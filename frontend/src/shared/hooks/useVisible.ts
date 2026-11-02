import { useSyncExternalStore } from 'react'

function subscribe(changed: () => void) {
  document.addEventListener('visibilitychange', changed)
  return () => document.removeEventListener('visibilitychange', changed)
}
export function useVisible() {
  return useSyncExternalStore(
    subscribe,
    () => document.visibilityState === 'visible',
  )
}
