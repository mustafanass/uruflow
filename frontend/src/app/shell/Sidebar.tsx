import { useEffect, useRef } from 'react'
import { Link, useLocation } from 'react-router-dom'

import { X } from 'lucide-react'

import { navigation, navigationSection } from '@/app/navigation'
import { AccountMenu } from '@/features/account/AccountMenu'
import { useWorkspace } from '@/shared/hooks'

import { Brand } from './Brand'

export function Sidebar({
  open,
  close,
  signOut,
}: {
  open: boolean
  close: () => void
  signOut: () => void
}) {
  const { snapshot, connection } = useWorkspace()
  const section = navigationSection(useLocation().pathname)
  const element = useRef<HTMLElement>(null)
  useEffect(() => {
    if (!open) {
      return
    }
    const previous = document.activeElement as HTMLElement | null
    const address = window.location.href
    element.current?.querySelector<HTMLElement>('button,a[href]')?.focus()
    return () => {
      if (window.location.href === address) {
        previous?.focus()
      }
    }
  }, [open])
  const online =
    snapshot?.agents.filter((agent) => agent.status === 'online').length || 0
  return (
    <>
      {open && (
        <button
          className="navigation-backdrop"
          aria-label="Close navigation"
          onClick={close}
        />
      )}
      <aside
        id="workspace-navigation"
        ref={element}
        role={open ? 'dialog' : undefined}
        aria-modal={open ? true : undefined}
        aria-label="Workspace navigation"
        onKeyDown={(event) => {
          if (event.key !== 'Tab' || !open) {
            return
          }
          const controls = Array.from(
            element.current?.querySelectorAll<HTMLElement>(
              'a[href],button:not(:disabled)',
            ) || [],
          )
          const first = controls[0],
            last = controls[controls.length - 1]
          if (event.shiftKey && document.activeElement === first) {
            event.preventDefault()
            last?.focus()
          } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault()
            first?.focus()
          }
        }}
        className={'sidebar' + (open ? ' open' : '')}
      >
        <div className="sidebar-heading">
          <Brand />
          <button
            className="icon-button mobile-only"
            onClick={close}
            aria-label="Close navigation"
          >
            <X size={19} />
          </button>
        </div>
        <div className="workspace-name">
          <span className="workspace-avatar">U</span>
          <div>
            <strong>{snapshot?.server_name || 'URUFLOW'}</strong>
            <span>Deployment workspace</span>
          </div>
        </div>
        <nav aria-label="Main navigation">
          {navigation.map((item) => (
            <Link
              key={item.to}
              to={item.to}
              title={item.label}
              aria-current={section === item.to ? 'page' : undefined}
              className={section === item.to ? 'active' : ''}
            >
              <item.icon size={18} />
              <span>{item.label}</span>
              {item.to === '/' && !!snapshot?.alerts.length && (
                <em>{snapshot.alerts.length}</em>
              )}
            </Link>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="agent-link-state">
            <span
              className={
                'status-dot' +
                (connection !== 'live' || !online ? ' inactive' : '')
              }
            />
            <span>
              {snapshot
                ? `${online} of ${snapshot.agents.length} agents online`
                : 'Connecting…'}
            </span>
          </div>
          <AccountMenu signOut={signOut} />
        </div>
      </aside>
    </>
  )
}
