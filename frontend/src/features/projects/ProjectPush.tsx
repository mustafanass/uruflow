import { useState } from 'react'

import { api, segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import type { PushSettings, PushSource } from '@/shared/types'
import { Code, DataTable, ErrorState, Facts, Panel, Spinner } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function ProjectPush({ name }: { name: string }) {
  const { notify } = useWorkspace()
  const url = '/api/projects/' + segment(name) + '/push'
  const { data, error, loading, reload, setData } = useLoad<PushSettings>(url)
  const [saving, setSaving] = useState(false)
  async function toggle(source: PushSource, enabled: boolean) {
    setSaving(true)
    try {
      setData(
        await api<PushSettings>(url, {
          method: 'PUT',
          body: {
            repository: source.repository,
            branch: source.branch,
            enabled,
          },
        }),
      )
      notify(enabled ? 'Deploy on push enabled' : 'Deploy on push disabled')
    } catch (failure) {
      notify(errorText(failure), 'error')
    } finally {
      setSaving(false)
    }
  }
  return error ? (
    <ErrorState message={error} retry={reload} />
  ) : loading || !data ? (
    <Spinner />
  ) : (
    <Panel
      title="Deploy on push"
      detail="A push to an enabled branch builds and releases that exact commit."
    >
      <Facts
        items={[
          ['Webhook URL', <Code value={data.endpoint} copy />],
          ['Secret', <Code value="webhook.secret" />],
          ['Events', 'Push'],
        ]}
      />
      <DataTable
        scope="project-push"
        rows={data.sources}
        rowKey={(s) => s.repository + '@' + s.branch}
        empty="No services build from source"
        columns={[
          {
            key: 'repository',
            title: 'Repository',
            render: (s) => <Code value={s.repository} />,
          },
          {
            key: 'branch',
            title: 'Branch',
            render: (s) => s.branch,
          },
          {
            key: 'services',
            title: 'Services',
            render: (s) =>
              s.services.map((service) => service || name).join(', '),
          },
          {
            key: 'enabled',
            title: 'Deploy on push',
            render: (s) => (
              <label className="checkbox-label">
                <input
                  type="checkbox"
                  checked={s.enabled}
                  disabled={saving}
                  onChange={(event) => toggle(s, event.target.checked)}
                />
                {s.enabled ? 'Enabled' : 'Disabled'}
              </label>
            ),
          },
        ]}
      />
    </Panel>
  )
}
