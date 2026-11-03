import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { Project, Release } from '@/shared/types'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { errorText, services } from '@/shared/utils'

export function DeployProject({
  project,
  onClose,
}: {
  project: Project
  onClose: () => void
}) {
  const [rebuild, setRebuild] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const { notify } = useWorkspace()
  const navigate = useNavigate()
  const builds =
    project.workflow !== 'deploy_only' &&
    services(project).some((service) => !service.image)
  return (
    <Modal title={'Deploy ' + project.name} onClose={onClose} locked={busy}>
      <p className="modal-description">
        {builds
          ? 'Build changed services and apply the saved configuration. Unchanged images are reused.'
          : 'Apply the saved images and configuration.'}
      </p>
      {builds && (
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={rebuild}
            disabled={busy}
            onChange={(event) => setRebuild(event.target.checked)}
          />
          Rebuild all images
        </label>
      )}
      {error && (
        <div className="inline-error" role="alert">
          {error}
        </div>
      )}
      <div className="modal-actions">
        <button className="button" disabled={busy} onClick={onClose}>
          Cancel
        </button>
        <button
          className="button primary"
          disabled={busy}
          aria-busy={busy}
          onClick={async () => {
            setBusy(true)
            setError('')
            try {
              const release = await api<Release>(
                '/api/projects/' + segment(project.name) + '/deploy',
                { method: 'POST', body: { rebuild } },
              )
              notify('Deployment started')
              onClose()
              navigate('/releases/' + segment(release.id) + '?tab=Logs')
            } catch (failure) {
              setError(errorText(failure))
              setBusy(false)
            }
          }}
        >
          {busy && <LoadingIndicator small />}
          {busy ? 'Starting…' : 'Deploy'}
        </button>
      </div>
    </Modal>
  )
}
