import { useCallback, useEffect, useState } from 'react'

import { WorkspaceProvider } from '@/app/providers'
import { ConsoleShell } from '@/app/shell/ConsoleShell'
import SignIn from '@/features/auth/LoginPage'
import { api, setCSRF } from '@/shared/api'
import { type Session } from '@/shared/types'
import { ErrorState, Spinner } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function App() {
  const [session, setSession] = useState<Session | null>(null)
  const [error, setError] = useState('')
  const expire = useCallback(() => {
    setCSRF('')
    setSession({ authenticated: false })
  }, [])
  const load = useCallback(() => {
    setError('')
    api<Session>('/api/session')
      .then((value) => {
        setCSRF(value.csrf_token || '')
        setSession(value)
      })
      .catch((e) => setError(errorText(e)))
  }, [])
  useEffect(() => {
    load()
    window.addEventListener('uruflow:unauthorized', expire)
    return () => window.removeEventListener('uruflow:unauthorized', expire)
  }, [expire, load])
  if (error) {
    return (
      <div className="auth-surface">
        <ErrorState message={error} retry={load} />
      </div>
    )
  }
  if (!session) {
    return (
      <div className="auth-surface">
        <Spinner label="Connecting to URUFLOW…" />
      </div>
    )
  }
  if (!session.authenticated) {
    return (
      <SignIn
        session={session}
        ready={(value) => {
          setCSRF(value.csrf_token || '')
          setSession(value)
        }}
      />
    )
  }
  return (
    <WorkspaceProvider session={session} expire={expire}>
      <ConsoleShell expire={expire} />
    </WorkspaceProvider>
  )
}
