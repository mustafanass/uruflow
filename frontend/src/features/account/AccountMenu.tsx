import { useEffect, useId, useRef, useState } from 'react'
import { ChevronDown, KeyRound, LogOut, Moon, Sun, Monitor } from 'lucide-react'
import { useWorkspace } from '@/shared/hooks'
import { setTheme, useTheme } from '@/shared/hooks/useTheme'
import { ChangePassword } from './ChangePassword'

export function AccountMenu({ signOut }: { signOut: () => void }) {
  const { session } = useWorkspace()
  const { theme } = useTheme()
  const [open, setOpen] = useState(false)
  const [password, setPassword] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const id = useId()
  const openAtEnd = useRef(false)
  const account = session.user!
  const nextTheme =
    theme === 'system' ? 'light' : theme === 'light' ? 'dark' : 'system'
  const Icon = theme === 'dark' ? Moon : theme === 'light' ? Sun : Monitor
  useEffect(() => {
    if (!open) return
    const items =
      menu.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')
    items?.[openAtEnd.current ? items.length - 1 : 0]?.focus()
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [open])
  function close() {
    setOpen(false)
    trigger.current?.focus()
  }
  return (
    <div className="account-menu" ref={root}>
      <button
        className="account-trigger"
        aria-label={'Account menu for ' + account.username}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={id}
        ref={trigger}
        onClick={() => {
          openAtEnd.current = false
          setOpen((value) => !value)
        }}
        onKeyDown={(event) => {
          if (['ArrowDown', 'ArrowUp'].includes(event.key)) {
            event.preventDefault()
            openAtEnd.current = event.key === 'ArrowUp'
            setOpen(true)
          }
        }}
      >
        <span className="account-avatar">
          {account.username[0].toUpperCase()}
        </span>
        <span className="account-name">
          <strong>{account.username}</strong>
          <span>{account.role}</span>
        </span>
        <ChevronDown size={15} aria-hidden="true" />
      </button>
      {open && (
        <div
          className="account-dropdown"
          role="menu"
          id={id}
          ref={menu}
          aria-label="Your account"
          onKeyDown={(event) => {
            if (event.key === 'Escape') {
              event.stopPropagation()
              close()
              return
            }
            if (event.key === 'Tab') {
              setOpen(false)
              return
            }
            const items = [
              ...menu.current!.querySelectorAll<HTMLButtonElement>(
                '[role="menuitem"]',
              ),
            ]
            const index = items.indexOf(
              document.activeElement as HTMLButtonElement,
            )
            const next =
              event.key === 'ArrowDown'
                ? (index + 1) % items.length
                : event.key === 'ArrowUp'
                  ? (index - 1 + items.length) % items.length
                  : event.key === 'Home'
                    ? 0
                    : event.key === 'End'
                      ? items.length - 1
                      : -1
            if (next >= 0) {
              event.preventDefault()
              items[next].focus()
            }
          }}
        >
          <div className="account-summary">
            <strong>{account.username}</strong>
            <small>{account.role}</small>
          </div>
          <button
            role="menuitem"
            tabIndex={-1}
            onClick={() => {
              close()
              setPassword(true)
            }}
          >
            <KeyRound size={16} aria-hidden="true" />
            Change password
          </button>
          <button
            role="menuitem"
            tabIndex={-1}
            onClick={() => setTheme(nextTheme)}
          >
            <Icon size={16} aria-hidden="true" />
            Theme: {theme}
            <span>{nextTheme}</span>
          </button>
          <button
            role="menuitem"
            tabIndex={-1}
            onClick={() => {
              close()
              signOut()
            }}
          >
            <LogOut size={16} aria-hidden="true" />
            Sign out
          </button>
        </div>
      )}
      {password && <ChangePassword onClose={() => setPassword(false)} />}
    </div>
  )
}
