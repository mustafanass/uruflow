import { useEffect, useRef } from 'react'
import { Link, useLocation } from 'react-router-dom'

import { ChevronRight, X } from 'lucide-react'

import { navigation, navigationSection } from '@/app/navigation'
import { useWorkspace } from '@/shared/hooks'

import { Brand } from './Brand'

export function Sidebar({ open, close }: { open: boolean; close: () => void }) {
  const { snapshot, stale, connection } = useWorkspace()
  const state = stale
    ? 'Data stale'
    : connection === 'live'
      ? 'Live'
      : connection === 'reconnecting'
        ? 'Reconnecting'
        : 'Connecting'
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
          <Link
            className={
              'live-indicator' +
              (stale || connection !== 'live' ? ' stale' : '')
            }
            to="/system"
            title={state}
            aria-label={state + ' updates'}
            onClick={close}
          >
            <span className="status-dot" aria-hidden="true" />
            <span role="status">{state}</span>
            <ChevronRight size={14} aria-hidden="true" />
          </Link>
        </div>
      </aside>
    </>
  )
}
