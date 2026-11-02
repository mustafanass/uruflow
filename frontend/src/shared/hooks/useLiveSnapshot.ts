import { useCallback, useEffect, useRef, useState } from 'react'

import { subscribeLive } from '@/shared/api/live-stream'
import { type Sample, type Snapshot } from '@/shared/types'
import { available } from '@/shared/utils'
import { WorkspaceState } from '@/shared/workspace/context'

const emptySamples: Sample[] = []

export function useLiveSnapshot(expire: () => void, username: string) {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [stale, setStale] = useState(false)
  const [connection, setConnection] =
    useState<WorkspaceState['connection']>('connecting')
  const history = useRef(new Map<string, Sample[]>())
  const current = useRef<Snapshot | null>(null)
  useEffect(() => {
    let timer = 0
    let outdated = false
    setStale(false)
    const received = () => {
      if (outdated) {
        outdated = false
        setStale(false)
      }
      clearTimeout(timer)
      timer = window.setTimeout(() => {
        outdated = true
        setStale(true)
      }, 6500)
    }
    const unsubscribe = subscribeLive(username, (event, data) => {
      if (event === 'session-expired') {
        expire()
        return
      }
      if (event === 'connection') {
        setConnection(data as WorkspaceState['connection'])
        return
      }
      if (event === 'heartbeat') {
        received()
        setConnection('live')
        return
      }
      if (event !== 'snapshot' && event !== 'snapshot-update') return
      try {
        const value = (
          event === 'snapshot-update'
            ? { ...current.current, ...(data as object) }
            : data
        ) as Snapshot
        current.current = value
        value.agents ??= []
        value.projects ??= []
        value.containers ??= []
        value.releases ??= []
        value.alerts ??= []
        value.activity ??= []
        value.problems ??= []
        value.graphs ??= {}
        value.project_states ??= {}
        const record = (key: string, sample: Sample) => {
          if (!Number.isFinite(sample.at) || sample.at <= 0) {
            return
          }
          const values = history.current.get(key) || []
          if (values.length && sample.at <= values[values.length - 1].at) {
            return
          }
          history.current.set(key, [...values.slice(-119), sample])
        }
        for (const a of value.agents) {
          const m = a.metrics
          if (m) {
            record('agent/' + a.id, {
              at: Date.parse(m.collected_at),
              cpu: available(m, 'cpu') ? m.cpu_percent : null,
              memory: available(m, 'memory') ? m.memory_used : null,
            })
          }
        }
        if (value.host) {
          record('server', {
            at: Date.parse(value.host.collected_at),
            cpu: available(value.host, 'cpu') ? value.host.cpu_percent : null,
            memory: available(value.host, 'memory')
              ? value.host.memory_used
              : null,
          })
        }
        const currentContainers = new Set(
          value.containers.map((c) => 'container/' + c.agent_id + '/' + c.id),
        )
        for (const key of history.current.keys())
          if (key.startsWith('container/') && !currentContainers.has(key)) {
            history.current.delete(key)
          }
        for (const c of value.containers)
          if (c.stats_available) {
            record('container/' + c.agent_id + '/' + c.id, {
              at: Date.parse(c.observed_at),
              cpu: c.cpu_percent,
              memory: c.memory_usage,
              rx: c.network_rx,
              tx: c.network_tx,
            })
          }
        setSnapshot(value)
        received()
      } catch {
        setConnection('reconnecting')
      }
    })
    return () => {
      clearTimeout(timer)
      unsubscribe()
    }
  }, [expire, username])
  const samples = useCallback(
    (key: string) => history.current.get(key) || emptySamples,
    [],
  )
  return {
    snapshot,
    stale,
    connection,
    samples,
  }
}
