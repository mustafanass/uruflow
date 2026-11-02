import { useState, type FormEvent } from 'react'

import { ProjectAssignments } from '@/features/system/access/ProjectAssignments'
import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { type User } from '@/shared/types'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function ProjectAccess({
  user,
  onClose,
  saved,
}: {
  user: User
  onClose: () => void
  saved: () => void
}) {
  const [projects, setProjects] = useState(user.projects || [])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const { session, notify } = useWorkspace()
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      await api('/api/users/' + segment(user.username) + '/projects', {
        method: 'PUT',
        body: { projects },
      })
      notify('Project access updated')
      saved()
      onClose()
      if (user.username === session.user?.username) {
        window.dispatchEvent(new Event('uruflow:unauthorized'))
      }
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Modal
      title={'Project access · ' + user.username}
      onClose={onClose}
      locked={busy}
    >
      <form onSubmit={submit}>
        <ProjectAssignments value={projects} onChange={setProjects} />
        <p className="modal-description">
          Saving signs this user out of active sessions, including live log
          streams.
        </p>
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
            {busy ? 'Saving…' : 'Save assignments'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
