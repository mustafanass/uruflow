import { useEffect, useState, type FormEvent } from 'react'

import { ArrowRight, Download, ShieldCheck } from 'lucide-react'

import { api } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { type Enrollment } from '@/shared/types'
import { Badge, CopyButton, LoadingIndicator, Modal } from '@/shared/ui'
import { errorText } from '@/shared/utils'

export function EnrollAgent({ onClose }: { onClose: () => void }) {
  const { snapshot, notify } = useWorkspace()
  const [name, setName] = useState('')
  const [roles, setRoles] = useState(['runner'])
  const [result, setResult] = useState<Enrollment | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const connected =
    result &&
    snapshot?.agents.find((a) => a.id === result.agent.id)?.status === 'online'
  useEffect(() => {
    if (!result || connected || saved) return
    const guard = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', guard)
    return () => window.removeEventListener('beforeunload', guard)
  }, [result, connected, saved])
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      setResult(
        await api<Enrollment>('/api/agents', {
          method: 'POST',
          body: { name, roles },
        }),
      )
      notify('Agent enrolled. Complete setup on its machine.')
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal
      title={result ? 'Connect ' + result.agent.name : 'Enroll an agent'}
      wide
      onClose={onClose}
      locked={busy || (!!result && !connected && !saved)}
    >
      {result ? (
        <>
          <div className="enrollment-status">
            <ShieldCheck size={22} />
            <div>
              <strong>
                {connected ? 'Agent connected' : 'Enrollment created'}
              </strong>
              <p>
                {connected
                  ? 'The agent is now reporting over UFP.'
                  : 'Waiting for the agent’s first authenticated UFP connection.'}
              </p>
            </div>
            <Badge value={connected ? 'online' : 'pending'} />
          </div>
          <ol className="enrollment-steps">
            <li>
              <h3>Install and initialize the agent</h3>
              <p>
                Install the matching uruflow-agent release on the target
                machine, then run this command. Its enrollment key is shown only
                here.
              </p>
              <div className="secret-command">
                <pre>{result.command}</pre>
                <CopyButton
                  value={result.command}
                  label="Copy enrollment command"
                />
              </div>
            </li>
            <li>
              <h3>Install the server trust root</h3>
              <p>
                Download the CA certificate and place it at{' '}
                <code>/etc/uruflow/ca.crt</code> on the agent machine.
              </p>
              <a
                className="button small"
                href={result.trust_root}
                download="uruflow-ca.crt"
              >
                <Download size={15} />
                Download CA certificate
              </a>
            </li>
            <li>
              <h3>Start the agent service</h3>
              <p>
                Start the installed uruflow-agent service. This view updates
                when the machine connects.
              </p>
            </li>
          </ol>
          <div className="modal-actions">
            {!connected && (
              <label className="checkbox-label">
                <input
                  type="checkbox"
                  checked={saved}
                  onChange={(event) => setSaved(event.target.checked)}
                />
                I saved the enrollment command
              </label>
            )}
            <button
              className="button primary"
              onClick={onClose}
              disabled={!connected && !saved}
            >
              {connected ? 'Done' : 'Close setup'}
            </button>
          </div>
        </>
      ) : (
        <form onSubmit={submit}>
          <p className="modal-description">
            Agents connect outward to this control plane. Choose what this
            machine is allowed to do.
          </p>
          <label>
            Agent name
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="runner-01"
              required
              pattern="[a-z0-9](?:[a-z0-9]|-)*"
              name="name"
              autoComplete="off"
              spellCheck={false}
            />
          </label>
          <fieldset>
            <legend>Roles</legend>
            {[
              {
                role: 'builder',
                title: 'Builder',
                detail:
                  'Checks out source, builds images and pushes artifacts.',
              },
              {
                role: 'runner',
                title: 'Runner',
                detail: 'Pulls release images and runs project workloads.',
              },
            ].map((item) => (
              <label className="role-choice" key={item.role}>
                <input
                  type="checkbox"
                  checked={roles.includes(item.role)}
                  onChange={(e) =>
                    setRoles(
                      e.target.checked
                        ? [...roles, item.role]
                        : roles.filter((r) => r !== item.role),
                    )
                  }
                  autoComplete="off"
                  spellCheck={false}
                />
                <span>
                  <strong>{item.title}</strong>
                  <small>{item.detail}</small>
                </span>
              </label>
            ))}
          </fieldset>
          {error && (
            <div className="inline-error" role="alert">
              {error}
            </div>
          )}
          <div className="modal-actions">
            <button
              type="button"
              className="button"
              onClick={onClose}
              disabled={busy}
            >
              Cancel
            </button>
            <button
              className="button primary"
              disabled={busy || !roles.length}
              aria-busy={busy}
            >
              {busy ? 'Enrolling…' : 'Create enrollment'}
              {busy ? <LoadingIndicator small /> : <ArrowRight size={15} />}
            </button>
          </div>
        </form>
      )}
    </Modal>
  )
}
