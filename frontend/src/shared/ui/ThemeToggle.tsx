import { Monitor, Moon, Sun } from 'lucide-react'

import { setTheme, useTheme } from '@/shared/hooks/useTheme'

export function ThemeToggle() {
  const { theme } = useTheme()
  const Icon = theme === 'dark' ? Moon : theme === 'light' ? Sun : Monitor
  const next =
    theme === 'system' ? 'light' : theme === 'light' ? 'dark' : 'system'
  const title = 'Theme: ' + theme + '. Switch to ' + next
  return (
    <button
      type="button"
      className="icon-button"
      title={title}
      aria-label={title}
      onClick={() => setTheme(next)}
    >
      <Icon size={18} aria-hidden="true" />
    </button>
  )
}
