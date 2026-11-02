import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useVisible } from '@/shared/hooks/useVisible'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { ArrowUpRight, Package, RefreshCw, Trash2 } from 'lucide-react'

import { api, segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { type Image } from '@/shared/types'
import {
  Badge,
  Code,
  DataTable,
  EmptyState,
  ErrorState,
  Facts,
  LoadingIndicator,
  Modal,
  Panel,
  Spinner,
} from '@/shared/ui'
import { bytes, short } from '@/shared/utils'

export function RegistryCatalog() {
  const visible = useVisible()
  const { canAdmin, snapshot, confirm, fleetAccess } = useWorkspace()
  const { data, error, loading, reload } = useLoad<Image[]>(
    '/api/images',
    fleetAccess && visible,
  )
  const [selected, setSelected] = useState<Image | null>(null)
  const [loaded, setLoaded] = useState('')
  useEffect(() => {
    if (data) {
      setLoaded(new Date().toISOString())
    }
  }, [data])
  useEffect(() => {
    if (!visible || !fleetAccess) return
    const timer = setInterval(reload, 15000)
    return () => clearInterval(timer)
  }, [reload, visible, fleetAccess])
  const images = data || []
  const refs = selected
    ? (snapshot?.releases || []).filter(
        (r) =>
          selected.digest &&
          (Object.values(r.images || {}).some((image) =>
            image.includes(selected.digest),
          ) ||
            r.image.includes(selected.digest)),
      )
    : []
  const remove = (image: Image) => {
    setSelected(null)
    confirm({
      title: 'Delete manifest ' + image.repository + ':' + image.tag,
      description:
        'Delete this digest from the registry. Other tags pointing to the same manifest may also be affected. Future image pulls and rollbacks that need it can fail; running containers are not removed.',
      label: 'Delete manifest',
      success: 'Image manifest deleted',
      danger: true,
      run: async () => {
        await api('/api/images', {
          method: 'DELETE',
          body: { repository: image.repository, tag: image.tag },
        })
        reload()
      },
    })
  }
  if (!fleetAccess) {
    return (
      <EmptyState
        title="Registry access restricted"
        detail="The registry catalogue requires access to all projects. Project images remain visible in your release details."
      />
    )
  }
  return (
    <>
      {loading && !data ? (
        <Spinner label="Reading the registry catalog…" />
      ) : (
        <Panel
          title="Artifact registry"
          actions={
            <button
              className="button"
              disabled={loading}
              aria-busy={loading}
              onClick={reload}
            >
              {loading ? <LoadingIndicator small /> : <RefreshCw size={15} />}
              {loading ? 'Refreshing…' : 'Refresh catalog'}
            </button>
          }
          detail={
            <>
              Catalog sampled <RelativeTime value={loaded} /> · refreshes every
              15 seconds
            </>
          }
        >
          {error && <ErrorState message={error} retry={reload} />}
          {(!error || data) && (
            <DataTable
              scope="registry-page"
              rows={images}
              rowKey={(im) => im.repository + ':' + im.tag}
              onRow={setSelected}
              searchText={(im) =>
                im.repository + ' ' + im.tag + ' ' + im.digest
              }
              empty="No image manifests stored"
              emptyDetail="Successfully built and pushed artifacts appear in this catalog."
              columns={[
                {
                  key: 'repository',
                  title: 'Repository',
                  render: (im) => (
                    <div className="resource-title">
                      <span className="resource-icon image">
                        <Package size={18} />
                      </span>
                      <strong>{im.repository}</strong>
                    </div>
                  ),
                  sort: (im) => im.repository,
                },
                {
                  key: 'tag',
                  title: 'Tag',
                  render: (im) => <code>{im.tag}</code>,
                  sort: (im) => im.tag,
                },
                {
                  key: 'digest',
                  title: 'Digest',
                  render: (im) =>
                    im.digest ? (
                      <Code value={short(im.digest, 30)} />
                    ) : (
                      <span className="warning-text">Unavailable</span>
                    ),
                },
                {
                  key: 'size',
                  title: 'Artifact size',
                  render: (im) => bytes(im.size),
                  sort: (im) => im.size,
                },
              ]}
            />
          )}
        </Panel>
      )}
      {selected && (
        <Modal
          title={selected.repository + ':' + selected.tag}
          onClose={() => setSelected(null)}
        >
          <Facts
            items={[
              ['Repository', selected.repository],
              ['Tag', selected.tag],
              ['Digest', <Code value={selected.digest} copy />],
              ['Artifact size', bytes(selected.size)],
              ['Catalog sample', <RelativeTime value={loaded} />],
            ]}
          />
          <h3 className="modal-section-title">
            References in recent release history
          </h3>
          {refs.length ? (
            refs.map((r) => (
              <Link
                className="resource-choice"
                to={'/releases/' + segment(r.id)}
                key={r.id}
              >
                <span>
                  {r.project}
                  <small>
                    {short(r.id)} · <RelativeTime value={r.started_at} />
                  </small>
                </span>
                <Badge value={r.status} />
                <ArrowUpRight size={15} />
              </Link>
            ))
          ) : (
            <p className="muted">
              No recent references. Older releases may use this digest.
            </p>
          )}
          <div className="modal-actions">
            <button className="button" onClick={() => setSelected(null)}>
              Close
            </button>
            {canAdmin && (
              <button
                className="button danger"
                onClick={() => remove(selected)}
              >
                <Trash2 size={14} />
                Delete manifest
              </button>
            )}
          </div>
        </Modal>
      )}
    </>
  )
}
