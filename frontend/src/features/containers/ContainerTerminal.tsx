import { useEffect, useRef, useState } from 'react'

import { RefreshCw } from 'lucide-react'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'

import { api, segment } from '@/shared/api'
import { readEvents, type StreamEvent } from '@/shared/api/sse'
import { Badge } from '@/shared/ui'
import { errorText } from '@/shared/utils'

const inputLimit = 16384

export function ContainerTerminal({
  agent,
  id,
}: {
  agent: string
  id: string
}) {
  const host = useRef<HTMLDivElement>(null)
  const [state, setState] = useState('connecting')
  const [error, setError] = useState('')
  const [version, setVersion] = useState(0)
  useEffect(() => {
    const element = host.current
    if (!element) return
    const url = `/api/containers/${segment(agent)}/${segment(id)}/terminal`
    const style = getComputedStyle(document.documentElement)
    const color = (name: string) => style.getPropertyValue(name).trim()
    const terminal = new Terminal({
      cursorBlink: true,
      fontFamily: "'IBM Plex Mono', monospace",
      fontSize: 13,
      theme: {
        background: color('--log-background'),
        foreground: color('--log-text'),
        cursor: color('--log-text'),
        selectionBackground: color('--log-border'),
      },
    })
    const fit = new FitAddon()
    const controller = new AbortController()
    const observer = new ResizeObserver(() => fit.fit())
    let session = ''
    let queue = ''
    let sending = false
    let ended = false
    let resizeTimer = 0
    function end(next: string, message = '') {
      ended = true
      session = ''
      queue = ''
      terminal.options.disableStdin = true
      setError(message)
      setState(next)
    }
    async function flush() {
      if (sending || !session || !queue) return
      sending = true
      const data = queue.slice(0, inputLimit)
      queue = queue.slice(data.length)
      try {
        await api(`${url}/${segment(session)}/input`, {
          method: 'POST',
          body: { data },
          signal: controller.signal,
        })
      } catch (failure) {
        if (!controller.signal.aborted) setError(errorText(failure))
      }
      sending = false
      void flush()
    }
    function resize() {
      clearTimeout(resizeTimer)
      resizeTimer = window.setTimeout(() => {
        if (!session) return
        api(`${url}/${segment(session)}/resize`, {
          method: 'POST',
          body: { columns: terminal.cols, rows: terminal.rows },
          signal: controller.signal,
        }).catch(() => {})
      }, 150)
    }
    function receive(message: StreamEvent) {
      const value = JSON.parse(message.data)
      if (message.event === 'session-expired') {
        end('unavailable')
        window.dispatchEvent(new Event('uruflow:unauthorized'))
      } else if (message.event === 'session') {
        session = value.id
        setState('connected')
        terminal.focus()
        void flush()
      } else if (message.event === 'output') {
        terminal.write(
          Uint8Array.from(atob(value.data), (char) => char.charCodeAt(0)),
        )
      } else if (message.event === 'complete') {
        end('complete')
      } else if (message.event === 'failure') {
        end('unavailable', value?.error || 'The terminal session ended.')
      }
    }
    async function connect() {
      try {
        const response = await fetch(
          `${url}?columns=${terminal.cols}&rows=${terminal.rows}`,
          {
            credentials: 'same-origin',
            headers: { Accept: 'text/event-stream' },
            signal: controller.signal,
          },
        )
        if (response.status === 401) {
          end('unavailable')
          window.dispatchEvent(new Event('uruflow:unauthorized'))
          return
        }
        if (!response.ok) {
          const value = await response.json().catch(() => null)
          end(
            'unavailable',
            value?.error ||
              `The terminal is unavailable (HTTP ${response.status}).`,
          )
          return
        }
        if (
          !response.body ||
          !response.headers.get('Content-Type')?.includes('text/event-stream')
        ) {
          end('unavailable', 'The server did not return a terminal stream.')
          return
        }
        await readEvents(response.body, receive, controller.signal)
        if (!ended && !controller.signal.aborted)
          end('unavailable', 'The terminal connection closed.')
      } catch {
        if (!ended && !controller.signal.aborted)
          end('unavailable', 'The terminal connection was interrupted.')
      }
    }
    terminal.loadAddon(fit)
    terminal.open(element)
    fit.fit()
    observer.observe(element)
    terminal.onData((data) => {
      queue += data
      void flush()
    })
    terminal.onResize(resize)
    setError('')
    setState('connecting')
    void connect()
    return () => {
      controller.abort()
      clearTimeout(resizeTimer)
      observer.disconnect()
      terminal.dispose()
    }
  }, [agent, id, version])
  return (
    <section className="log-viewer">
      <div className="log-toolbar">
        <Badge value={state} />
        <div className="toolbar-actions">
          <button
            className="icon-button"
            title="Open a new terminal session"
            aria-label="Open a new terminal session"
            onClick={() => setVersion((v) => v + 1)}
          >
            <RefreshCw size={16} />
          </button>
        </div>
      </div>
      {error && (
        <div className="editor-error" role="alert">
          {error}
        </div>
      )}
      <div className="terminal-output" ref={host} />
      <div className="log-footer">
        <span>Commands run inside this container as its configured user</span>
        <span>Leaving this tab ends the shell session</span>
      </div>
    </section>
  )
}
