import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import { Trash2 } from 'lucide-react'

import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { LoadingIndicator, Modal, Panel } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function ProjectDelete({ name }: { name: string }) {
  const { notify } = useWorkspace()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [volumes, setVolumes] = useState(false)
  const [images, setImages] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      await api('/api/projects/' + segment(name), {
        method: 'DELETE',
        body: { volumes, images },
      })
      notify('Project deleted')
      navigate('/projects')
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <>
      <Panel title="Delete project" className="danger-section">
        <div className="danger-action">
          <p>
            Remove this environment from its runners and from URUFLOW. Other
            environments of the project are not affected.
          </p>
          <button
            className="button danger-subtle"
            onClick={() => setOpen(true)}
          >
            <Trash2 size={15} aria-hidden="true" />
            Delete project
          </button>
        </div>
      </Panel>
      {open && (
        <Modal
          title={'Delete ' + name}
          onClose={() => setOpen(false)}
          locked={busy}
        >
          <form onSubmit={submit}>
            <p className="modal-description">
              Its containers, configuration files, release history, secrets and
              access grants are deleted. Deletion waits until no release is
              running. Data volumes and registry images are kept unless you
              select them; selected items cannot be recovered.
            </p>
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={volumes}
                onChange={(e) => setVolumes(e.target.checked)}
                name="volumes"
              />
              Also delete its data volumes
            </label>
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={images}
                onChange={(e) => setImages(e.target.checked)}
                name="images"
              />
              Also delete its registry images
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
                onClick={() => setOpen(false)}
                disabled={busy}
              >
                Cancel
              </button>
              <button
                className="button danger"
                disabled={busy}
                aria-busy={busy}
              >
                {busy && <LoadingIndicator small />}
                {busy ? 'Deleting…' : 'Delete project'}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </>
  )
}
