import { useState, type FormEvent } from 'react'

import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { type User } from '@/shared/types'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { PasswordInput } from '@/shared/ui/PasswordInput'
import { errorText } from '@/shared/utils'

export function ResetPassword({
  user,
  onClose,
}: {
  user: User
  onClose: () => void
}) {
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const { notify } = useWorkspace()
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      await api('/api/users/' + segment(user.username) + '/password', {
        method: 'PUT',
        body: { password },
      })
      notify(
        'Password reset; all sessions for ' + user.username + ' were revoked.',
      )
      onClose()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Modal
      title={'Reset password · ' + user.username}
      onClose={onClose}
      locked={busy}
    >
      <form onSubmit={submit}>
        <p className="modal-description">
          Set a new password and share it privately with this user. This signs
          the account out of all active sessions.
        </p>
        <label>
          New password
          <PasswordInput
            autoComplete="new-password"
            minLength={15}
            maxLength={72}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            name="password"
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
            {busy ? 'Resetting…' : 'Reset password'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
