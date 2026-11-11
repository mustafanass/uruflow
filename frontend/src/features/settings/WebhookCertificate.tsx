import { useState, type FormEvent } from 'react'

import { api } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { WebhookSettings } from '@/shared/types'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function WebhookCertificate({
  settings,
  saved,
  onClose,
}: {
  settings: WebhookSettings
  saved: (settings: WebhookSettings) => void
  onClose: () => void
}) {
  const { notify } = useWorkspace()
  const [certificate, setCertificate] = useState('')
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  async function apply(nextCertificate: string, nextKey: string) {
    setBusy(true)
    try {
      saved(
        await api<WebhookSettings>('/api/settings/webhook/certificate', {
          method: 'PUT',
          body: {
            revision: settings.revision,
            certificate: nextCertificate,
            key: nextKey,
          },
        }),
      )
      notify(
        nextCertificate
          ? 'Webhook certificate updated'
          : 'Webhooks use the server certificate',
      )
      onClose()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    void apply(certificate, key)
  }
  return (
    <Modal title="Webhook certificate" onClose={onClose} locked={busy}>
      <form onSubmit={submit}>
        <p className="modal-description">
          Public Git providers must trust this certificate. Without one, the
          listener uses the server certificate signed by the URUFLOW CA.
        </p>
        <label>
          Certificate chain (PEM)
          <textarea
            rows={6}
            value={certificate}
            onChange={(e) => setCertificate(e.target.value)}
            required
            spellCheck={false}
            name="certificate"
          />
        </label>
        <label>
          Private key (PEM)
          <textarea
            rows={6}
            value={key}
            onChange={(e) => setKey(e.target.value)}
            required
            autoComplete="off"
            spellCheck={false}
            name="key"
          />
        </label>
        {error && (
          <div className="inline-error" role="alert">
            {error}
          </div>
        )}
        <div className="modal-actions">
          {settings.certificate && (
            <button
              className="button"
              type="button"
              onClick={() => void apply('', '')}
              disabled={busy}
            >
              Use server certificate
            </button>
          )}
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
            {busy ? 'Applying…' : 'Save certificate'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
