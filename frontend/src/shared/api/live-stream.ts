import { api } from './client'

type Listener = (event: string, value: unknown) => void

export function subscribeLive(username: string, receive: Listener) {
  const controller = new AbortController()
  let source: EventSource | null = null
  let timer = 0
  let stopped = false
  let leader = false
  let connectionState = 'connecting'
  let delay = 1000
  let latest: Record<string, unknown> | null = null
  const shared = typeof BroadcastChannel !== 'undefined' && !!navigator.locks
  let channel: BroadcastChannel | null = null
  try {
    if (shared) channel = new BroadcastChannel('uruflow:live:' + username)
  } catch {}
  function deliver(event: string, value: unknown) {
    if (stopped) return
    if (event === 'connection') connectionState = value as string
    if (event === 'snapshot') latest = value as Record<string, unknown>
    if (event === 'snapshot-update')
      latest = { ...latest, ...(value as object) }
    receive(event, value)
  }
  function publish(event: string, value: unknown) {
    if (stopped) return
    deliver(event, value)
    channel?.postMessage({ event, value })
  }
  async function reconnect() {
    if (stopped) return
    try {
      const session = await api<{ authenticated: boolean }>('/api/session', {
        signal: AbortSignal.any([controller.signal, AbortSignal.timeout(5000)]),
      })
      if (stopped) return
      if (!session.authenticated) {
        publish('session-expired', null)
        return
      }
      connect()
    } catch {
      if (!stopped) retry()
    }
  }
  function retry() {
    if (stopped) return
    source?.close()
    source = null
    publish('connection', 'reconnecting')
    clearTimeout(timer)
    timer = window.setTimeout(reconnect, delay)
    delay = Math.min(delay * 2, 10000)
  }
  function connect() {
    if (stopped) return
    const next = new EventSource('/api/stream')
    source = next
    next.onopen = () => {
      if (source !== next) return
      delay = 1000
      publish('connection', 'live')
    }
    next.onerror = () => {
      if (source === next) retry()
    }
    for (const event of [
      'snapshot',
      'snapshot-update',
      'heartbeat',
      'session-expired',
    ]) {
      next.addEventListener(event, (message) => {
        if (source !== next || stopped) return
        try {
          publish(event, JSON.parse((message as MessageEvent).data))
          if (event === 'session-expired') source?.close()
        } catch {
          retry()
        }
      })
    }
  }
  let release = () => {}
  if (channel) {
    channel.onmessage = ({ data }) => {
      if (data.event === 'request') {
        if (leader) {
          if (latest) channel?.postMessage({ event: 'snapshot', value: latest })
          channel?.postMessage({ event: 'connection', value: connectionState })
        }
      } else if (!leader) {
        deliver(data.event, data.value)
      }
    }
    channel.postMessage({ event: 'request' })
    navigator.locks
      .request(
        'uruflow:live:' + username,
        { signal: controller.signal },
        async () => {
          if (stopped) return
          leader = true
          publish('connection', 'connecting')
          connect()
          await new Promise<void>((resolve) => {
            release = resolve
          })
        },
      )
      .catch(() => {
        if (!stopped) {
          channel?.close()
          channel = null
          connect()
        }
      })
  } else {
    connect()
  }
  return () => {
    stopped = true
    controller.abort()
    clearTimeout(timer)
    source?.close()
    channel?.close()
    release()
  }
}
