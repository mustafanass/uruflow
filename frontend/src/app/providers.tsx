import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from 'react'

import { api, setCSRF } from '@/shared/api'
import { useLiveSnapshot } from '@/shared/hooks'
import { type Session } from '@/shared/types'
import {
  Confirmation,
  Notice,
  WorkspaceContext,
  type WorkspaceState,
} from '@/shared/workspace/context'

export function WorkspaceProvider({
  session,
  expire,
  children,
}: {
  session: Session
  expire: () => void
  children: ReactNode
}) {
  const { snapshot, stale, connection, samples } = useLiveSnapshot(
    expire,
    session.user!.username,
  )
  const [notice, setNotice] = useState<Notice | null>(null)
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null)
  const [dirtyEditors, setDirtyEditors] = useState<Set<string>>(new Set())
  const dirty = dirtyEditors.size > 0
  const setDirty = useCallback((value: boolean, key?: string) => {
    setDirtyEditors((previous) => {
      if (!value && !key) return previous.size ? new Set() : previous
      const id = key || 'workspace'
      if (previous.has(id) === value) return previous
      const next = new Set(previous)
      if (value) next.add(id)
      else next.delete(id)
      return next
    })
  }, [])
  useEffect(() => {
    setCSRF(session.csrf_token || '')
  }, [session.csrf_token])
  useEffect(() => {
    if (!notice) {
      return
    }
    const timer = setTimeout(() => setNotice(null), 6000)
    return () => clearTimeout(timer)
  }, [notice])
  useEffect(() => {
    let lastRefresh = 0
    const active = () => {
      if (
        document.visibilityState !== 'visible' ||
        Date.now() - lastRefresh < 60000
      ) {
        return
      }
      lastRefresh = Date.now()
      api('/api/session/refresh', { method: 'POST', body: {} }).catch(() => {})
    }
    document.addEventListener('pointerdown', active, { passive: true })
    document.addEventListener('keydown', active)
    return () => {
      document.removeEventListener('pointerdown', active)
      document.removeEventListener('keydown', active)
    }
  }, [])
  useEffect(() => {
    const guard = (event: BeforeUnloadEvent) => {
      if (dirty) {
        event.preventDefault()
        event.returnValue = ''
      }
    }
    window.addEventListener('beforeunload', guard)
    return () => window.removeEventListener('beforeunload', guard)
  }, [dirty])
  const notify = useCallback<WorkspaceState['notify']>(
    (message, kind = 'success') => setNotice({ id: Date.now(), message, kind }),
    [],
  )
  const value = useMemo<WorkspaceState>(
    () => ({
      snapshot,
      connection,
      stale,
      session,
      canOperate:
        session.user?.role === 'operator' || session.user?.role === 'admin',
      fleetAccess:
        session.user?.role === 'admin' ||
        !!session.user?.projects?.includes('*'),
      canAdmin: session.user?.role === 'admin',
      notice,
      notify,
      confirm: setConfirmation,
      confirmation,
      samples,
      dirty,
      setDirty,
    }),
    [
      snapshot,
      connection,
      stale,
      session,
      notice,
      notify,
      confirmation,
      samples,
      dirty,
      setDirty,
    ],
  )
  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  )
}
