import { useState, type FormEvent } from 'react'

import { api } from '@/shared/api'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { PasswordInput } from '@/shared/ui/PasswordInput'
import { errorText } from '@/shared/utils'
import { useWorkspace } from '@/shared/hooks'

export function ChangePassword({ onClose }: { onClose: () => void }) {
  const { dirty } = useWorkspace()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      await api('/api/account/password', {
        method: 'POST',
        body: { current_password: current, new_password: next },
      })
      onClose()
      window.dispatchEvent(new Event('uruflow:unauthorized'))
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Modal title="Change your password" onClose={onClose} locked={busy}>
      <form onSubmit={submit}>
        <p className="modal-description">
          Changing the password signs this account out of all sessions. Sign in
          again with the new password.
        </p>
        {dirty && (
          <p className="inline-warning">
            Unsaved project edits will be discarded when you sign out.
          </p>
        )}
        <label>
          Current password
          <PasswordInput
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            required
            name="current"
            spellCheck={false}
            aria-label="Current password"
          />
        </label>
        <label>
          New password
          <PasswordInput
            autoComplete="new-password"
            minLength={15}
            maxLength={72}
            value={next}
            onChange={(e) => setNext(e.target.value)}
            required
            name="next"
            spellCheck={false}
            aria-label="New password"
          />
        </label>
        {error && (
          <div className="inline-error" role="alert">
            {error}
          </div>
        )}
        <div className="modal-actions">
          <button
            className="button"
            type="button"
            onClick={onClose}
            disabled={busy}
          >
            Cancel
          </button>
          <button className="button primary" disabled={busy} aria-busy={busy}>
            {busy && <LoadingIndicator small />}
            {busy ? 'Saving…' : 'Change password'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
