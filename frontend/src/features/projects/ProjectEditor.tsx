import { Suspense, lazy, useEffect, useState } from 'react'

import { diffLines } from 'diff'
import { Save } from 'lucide-react'

import { APIError, api, segment } from '@/shared/api'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { type Document, type Project } from '@/shared/types'
import {
  CopyButton,
  ErrorState,
  LoadingIndicator,
  Panel,
  Spinner,
} from '@/shared/ui'
import { errorText } from '@/shared/utils'

const CodeEditor = lazy(() => import('@/shared/ui/CodeEditor'))
export function ProjectEditor({
  project,
  variables,
  onDirtyChange,
}: {
  project: Project
  variables: boolean
  onDirtyChange?: (changed: boolean) => void
}) {
  const { data, error, loading, reload, setData } = useLoad<Document>(
    `/api/projects/${segment(project.name)}/${variables ? 'variables' : 'configuration'}`,
  )
  const { notify, setDirty, confirm } = useWorkspace()
  const [content, setContent] = useState('')
  const [saving, setSaving] = useState(false)
  const [failure, setFailure] = useState('')
  const [diff, setDiff] = useState(false)
  const [conflict, setConflict] = useState(false)
  const changed = !!data && content !== data.content
  const editorKey = project.name + (variables ? '/variables' : '/configuration')
  useEffect(() => {
    if (data) {
      setContent(data.content)
    }
    setFailure('')
    setConflict(false)
  }, [data])
  useEffect(() => {
    setDirty(changed, editorKey)
    return () => setDirty(false, editorKey)
  }, [changed, editorKey, setDirty])
  useEffect(() => {
    onDirtyChange?.(changed)
  }, [changed, onDirtyChange])
  if (error) {
    return <ErrorState message={error} retry={reload} />
  }
  if (loading || !data) {
    return <Spinner />
  }
  async function save() {
    setSaving(true)
    setFailure('')
    try {
      const saved = await api<Document>(
        `/api/projects/${segment(project.name)}/${variables ? 'variables' : 'configuration'}`,
        { method: 'PUT', body: { content, revision: data!.revision } },
      )
      setData(saved)
      setContent(saved.content)
      setDirty(false, editorKey)
      notify(variables ? 'Project variables saved' : 'Configuration saved')
    } catch (e) {
      setFailure(
        e instanceof APIError && e.status === 409
          ? 'This file changed on the server. Copy your edits, then load the latest version before saving.'
          : errorText(e),
      )
      setConflict(e instanceof APIError && e.status === 409)
    } finally {
      setSaving(false)
    }
  }
  function loadLatest() {
    confirm({
      title: 'Load the latest version?',
      description:
        'This replaces your unsaved edits. Copy them first if you need to keep them.',
      label: 'Load latest',
      success: 'Latest version loaded',
      run: async () => {
        const latest = await api<Document>(
          `/api/projects/${segment(project.name)}/${variables ? 'variables' : 'configuration'}`,
        )
        setData(latest)
        setContent(latest.content)
        setDirty(false, editorKey)
        setConflict(false)
        setFailure('')
        setDiff(false)
      },
    })
  }
  return (
    <Panel
      className="editor-panel"
      title={variables ? 'Secrets & variables' : 'Project configuration'}
      detail={
        variables ? 'Saved secrets reopen as references.' : project.source
      }
      actions={
        <>
          <button
            className={'button small ' + (diff ? 'pressed' : '')}
            disabled={!changed}
            onClick={() => setDiff((v) => !v)}
          >
            Review changes
          </button>
          <button
            className="button small"
            disabled={!changed || saving}
            onClick={() => {
              if (conflict) {
                loadLatest()
                return
              }
              setContent(data.content)
              setFailure('')
            }}
          >
            Discard edits
          </button>
          <button
            className="button primary small"
            disabled={!changed || saving}
            aria-busy={saving}
            onClick={save}
          >
            {saving ? <LoadingIndicator small /> : <Save size={14} />}
            {saving ? 'Validating…' : 'Save changes'}
          </button>
        </>
      }
    >
      {variables && (
        <div className="editor-guidance">
          <code>NAME=value</code> or <code>secret NAME=value</code>. Replace a
          secret reference to rotate it.
        </div>
      )}
      {failure && (
        <div className="editor-error" role="alert">
          {failure}
          {conflict && (
            <div className="inline-actions">
              <CopyButton value={content} label="Copy unsaved edits" />
              <button className="button small" onClick={loadLatest}>
                Load latest
              </button>
            </div>
          )}
        </div>
      )}
      {diff ? (
        <pre className="diff-view">
          {diffLines(data.content, content).map((part, i) => (
            <span
              key={i}
              className={part.added ? 'added' : part.removed ? 'removed' : ''}
            >
              {part.value.split('\n').map((line, j) =>
                line ? (
                  <span key={j}>
                    {part.added ? '+ ' : part.removed ? '- ' : '  '}
                    {line}
                    {'\n'}
                  </span>
                ) : null,
              )}
            </span>
          ))}
        </pre>
      ) : (
        <Suspense fallback={<Spinner label="Loading editor…" />}>
          <CodeEditor
            height="var(--project-editor-height)"
            value={content}
            onChange={setContent}
            variables={variables}
            readOnly={saving}
          />
        </Suspense>
      )}
      <div className="editor-footer">
        <span>
          {changed
            ? 'Unsaved changes'
            : variables
              ? 'Saved variables'
              : 'Saved configuration'}
        </span>
        <span>Save first, then deploy.</span>
      </div>
    </Panel>
  )
}
