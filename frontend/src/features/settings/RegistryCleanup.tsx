import { useState, type FormEvent } from 'react'

import { api } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { RegistrySettings } from '@/shared/types'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function RegistryCleanup({
  settings,
  saved,
  onClose,
}: {
  settings: RegistrySettings
  saved: (settings: RegistrySettings) => void
  onClose: () => void
}) {
  const { notify } = useWorkspace()
  const [retain, setRetain] = useState(String(settings.retain_releases))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      saved(
        await api<RegistrySettings>('/api/settings/registry', {
          method: 'PUT',
          body: {
            revision: settings.revision,
            retain_releases: Number(retain),
          },
        }),
      )
      notify('Image cleanup updated')
      onClose()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Modal title="Image cleanup" onClose={onClose} locked={busy}>
      <form onSubmit={submit}>
        <p className="modal-description">
          Cleanup runs ten minutes after the server starts and then daily.
          Running images, pinned images and manual tags are always kept. Enter 0
          to turn cleanup off.
        </p>
        <label>
          Successful images kept per service
          <input
            type="number"
            min={0}
            max={1000}
            value={retain}
            onChange={(e) => setRetain(e.target.value)}
            required
            name="retain_releases"
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
            {busy ? 'Saving…' : 'Save cleanup'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
