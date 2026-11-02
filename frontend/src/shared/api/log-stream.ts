import { api } from './client'
import { readEvents, type StreamEvent } from './sse'

type Position = { after: string; seen: Set<string> }
type Receive = (event: string, value: unknown) => void

export function subscribeLogs(
  url: string,
  position: Position,
  receive: Receive,
) {
  const controller = new AbortController()
  let connection: AbortController | null = null
  let timer = 0
  let watchdog = 0
  let stopped = false
  let delay = 1000
  function finish() {
    stopped = true
    clearTimeout(timer)
    clearTimeout(watchdog)
    connection?.abort()
    controller.abort()
  }
  function unavailable(error: string) {
    receive('error', error)
    receive('state', 'unavailable')
    finish()
  }
  function expired() {
    finish()
    window.dispatchEvent(new Event('uruflow:unauthorized'))
  }
  function retry() {
    if (stopped) return
    connection?.abort()
    clearTimeout(watchdog)
    receive('state', 'reconnecting')
    clearTimeout(timer)
    timer = window.setTimeout(reconnect, delay)
    delay = Math.min(delay * 2, 10000)
  }
  async function reconnect() {
    if (stopped) return
    try {
      const session = await api<{ authenticated: boolean }>('/api/session', {
        signal: AbortSignal.any([controller.signal, AbortSignal.timeout(5000)]),
      })
      if (stopped) return
      if (!session.authenticated) return expired()
      void connect()
    } catch {
      retry()
    }
  }
  function receiveEvent(message: StreamEvent) {
    if (stopped) return
    clearTimeout(watchdog)
    watchdog = window.setTimeout(retry, 20000)
    delay = 1000
    const value = JSON.parse(message.data)
    if (message.event === 'session-expired') return expired()
    if (message.event === 'complete') {
      receive('state', 'complete')
      finish()
    } else if (message.event === 'failure') {
      const error = value?.error || 'The log stream ended.'
      if (value?.retryable === false) return unavailable(error)
      receive('error', error + ' Reconnecting…')
      retry()
    } else if (message.event === 'log') {
      const id =
        message.id || value.cursor || (value.id ? String(value.id) : '')
      if (id && position.seen.has(id)) return
      if (
        value.id &&
        /^\d+$/.test(position.after) &&
        value.id <= Number(position.after)
      )
        return
      if (typeof value.line !== 'string') throw new Error('Invalid log entry')
      receive('log', value)
      if (id) {
        position.after = id
        position.seen.add(id)
        if (position.seen.size > 2000)
          position.seen.delete(position.seen.values().next().value!)
      }
    }
  }
  async function connect() {
    if (stopped) return
    const active = new AbortController()
    connection = active
    const signal = AbortSignal.any([controller.signal, active.signal])
    const address = new URL(url, location.origin)
    if (position.after) address.searchParams.set('after', position.after)
    watchdog = window.setTimeout(retry, 20000)
    try {
      const response = await fetch(address.pathname + address.search, {
        credentials: 'same-origin',
        headers: { Accept: 'text/event-stream' },
        signal,
      })
      if (stopped || active.signal.aborted) return
      if (response.status === 401) return expired()
      if (!response.ok) {
        const value = await response.json().catch(() => null)
        const error =
          value?.error || `Logs are unavailable (HTTP ${response.status}).`
        if (
          response.status >= 400 &&
          response.status < 500 &&
          response.status !== 408
        ) {
          return unavailable(
            error +
              (response.status === 429
                ? ' Close another log view, then retry.'
                : ''),
          )
        }
        receive('error', error)
        retry()
        return
      }
      if (
        !response.body ||
        !response.headers.get('Content-Type')?.includes('text/event-stream')
      ) {
        return unavailable(
          'The server did not return a log stream. Retry or check the server connection.',
        )
      }
      receive('error', '')
      receive('state', 'following')
      await readEvents(response.body, receiveEvent, signal)
      if (!stopped && !active.signal.aborted) retry()
    } catch {
      if (!stopped && !active.signal.aborted) retry()
    }
  }
  receive('state', 'connecting')
  void connect()
  return finish
}
