import { useState, type FormEvent } from 'react'

import { ProjectAssignments } from '@/features/system/access/ProjectAssignments'
import { api } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { PasswordInput } from '@/shared/ui/PasswordInput'
import { errorText } from '@/shared/utils'

export function AddUser({
  onClose,
  saved,
}: {
  onClose: () => void
  saved: () => void
}) {
  const { notify } = useWorkspace()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('viewer')
  const [projects, setProjects] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      await api('/api/users', {
        method: 'POST',
        body: { username, password, role, projects },
      })
      notify('User added')
      saved()
      onClose()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Modal title="Add workspace user" onClose={onClose} locked={busy}>
      <form onSubmit={submit}>
        <label>
          Username
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            pattern="[a-z0-9](?:[a-z0-9]|-)*"
            required
            autoComplete="off"
            name="username"
            spellCheck={false}
          />
        </label>
        <label>
          Initial password
          <PasswordInput
            autoComplete="new-password"
            minLength={15}
            maxLength={72}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            name="password"
            spellCheck={false}
            aria-label="Initial password"
          />
        </label>
        <label>
          Role
          <select value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="viewer">Viewer</option>
            <option value="operator">Operator</option>
            <option value="admin">Administrator</option>
          </select>
        </label>
        {role !== 'admin' && (
          <ProjectAssignments value={projects} onChange={setProjects} />
        )}
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
            {busy ? 'Creating…' : 'Create user'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
