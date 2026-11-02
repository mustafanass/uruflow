import { useState } from 'react'

import { KeyRound, Plus, ShieldCheck, Trash2 } from 'lucide-react'

import { AddUser } from '@/features/system/access/AddUser'
import { ProjectAccess } from '@/features/system/access/ProjectAccess'
import { ResetPassword } from '@/features/system/access/ResetPassword'
import { api, segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { type User } from '@/shared/types'
import { DataTable, ErrorState, Panel, Spinner } from '@/shared/ui'
import { time } from '@/shared/utils'

export function AccessManagement() {
  const { data, error, loading, reload } = useLoad<User[]>('/api/users')
  const { session, confirm } = useWorkspace()
  const [editing, setEditing] = useState<User | null>(null)
  const [resetting, setResetting] = useState<User | null>(null)
  const [adding, setAdding] = useState(false)
  return (
    <>
      {error ? (
        <ErrorState message={error} retry={reload} />
      ) : loading && !data ? (
        <Spinner />
      ) : (
        <Panel
          title="Workspace access"
          detail="Roles control actions; project assignments control visible data. Changes revoke the affected user’s sessions."
          actions={
            <button
              className="button primary small"
              onClick={() => setAdding(true)}
            >
              <Plus size={14} />
              Add user
            </button>
          }
        >
          <DataTable
            scope="access-management"
            rows={data || []}
            rowKey={(u) => u.username}
            searchText={(u) => u.username + ' ' + u.role}
            columns={[
              {
                key: 'user',
                title: 'User',
                render: (u) => (
                  <div className="resource-title">
                    <span className="resource-icon user">
                      <ShieldCheck size={17} />
                    </span>
                    <div>
                      <strong>
                        {u.username}
                        {u.username === session.user?.username ? ' (you)' : ''}
                      </strong>
                      <small>Created {time(u.created_at)}</small>
                    </div>
                  </div>
                ),
              },
              {
                key: 'role',
                title: 'Role',
                render: (u) => (
                  <select
                    className="role-select"
                    aria-label={'Role for ' + u.username}
                    value={u.role}
                    onChange={(e) => {
                      const role = e.target.value
                      confirm({
                        title: 'Change ' + u.username + ' to ' + role,
                        description:
                          'This changes the account’s permissions and signs it out of active sessions. The last administrator cannot be demoted.',
                        label: 'Change role',
                        success: 'Role updated',
                        run: async () => {
                          await api(
                            '/api/users/' + segment(u.username) + '/role',
                            { method: 'PUT', body: { role } },
                          )
                          reload()
                          if (u.username === session.user?.username) {
                            window.dispatchEvent(
                              new Event('uruflow:unauthorized'),
                            )
                          }
                        },
                      })
                    }}
                  >
                    <option value="viewer">Viewer</option>
                    <option value="operator">Operator</option>
                    <option value="admin">Administrator</option>
                  </select>
                ),
              },
              {
                key: 'permissions',
                title: 'Project access',
                render: (u) => (
                  <div>
                    <span>
                      {u.role === 'admin' || u.projects?.includes('*')
                        ? 'All projects'
                        : u.projects?.length
                          ? u.projects.join(', ')
                          : 'No projects assigned'}
                    </span>
                    {u.role !== 'admin' && (
                      <button
                        className="text-button"
                        onClick={() => setEditing(u)}
                      >
                        Edit assignments
                      </button>
                    )}
                  </div>
                ),
              },
              {
                key: 'actions',
                title: '',
                render: (u) =>
                  u.username !== session.user?.username && (
                    <div className="inline-actions">
                      <button
                        className="icon-button"
                        title={'Reset password for ' + u.username}
                        aria-label={'Reset password for ' + u.username}
                        onClick={() => setResetting(u)}
                      >
                        <KeyRound size={16} />
                      </button>
                      <button
                        className="icon-button danger-text"
                        title={'Remove ' + u.username}
                        aria-label={'Remove ' + u.username}
                        onClick={() =>
                          confirm({
                            title: 'Remove user ' + u.username,
                            description:
                              'Remove this account and revoke its access. The last administrator cannot be removed.',
                            label: 'Remove user',
                            success: 'User removed',
                            danger: true,
                            run: async () => {
                              await api('/api/users/' + segment(u.username), {
                                method: 'DELETE',
                              })
                              reload()
                            },
                          })
                        }
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  ),
              },
            ]}
          />
        </Panel>
      )}
      {adding && <AddUser onClose={() => setAdding(false)} saved={reload} />}
      {editing && (
        <ProjectAccess
          user={editing}
          onClose={() => setEditing(null)}
          saved={reload}
        />
      )}
      {resetting && (
        <ResetPassword user={resetting} onClose={() => setResetting(null)} />
      )}
    </>
  )
}
