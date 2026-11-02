import { Suspense, lazy, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import { FileCode, Server } from 'lucide-react'

import { api, segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'
import { type Project } from '@/shared/types'
import { LoadingIndicator, Badge, Modal, Spinner } from '@/shared/ui'
import { errorText } from '@/shared/utils'

const CodeEditor = lazy(() => import('@/shared/ui/CodeEditor'))
export function CreateProject({ onClose }: { onClose: () => void }) {
  const { snapshot, notify } = useWorkspace()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [environment, setEnvironment] = useState('prod')
  const [service, setService] = useState('api')
  const [mode, setMode] = useState<'source' | 'image'>('source')
  const [source, setSource] = useState('')
  const [branch, setBranch] = useState('main')
  const [builder, setBuilder] = useState('')
  const [runners, setRunners] = useState<string[]>([])
  const [port, setPort] = useState('')
  const [advanced, setAdvanced] = useState(false)
  const [custom, setCustom] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const q = JSON.stringify
  const builders =
    snapshot?.agents.filter((a) => a.roles.includes('builder')) || []
  const runnerChoices =
    snapshot?.agents.filter((a) => a.roles.includes('runner')) || []
  const generated =
    [
      `workflow: ${mode === 'image' ? 'deploy_only' : runners.length ? 'build_deploy' : 'build_only'}`,
      ...(mode === 'source' && builder ? [`builder: ${q(builder)}`] : []),
      ...(runners.length
        ? ['runners:', ...runners.map((r) => '  - ' + q(r))]
        : []),
      'services:',
      `  ${service || 'api'}:`,
      mode === 'image' ? `    image: ${q(source)}` : `    git: ${q(source)}`,
      ...(mode === 'source'
        ? [
            `    branch: ${q(branch)}`,
            '    dockerfile: Dockerfile',
            '    context: .',
          ]
        : []),
      ...(port
        ? [
            '    ports:',
            ...port.split(',').map((p) => '      - ' + q(p.trim())),
          ]
        : []),
    ].join('\n') + '\n'
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const p = await api<Project>('/api/projects', {
        method: 'POST',
        body: { name, environment, content: advanced ? custom : generated },
      })
      notify('Project environment created')
      onClose()
      navigate('/projects/' + segment(p.name))
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Modal
      title="Create a project environment"
      wide
      onClose={onClose}
      locked={busy}
    >
      <form onSubmit={submit}>
        <p className="modal-description">
          Define the first service and where it should run. You can add
          dependencies, jobs and storage in the configuration editor.
        </p>
        <div className="form-grid">
          <label>
            Project name
            <input
              required
              pattern="[a-z0-9](?:[a-z0-9]|-)*"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="urufi"
              name="name"
              autoComplete="off"
              spellCheck={false}
            />
          </label>
          <label>
            Environment
            <input
              required
              pattern="[a-z0-9](?:[a-z0-9]|-)*"
              value={environment}
              onChange={(e) => setEnvironment(e.target.value)}
              name="environment"
              autoComplete="off"
              spellCheck={false}
            />
          </label>
        </div>
        <div className="form-mode">
          <button
            type="button"
            className={!advanced ? 'selected' : ''}
            aria-pressed={!advanced}
            onClick={() => setAdvanced(false)}
          >
            Guided setup
          </button>
          <button
            type="button"
            className={advanced ? 'selected' : ''}
            aria-pressed={advanced}
            onClick={() => {
              setCustom(generated)
              setAdvanced(true)
            }}
          >
            <FileCode size={15} />
            Environment YAML
          </button>
        </div>
        {advanced ? (
          <Suspense fallback={<Spinner />}>
            <CodeEditor value={custom} onChange={setCustom} />
          </Suspense>
        ) : (
          <>
            <div className="form-grid">
              <label>
                Service name
                <input
                  required
                  pattern="[a-z0-9](?:[a-z0-9]|-)*"
                  value={service}
                  onChange={(e) => setService(e.target.value)}
                  name="service"
                  autoComplete="off"
                  spellCheck={false}
                />
              </label>
              <label>
                Service source
                <select
                  value={mode}
                  onChange={(e) => {
                    setMode(e.target.value as 'source' | 'image')
                    setSource('')
                  }}
                >
                  <option value="source">Build from Git</option>
                  <option value="image">Use an existing image</option>
                </select>
              </label>
            </div>
            <label>
              {mode === 'source' ? 'Git repository' : 'Container image'}
              <input
                required
                value={source}
                onChange={(e) => setSource(e.target.value)}
                placeholder={
                  mode === 'source'
                    ? 'git@github.com:your-org/service.git'
                    : 'postgres@sha256:…'
                }
                pattern={
                  mode === 'image' ? '.+@sha256:[a-f0-9]{64}' : undefined
                }
                title={
                  mode === 'image'
                    ? 'Use the full sha256 digest: 64 lowercase hexadecimal characters.'
                    : undefined
                }
                name="source"
                autoComplete="off"
                spellCheck={false}
              />
              {mode === 'image' && (
                <small>
                  Use an immutable image reference with its full sha256 digest.
                </small>
              )}
            </label>
            {mode === 'source' && (
              <div className="form-grid">
                <label>
                  Branch
                  <input
                    required
                    value={branch}
                    onChange={(e) => setBranch(e.target.value)}
                    name="branch"
                    autoComplete="off"
                    spellCheck={false}
                  />
                </label>
                <label>
                  Builder
                  <select
                    required
                    value={builder}
                    onChange={(e) => setBuilder(e.target.value)}
                  >
                    <option value="">Select a builder</option>
                    {builders.map((a) => (
                      <option key={a.id} value={a.name}>
                        {a.name} · {a.status}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
            )}
            <fieldset>
              <legend>Runner machines</legend>
              {runnerChoices.length ? (
                <div className="runner-choices">
                  {runnerChoices.map((a) => (
                    <label key={a.id}>
                      <input
                        type="checkbox"
                        checked={runners.includes(a.name)}
                        onChange={(e) =>
                          setRunners(
                            e.target.checked
                              ? [...runners, a.name]
                              : runners.filter((n) => n !== a.name),
                          )
                        }
                        autoComplete="off"
                        spellCheck={false}
                      />
                      <Server size={15} />
                      <span>{a.name}</span>
                      <Badge value={a.status} />
                    </label>
                  ))}
                </div>
              ) : (
                <p className="muted">
                  Enroll a runner before deploying workloads.
                </p>
              )}
              {mode === 'source' && (
                <small>Leave empty for a build-only project.</small>
              )}
            </fieldset>
            <label>
              Published ports
              <small>Optional, comma separated. For example: 8080:80</small>
              <input
                value={port}
                onChange={(e) => setPort(e.target.value)}
                placeholder="8080:80"
                name="port"
                autoComplete="off"
                spellCheck={false}
              />
            </label>
          </>
        )}
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
          <button className="button primary" disabled={busy} aria-busy={busy}>
            {busy && <LoadingIndicator small />}
            {busy ? 'Validating…' : 'Create project'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
