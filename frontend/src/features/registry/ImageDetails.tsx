import { Link } from 'react-router-dom'
import { ArrowUpRight, Trash2 } from 'lucide-react'
import type { Image } from '@/shared/types'
import { segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { Badge, Code, ErrorState, Facts, Modal, Spinner } from '@/shared/ui'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { bytes, short } from '@/shared/utils'

export function ImageDetails({
  image,
  close,
  remove,
}: {
  image: Image
  close: () => void
  remove: (image: Image) => void
}) {
  const { canAdmin, snapshot } = useWorkspace()
  const { data, error, loading, reload } = useLoad<Image>(
    '/api/images/details?repository=' +
      segment(image.repository) +
      '&tag=' +
      segment(image.tag),
  )
  const selected = data || image
  const refs = (snapshot?.releases || []).filter(
    (release) =>
      selected.digest &&
      [release.image, ...Object.values(release.images || {})].some((ref) =>
        ref.endsWith('@' + selected.digest),
      ),
  )
  return (
    <Modal title={image.repository + ':' + image.tag} onClose={close}>
      {loading && <Spinner label="Reading manifest details…" />}
      {error && <ErrorState message={error} retry={reload} />}
      <Facts
        items={[
          ['Repository', selected.repository],
          ['Tag', selected.tag],
          [
            'Digest',
            selected.digest ? <Code value={selected.digest} copy /> : 'Unknown',
          ],
          [
            'Artifact size',
            selected.size_known ? bytes(selected.size) : 'Size unknown',
          ],
        ]}
      />
      <h3 className="modal-section-title">
        References in recent release history
      </h3>
      {refs.length ? (
        refs.map((release) => (
          <Link
            className="resource-choice"
            to={'/releases/' + segment(release.id)}
            key={release.id}
          >
            <span>
              {release.project}
              <small>
                {short(release.id)} ·{' '}
                <RelativeTime value={release.started_at} />
              </small>
            </span>
            <Badge value={release.status} />
            <ArrowUpRight size={15} />
          </Link>
        ))
      ) : (
        <p className="muted">
          No recent references. Older releases may use this digest.
        </p>
      )}
      <div className="modal-actions">
        <button className="button" onClick={close}>
          Close
        </button>
        {canAdmin && (
          <button
            className="button danger"
            disabled={loading || !data?.digest}
            onClick={() => remove(selected)}
          >
            <Trash2 size={14} />
            Delete manifest
          </button>
        )}
      </div>
    </Modal>
  )
}
