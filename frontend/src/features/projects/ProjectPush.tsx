import { Link } from 'react-router-dom'

import { Power } from 'lucide-react'

import { api, segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import type { PushSettings, PushSource } from '@/shared/types'
import {
  Badge,
  Code,
  DataTable,
  ErrorState,
  Facts,
  Panel,
  Spinner,
} from '@/shared/ui'

export function ProjectPush({ name }: { name: string }) {
  const { canAdmin, confirm } = useWorkspace()
  const url = '/api/projects/' + segment(name) + '/push'
  const { data, error, loading, reload, setData } = useLoad<PushSettings>(url)
  function toggle(source: PushSource) {
    const enabled = !source.enabled
    confirm({
      title: enabled ? 'Enable deploy on push' : 'Disable deploy on push',
      description: enabled
        ? `Every signed push to ${source.branch} builds and releases the exact commit that was pushed.`
        : `Pushes to ${source.branch} no longer start releases. Running containers stay as they are.`,
      label: enabled ? 'Enable' : 'Disable',
      danger: !enabled,
      success: enabled ? 'Deploy on push enabled' : 'Deploy on push disabled',
      run: async () =>
        setData(
          await api<PushSettings>(url, {
            method: 'PUT',
            body: {
              repository: source.repository,
              branch: source.branch,
              enabled,
            },
          }),
        ),
    })
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
          [
            'Push deliveries',
            <Badge value={data.receiving ? 'enabled' : 'disabled'} />,
          ],
          [
            'Webhook',
            canAdmin ? (
              <Link to="/settings/webhooks">Manage in Settings → Webhooks</Link>
            ) : (
              'Managed by administrators in Settings → Webhooks'
            ),
          ],
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
            render: (s) => <Badge value={s.enabled ? 'enabled' : 'disabled'} />,
          },
          {
            key: 'actions',
            title: '',
            render: (s) => (
              <button className="button small" onClick={() => toggle(s)}>
                <Power size={14} />
                {s.enabled ? 'Disable' : 'Enable'}
              </button>
            ),
          },
        ]}
      />
    </Panel>
  )
}
