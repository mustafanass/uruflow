import { useEffect, useId, useRef, useState, type FormEvent } from 'react'

import { ArrowRight, Boxes, LockKeyhole, Network, Server } from 'lucide-react'

import { Brand } from '@/app/shell/Brand'
import { api } from '@/shared/api'
import { type Session } from '@/shared/types'
import { LoadingIndicator, PasswordInput } from '@/shared/ui'
import { ThemeToggle } from '@/shared/ui/ThemeToggle'
import { errorText } from '@/shared/utils'

export default function LoginPage({
  session,
  ready,
}: {
  session: Session
  ready: (session: Session) => void
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const errorID = useId()
  const errorRef = useRef<HTMLDivElement>(null)
  const [created, setCreated] = useState(false)
  const setup = !!session.setup_required && !created
  useEffect(() => {
    document.title = (setup ? 'Create administrator' : 'Sign in') + ' · URUFLOW'
  }, [setup])
  useEffect(() => {
    if (error) {
      errorRef.current?.focus()
    }
  }, [error])
  async function submit(event: FormEvent) {
    event.preventDefault()
    if (busy) {
      return
    }
    setBusy(true)
    setError('')
    try {
      if (setup) {
        await api('/api/setup', {
          method: 'POST',
          body: { token, username, password },
        })
        setCreated(true)
      }
      const value = await api<Session>('/api/session', {
        method: 'POST',
        body: { username, password },
      })
      ready(value)
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="auth-surface">
      <div className="auth-introduction">
        <Brand />
        <div>
          <span className="auth-topology" aria-hidden="true">
            <Network size={52} />
            <span />
            <Server size={40} />
            <span />
            <Boxes size={46} />
          </span>
          <h2>
            Deployments,
            <br />
            in view.
          </h2>
          <p>Projects, hosts and workloads in one place.</p>
        </div>
        <span className="auth-signature">
          <LockKeyhole size={15} aria-hidden="true" />
          Your infrastructure. Your workspace.
        </span>
      </div>
      <main className="auth-main">
        <div className="auth-theme">
          <ThemeToggle />
        </div>
        <form
          className="auth-form"
          onSubmit={submit}
          aria-busy={busy}
          aria-labelledby="auth-title"
        >
          <header className="auth-form-heading">
            <img
              className="auth-form-mark"
              src="/uruflow-logo.png"
              alt=""
              width="48"
              height="48"
            />
            <h1 id="auth-title">
              {setup ? 'Create administrator' : 'Sign in to URUFLOW'}
            </h1>
            <p>
              {setup
                ? 'Secure your deployment workspace.'
                : 'Welcome back to your workspace.'}
            </p>
          </header>
          {setup && (
            <label>
              Server setup token
              <input
                autoComplete="off"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                required
                name="token"
                spellCheck={false}
                disabled={busy}
              />
              <small>
                Use the token in the server data directory’s web-setup-token
                file.
              </small>
            </label>
          )}
          <label>
            Username
            <input
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              pattern="[a-z0-9](?:[a-z0-9]|-)*"
              name="username"
              spellCheck={false}
              autoCapitalize="none"
              disabled={busy}
              aria-describedby={error ? errorID : undefined}
            />
          </label>
          <label>
            Password
            <PasswordInput
              autoComplete={setup ? 'new-password' : 'current-password'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              minLength={setup ? 15 : undefined}
              maxLength={72}
              required
              name="password"
              spellCheck={false}
              aria-label="Password"
              disabled={busy}
              aria-describedby={error ? errorID : undefined}
            />
            {setup && (
              <small>
                At least 15 characters. Store this password securely.
              </small>
            )}
          </label>
          {error && (
            <div
              className="inline-error auth-error"
              role="alert"
              id={errorID}
              ref={errorRef}
              tabIndex={-1}
            >
              {error}
            </div>
          )}
          <button
            className="button primary auth-submit"
            disabled={busy}
            aria-busy={busy}
          >
            <span aria-live="polite">
              {busy
                ? setup
                  ? 'Creating administrator…'
                  : 'Signing in…'
                : setup
                  ? 'Create administrator'
                  : 'Sign in'}
            </span>
            {busy ? (
              <LoadingIndicator small />
            ) : (
              <ArrowRight size={18} aria-hidden="true" />
            )}
          </button>
          <div className="auth-footnote">
            <LockKeyhole size={13} aria-hidden="true" />
            Private deployment workspace
          </div>
        </form>
      </main>
    </div>
  )
}
