import { useState, type FormEvent } from 'react'

import { api } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import type { AgentReporting } from '@/shared/types'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function AgentReportingForm({
  settings,
  saved,
  onClose,
}: {
  settings: AgentReporting
  saved: (settings: AgentReporting) => void
  onClose: () => void
}) {
  const { notify } = useWorkspace()
  const [metrics, setMetrics] = useState(String(settings.metrics_sec))
  const [reconnect, setReconnect] = useState(String(settings.reconnect_sec))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      saved(
        await api<AgentReporting>('/api/settings/agents', {
          method: 'PUT',
          body: {
            revision: settings.revision,
            metrics_sec: Number(metrics),
            reconnect_sec: Number(reconnect),
          },
        }),
      )
      notify('Agent reporting updated')
      onClose()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Modal title="Agent reporting" onClose={onClose} locked={busy}>
      <form onSubmit={submit}>
        <p className="modal-description">
          Connected agents apply these values immediately and keep them until
          the next change. A longer interval also lengthens the time before a
          silent agent appears stale.
        </p>
        <label>
          Report interval in seconds
          <input
            type="number"
            min={2}
            max={300}
            value={metrics}
            onChange={(e) => setMetrics(e.target.value)}
            required
            name="metrics_sec"
          />
        </label>
        <label>
          Reconnect delay in seconds
          <input
            type="number"
            min={1}
            max={60}
            value={reconnect}
            onChange={(e) => setReconnect(e.target.value)}
            required
            name="reconnect_sec"
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
            {busy ? 'Applying…' : 'Apply to agents'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
