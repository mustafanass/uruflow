import { useEffect, useLayoutEffect, useRef, useState } from 'react'

import { Download, Pause, Play, RefreshCw, Search } from 'lucide-react'

import { subscribeLogs } from '@/shared/api/log-stream'

import { count, time } from '@/shared/utils'

import { Badge } from './Badge'
import { EmptyState } from './EmptyState'

interface Line {
  id?: number
  line: string
  stream?: string
  timestamp: string | number
  agent_name?: string
  stage?: string
  key: number
}
export function LogViewer({ url }: { url: string }) {
  const [lines, setLines] = useState<Line[]>([])
  const [filter, setFilter] = useState('')
  const [following, setFollowing] = useState(true)
  const [state, setState] = useState('connecting')
  const [error, setError] = useState('')
  const [wrap, setWrap] = useState(true)
  const [version, setVersion] = useState(0)
  const [dropped, setDropped] = useState(false)
  const [visiblePage, setVisiblePage] = useState(
    document.visibilityState === 'visible',
  )
  const viewport = useRef<HTMLDivElement>(null)
  const counter = useRef(0)
  const lineCount = useRef(0)
  const position = useRef({ after: '', seen: new Set<string>() })
  const completed = useRef(false)
  const previousURL = useRef('')
  useEffect(() => {
    const changed = () => setVisiblePage(document.visibilityState === 'visible')
    document.addEventListener('visibilitychange', changed)
    return () => document.removeEventListener('visibilitychange', changed)
  }, [])
  useEffect(() => {
    if (previousURL.current !== url) {
      setLines([])
      lineCount.current = 0
      position.current = { after: '', seen: new Set<string>() }
      completed.current = false
      setDropped(false)
      previousURL.current = url
    }
    if (completed.current) return
    if (!visiblePage) {
      setState('paused')
      return
    }
    setError('')
    let pending: Line[] = []
    let discarded = false
    let timer = 0
    const flush = () => {
      clearTimeout(timer)
      timer = 0
      if (!pending.length) return
      const batch = pending
      pending = []
      if (discarded || lineCount.current + batch.length > 2000) setDropped(true)
      lineCount.current = Math.min(2000, lineCount.current + batch.length)
      setLines((old) => [...old, ...batch].slice(-2000))
    }
    const stop = subscribeLogs(url, position.current, (event, value) => {
      if (event === 'state') {
        setState(value as string)
        if (value === 'complete' || value === 'unavailable')
          completed.current = true
        if (value === 'complete' || value === 'unavailable') flush()
      } else if (event === 'error') {
        setError(value as string)
      } else if (event === 'log') {
        const line = value as Line
        line.key = ++counter.current
        line.line = line.line.replace(/\x1b\[[0-9;]*[A-Za-z]/g, '')
        pending.push(line)
        if (pending.length >= 4000) {
          pending = pending.slice(-2000)
          discarded = true
        }
        if (!timer) timer = window.setTimeout(flush, 50)
      }
    })
    return () => {
      stop()
      flush()
    }
  }, [url, version, visiblePage])
  useLayoutEffect(() => {
    if (following && viewport.current) {
      viewport.current.scrollTop = viewport.current.scrollHeight
    }
  }, [lines, following, filter, wrap])
  const visible = filter
    ? lines.filter((line) =>
        (line.line + ' ' + line.agent_name + ' ' + line.stream)
          .toLowerCase()
          .includes(filter.toLowerCase()),
      )
    : lines
  function download() {
    const blob = new Blob(
      [
        lines
          .map(
            (line) =>
              `${typeof line.timestamp === 'number' ? new Date(line.timestamp * 1000).toISOString() : line.timestamp} ${line.agent_name || ''} ${line.stream || ''} ${line.line}`,
          )
          .join('\n'),
      ],
      { type: 'text/plain' },
    )
    const href = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = href
    a.download = 'uruflow-logs.txt'
    a.click()
    URL.revokeObjectURL(href)
  }
  return (
    <section className="log-viewer">
      <div className="log-toolbar">
        <label className="filter-input">
          <Search size={15} />
          <input
            aria-label="Search loaded logs"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Search loaded output…"
            name="filter"
            autoComplete="off"
            spellCheck={false}
          />
        </label>
        <Badge value={state} />
        <div className="toolbar-actions">
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={wrap}
              onChange={(e) => setWrap(e.target.checked)}
              autoComplete="off"
              spellCheck={false}
            />
            Wrap
          </label>
          {!['complete', 'unavailable'].includes(state) && (
            <button
              className="button small"
              onClick={() => setFollowing((v) => !v)}
            >
              {following ? <Pause size={14} /> : <Play size={14} />}
              {following ? 'Pause follow' : 'Follow latest'}
            </button>
          )}
          <button
            className="icon-button"
            title="Reconnect logs"
            aria-label="Reconnect logs"
            onClick={() => {
              completed.current = false
              setVersion((v) => v + 1)
            }}
          >
            <RefreshCw size={16} />
          </button>
          <button
            className="icon-button"
            title="Download loaded logs"
            aria-label="Download loaded logs"
            disabled={!lines.length}
            onClick={download}
          >
            <Download size={16} />
          </button>
        </div>
      </div>
      {error && (
        <div className="editor-error" role="alert">
          {error}
        </div>
      )}
      {dropped && (
        <div className="log-buffer-note">
          Showing the latest 2,000 loaded lines. Earlier lines have left this
          browser buffer.
        </div>
      )}
      <div
        className={'log-output ' + (wrap ? 'wrap' : '')}
        ref={viewport}
        tabIndex={0}
        role="region"
        aria-label="Log output"
        onScroll={() => {
          const el = viewport.current
          if (el && el.scrollHeight - el.scrollTop - el.clientHeight > 48) {
            setFollowing(false)
          }
        }}
      >
        {visible.length ? (
          visible.map((line) => (
            <div className="log-line" key={line.key}>
              <time>
                {typeof line.timestamp === 'number'
                  ? new Date(line.timestamp * 1000).toLocaleTimeString()
                  : time(line.timestamp)}
              </time>
              <span className="log-source">
                {line.agent_name || line.stream || 'stdout'}
                {line.stage ? ' / ' + line.stage : ''}
              </span>
              <pre>{line.line}</pre>
            </div>
          ))
        ) : (
          <EmptyState
            title={filter ? 'No loaded lines match' : 'Waiting for output'}
            detail={
              filter
                ? 'Try a different search.'
                : 'New output appears here as the agent reports it. An idle service may not produce new lines.'
            }
          />
        )}
      </div>
      <div className="log-footer">
        <span>
          {count(visible.length, 'loaded line')}
          {state === 'complete'
            ? ' · complete'
            : state === 'unavailable'
              ? ' · stream ended'
              : following
                ? ' · following latest'
                : ' · follow paused'}
        </span>
        <span>Closing this view does not cancel a release</span>
      </div>
    </section>
  )
}
