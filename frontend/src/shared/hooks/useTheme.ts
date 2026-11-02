import { useSyncExternalStore } from 'react'

type Theme = 'system' | 'light' | 'dark'
const media = window.matchMedia('(prefers-color-scheme: dark)')
function subscribe(changed: () => void) {
  window.addEventListener('uruflow:theme', changed)
  media.addEventListener('change', changed)
  return () => {
    window.removeEventListener('uruflow:theme', changed)
    media.removeEventListener('change', changed)
  }
}
function selectedTheme(): Theme {
  const value = document.documentElement.dataset.theme
  return value === 'dark' || value === 'light' ? value : 'system'
}
export function setTheme(value: Theme) {
  document.documentElement.dataset.theme = value
  try {
    localStorage.setItem('uruflow:theme', value)
  } catch {}
  window.dispatchEvent(new Event('uruflow:theme'))
}
export function useTheme() {
  const theme = useSyncExternalStore(subscribe, selectedTheme)
  const resolved = useSyncExternalStore(subscribe, () =>
    selectedTheme() === 'system'
      ? media.matches
        ? 'dark'
        : 'light'
      : selectedTheme(),
  )
  return { theme, resolved }
}
