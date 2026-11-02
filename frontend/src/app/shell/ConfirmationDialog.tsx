import { useEffect, useState } from 'react'

import { useWorkspace } from '@/shared/hooks'
import { LoadingIndicator, Modal } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function ConfirmationDialog() {
  const { confirmation, confirm, notify } = useWorkspace()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => setError(''), [confirmation])
  if (!confirmation) {
    return null
  }
  return (
    <Modal
      title={confirmation.title}
      locked={busy}
      onClose={() => confirm(null)}
    >
      <p className="modal-description">{confirmation.description}</p>
      {error && (
        <div className="inline-error" role="alert">
          {error}
        </div>
      )}
      <div className="modal-actions">
        <button
          className="button"
          disabled={busy}
          onClick={() => confirm(null)}
        >
          Cancel
        </button>
        <button
          className={'button ' + (confirmation.danger ? 'danger' : 'primary')}
          aria-busy={busy}
          disabled={busy}
          onClick={async () => {
            setBusy(true)
            try {
              await confirmation.run()
              notify(confirmation.success || 'Request accepted')
              confirm(null)
            } catch (e) {
              setError(errorText(e))
            } finally {
              setBusy(false)
            }
          }}
        >
          {busy && <LoadingIndicator small />}
          {busy ? 'Working…' : confirmation.label}
        </button>
      </div>
    </Modal>
  )
}
