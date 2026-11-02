import { useEffect, useState } from 'react'
import { useBlocker, useLocation } from 'react-router-dom'

import { pageTitle } from '@/app/navigation'
import { AppRoutes } from '@/app/router'
import { ViewBoundary } from '@/app/ViewBoundary'
import { api } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { Modal } from '@/shared/ui'
import { errorText } from '@/shared/utils'

import { ConfirmationDialog } from './ConfirmationDialog'
import { Sidebar } from './Sidebar'
import { Topbar } from './Topbar'
import { WorkspaceHelp } from './WorkspaceHelp'

export function ConsoleShell({ expire }: { expire: () => void }) {
  const {
    snapshot,
    connection,
    stale,
    notice,
    dirty,
    setDirty,
    confirm,
    notify,
  } = useWorkspace()
  const location = useLocation()
  const blocker = useBlocker(dirty)
  const [navOpen, setNavOpen] = useState(false)
  const [help, setHelp] = useState(false)
  useEffect(() => {
    document.title = pageTitle(location.pathname) + ' · URUFLOW'
    setNavOpen(false)
  }, [location.pathname, location.search])
  useEffect(() => {
    if (!navOpen) {
      return
    }
    const close = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setNavOpen(false)
      }
    }
    document.addEventListener('keydown', close)
    return () => document.removeEventListener('keydown', close)
  }, [navOpen])
  useEffect(() => {
    const wide = window.matchMedia('(min-width: 1021px)')
    const close = () => {
      if (wide.matches) {
        setNavOpen(false)
      }
    }
    wide.addEventListener('change', close)
    return () => wide.removeEventListener('change', close)
  }, [])
  async function signOut() {
    await api('/api/session', { method: 'DELETE' })
    setDirty(false)
    expire()
  }
  function requestSignOut() {
    if (dirty) {
      confirm({
        title: 'Discard edits and sign out?',
        description: 'Your unsaved changes will be lost.',
        label: 'Sign out',
        danger: true,
        run: signOut,
      })
      return
    }
    signOut().catch((error) => notify(errorText(error), 'error'))
  }
  return (
    <div className="application">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <Sidebar
        open={navOpen}
        close={() => setNavOpen(false)}
        signOut={requestSignOut}
      />
      <div className="workspace" inert={navOpen}>
        <Topbar
          navigationOpen={navOpen}
          openNavigation={() => setNavOpen(true)}
          openHelp={() => setHelp(true)}
        />
        {(stale || connection === 'reconnecting') && (
          <div className="connection-banner" role="status">
            Reconnecting. Showing the last observation.
          </div>
        )}
        <main
          id="main-content"
          className="main-content"
          tabIndex={-1}
          aria-label={pageTitle(location.pathname)}
        >
          <ViewBoundary key={location.pathname}>
            <AppRoutes />
          </ViewBoundary>
        </main>
        <footer className="workspace-footer">
          <span>URUFLOW {snapshot?.version}</span>
          <span>
            {connection === 'live' && !stale
              ? 'Live observations'
              : 'Waiting for updates'}
          </span>
        </footer>
      </div>
      <ConfirmationDialog />
      {notice && (
        <div
          key={notice.id}
          className={'toast ' + notice.kind}
          role={notice.kind === 'error' ? 'alert' : 'status'}
        >
          {notice.message}
        </div>
      )}
      {blocker.state === 'blocked' && (
        <Modal title="Discard unsaved changes?" onClose={() => blocker.reset()}>
          <p className="modal-description">Your edits have not been saved.</p>
          <div className="modal-actions">
            <button className="button" onClick={() => blocker.reset()}>
              Keep editing
            </button>
            <button
              className="button danger"
              onClick={() => {
                setDirty(false)
                blocker.proceed()
              }}
            >
              Discard changes
            </button>
          </div>
        </Modal>
      )}
      {help && <WorkspaceHelp close={() => setHelp(false)} />}
    </div>
  )
}
