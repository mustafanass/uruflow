import { useEffect, useState } from 'react'
import { Package, RefreshCw } from 'lucide-react'
import { api } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { useVisible } from '@/shared/hooks/useVisible'
import type { Image, ImageCatalog } from '@/shared/types'
import {
  DataTable,
  EmptyState,
  ErrorState,
  LoadingIndicator,
  Panel,
  Spinner,
} from '@/shared/ui'
import { RelativeTime } from '@/shared/ui/RelativeTime'
import { errorText } from '@/shared/utils'
import { ImageDetails } from './ImageDetails'

export function RegistryCatalog() {
  const visible = useVisible()
  const { confirm, fleetAccess, notify } = useWorkspace()
  const { data, error, loading, reload, setData } = useLoad<ImageCatalog>(
    '/api/registry/catalog',
    fleetAccess && visible,
  )
  const [selected, setSelected] = useState<Image | null>(null)
  useEffect(() => {
    if (!visible || !fleetAccess) return
    const timer = setInterval(reload, data?.refreshing ? 2000 : 15000)
    return () => clearInterval(timer)
  }, [reload, visible, fleetAccess, data?.refreshing])
  const refresh = async () => {
    try {
      setData(await api<ImageCatalog>('/api/registry/catalog?refresh=true'))
    } catch (error) {
      notify(errorText(error), 'error')
    }
  }
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
  if (!fleetAccess)
    return (
      <EmptyState
        title="Registry access restricted"
        detail="The registry catalogue requires access to all projects. Project images remain visible in your release details."
      />
    )
  const updating = loading || data?.refreshing
  const failure = error || data?.error
  return (
    <>
      <Panel
        title="Artifact registry"
        actions={
          <button
            className="button"
            disabled={!!updating}
            aria-busy={!!updating}
            onClick={refresh}
          >
            {updating ? <LoadingIndicator small /> : <RefreshCw size={15} />}{' '}
            {updating ? 'Refreshing…' : 'Refresh catalog'}
          </button>
        }
        detail={
          data?.sampled_at && !data.sampled_at.startsWith('0001') ? (
            <>
              Catalog sampled <RelativeTime value={data.sampled_at} /> · details
              load when opened
            </>
          ) : (
            'Repository names and tags load first'
          )
        }
      >
        {failure && <ErrorState message={failure} retry={refresh} />}
        {data?.partial && (
          <p className="subtle-note">
            Some repositories could not be refreshed. Available tags are shown.
          </p>
        )}
        {updating && !data?.images.length ? (
          <Spinner label="Reading repository names…" />
        ) : failure && !data?.images.length ? null : (
          <DataTable
            scope="registry-page"
            rows={data?.images || []}
            rowKey={(image) => image.repository + ':' + image.tag}
            onRow={setSelected}
            searchText={(image) => image.repository + ' ' + image.tag}
            empty="No image manifests stored"
            emptyDetail="Successfully built and pushed artifacts appear in this catalog."
            columns={[
              {
                key: 'repository',
                title: 'Repository',
                render: (image) => (
                  <div className="resource-title">
                    <span className="resource-icon image">
                      <Package size={18} />
                    </span>
                    <strong>{image.repository}</strong>
                  </div>
                ),
                sort: (image) => image.repository,
              },
              {
                key: 'tag',
                title: 'Tag',
                render: (image) => <code>{image.tag}</code>,
                sort: (image) => image.tag,
              },
            ]}
          />
        )}
      </Panel>
      {selected && (
        <ImageDetails
          key={selected.repository + ':' + selected.tag}
          image={selected}
          close={() => setSelected(null)}
          remove={remove}
        />
      )}
    </>
  )
}
