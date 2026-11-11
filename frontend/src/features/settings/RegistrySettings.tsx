import { useState } from 'react'

import { KeyRound, Pencil } from 'lucide-react'

import { api } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import type { RegistrySettings as Settings } from '@/shared/types'
import { ErrorState, Facts, Panel, Spinner } from '@/shared/ui'
import { RelativeTime } from '@/shared/ui/RelativeTime'

import { RegistryCleanup } from './RegistryCleanup'

const url = '/api/settings/registry'

export function RegistrySettings() {
  const { confirm } = useWorkspace()
  const { data, error, reload, setData } = useLoad<Settings>(url)
  const [editing, setEditing] = useState(false)
  if (error) return <ErrorState message={error} retry={reload} />
  if (!data) return <Spinner />
  const settings = data
  function rotate() {
    confirm({
      title: 'Rotate registry password',
      description:
        'A new password is generated, the registry restarts for a few seconds and every connected agent receives the new login. Offline agents receive it when they reconnect. Releases cannot start until it finishes.',
      label: 'Rotate password',
      success: 'Registry password rotated',
      run: async () =>
        setData(
          await api<Settings>(url + '/password', {
            method: 'POST',
            body: { revision: settings.revision },
          }),
        ),
    })
  }
  return (
    <>
      <Panel
        title="Registry"
        detail="Image cleanup and the login agents use to push and pull images."
        actions={
          <>
            <button className="button small" onClick={() => setEditing(true)}>
              <Pencil size={14} />
              Cleanup
            </button>
            <button className="button small" onClick={rotate}>
              <KeyRound size={14} />
              Rotate password
            </button>
          </>
        }
      >
        <Facts
          items={[
            [
              'Image cleanup',
              settings.retain_releases > 0
                ? `Keeps the latest ${settings.retain_releases} successful images per service`
                : 'Off',
            ],
            [
              'Password',
              <>
                Changed <RelativeTime value={settings.password_at} />
              </>,
            ],
          ]}
        />
      </Panel>
      {editing && (
        <RegistryCleanup
          settings={settings}
          saved={setData}
          onClose={() => setEditing(false)}
        />
      )}
    </>
  )
}
