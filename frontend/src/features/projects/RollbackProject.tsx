import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { api, segment } from '@/shared/api'
import { useLoad } from '@/shared/hooks'
import { type Project, type Release } from '@/shared/types'
import {
  Code,
  EmptyState,
  ErrorState,
  Facts,
  LoadingIndicator,
  Modal,
  Spinner,
} from '@/shared/ui'
import { errorText, short, time } from '@/shared/utils'
import { useWorkspace } from '@/shared/hooks'
import { currentRelease } from './release-state'

export function RollbackProject({
  project,
  onClose,
}: {
  project: Project
  onClose: () => void
}) {
  const { data, error, loading, reload } = useLoad<Release[]>(
    '/api/projects/' + segment(project.name) + '/releases',
  )
  const [selected, setSelected] = useState('')
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState('')
  const navigate = useNavigate()
  const { snapshot } = useWorkspace()
  const releases = (data || []).filter(
    (r) =>
      r.status === 'succeeded' &&
      !r.artifacts_expired &&
      (r.image || Object.keys(r.images || {}).length) &&
      !currentRelease(snapshot, r),
  )
  const chosen = releases.find((r) => r.id === selected)
  return (
    <Modal title={'Rollback ' + project.name} onClose={onClose} locked={busy}>
      <p className="modal-description">
        Choose a successful release. URUFLOW restores its recorded configuration
        and exact service artifacts without rebuilding.
      </p>
      {loading ? (
        <Spinner />
      ) : error ? (
        <ErrorState message={error} retry={reload} />
      ) : releases.length ? (
        <>
          <label>
            Restore release
            <select
              value={selected}
              onChange={(e) => setSelected(e.target.value)}
            >
              <option value="">Choose a successful release</option>
              {releases.map((r) => (
                <option value={r.id} key={r.id}>
                  {short(r.id)} · {time(r.started_at)} · {short(r.commit)}
                </option>
              ))}
            </select>
          </label>
          {chosen && (
            <Facts
              items={[
                ['Release', <Code value={chosen.id} />],
                [
                  'Artifact count',
                  Object.keys(chosen.images || {}).length || 1,
                ],
                [
                  'Original runners',
                  chosen.targets?.map((t) => t.agent_name).join(', '),
                ],
                ['Source commit', <Code value={chosen.commit} />],
              ]}
            />
          )}
        </>
      ) : (
        <EmptyState title="No successful releases to restore" />
      )}
      {failure && (
        <div className="inline-error" role="alert">
          {failure}
        </div>
      )}
      <div className="modal-actions">
        <button className="button" onClick={onClose} disabled={busy}>
          Cancel
        </button>
        <button
          className="button primary"
          disabled={!selected || busy}
          aria-busy={busy}
          onClick={async () => {
            setBusy(true)
            try {
              const release = await api<Release>(
                '/api/projects/' + segment(project.name) + '/rollback',
                { method: 'POST', body: { release_id: selected } },
              )
              onClose()
              navigate('/releases/' + segment(release.id) + '?tab=Logs')
            } catch (e) {
              setFailure(errorText(e))
              setBusy(false)
            }
          }}
        >
          {busy && <LoadingIndicator small />}
          {busy ? 'Starting…' : 'Restore selected release'}
        </button>
      </div>
    </Modal>
  )
}
