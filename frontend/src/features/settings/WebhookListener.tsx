import { useState, type FormEvent } from 'react'

import { api } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { WebhookSettings } from '@/shared/types'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function WebhookListener({
  settings,
  saved,
  onClose,
}: {
  settings: WebhookSettings
  saved: (settings: WebhookSettings) => void
  onClose: () => void
}) {
  const { notify } = useWorkspace()
  const [host, setHost] = useState(settings.host)
  const [port, setPort] = useState(String(settings.port))
  const [path, setPath] = useState(settings.path)
  const [tls, setTLS] = useState(settings.tls)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      saved(
        await api<WebhookSettings>('/api/settings/webhook', {
          method: 'PUT',
          body: {
            revision: settings.revision,
            enabled: settings.enabled,
            host,
            port: Number(port),
            path,
            tls,
          },
        }),
      )
      notify('Webhook listener updated')
      onClose()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Modal title="Webhook listener" onClose={onClose} locked={busy}>
      <form onSubmit={submit}>
        <p className="modal-description">
          Changes apply immediately. A new address or port restarts only the
          webhook listener; update repository webhooks with the new URL.
        </p>
        <label>
          Bind address
          <input
            value={host}
            onChange={(e) => setHost(e.target.value)}
            placeholder="Server bind address"
            autoComplete="off"
            spellCheck={false}
            name="host"
          />
        </label>
        <label>
          Port
          <input
            type="number"
            min={1}
            max={65535}
            value={port}
            onChange={(e) => setPort(e.target.value)}
            required
            name="port"
          />
        </label>
        <label>
          Path
          <input
            value={path}
            onChange={(e) => setPath(e.target.value)}
            pattern="/.*"
            required
            autoComplete="off"
            spellCheck={false}
            name="path"
          />
        </label>
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={tls}
            onChange={(e) => setTLS(e.target.checked)}
          />
          Serve HTTPS
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
            {busy ? 'Applying…' : 'Apply listener'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
