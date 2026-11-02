import { useEffect, useMemo, useRef } from 'react'
import { useLoad, useWorkspace } from '@/shared/hooks'
import { useVisible } from '@/shared/hooks/useVisible'
import type { Alert } from '@/shared/types'

export function useAlertHistory() {
  const visible = useVisible()
  const { snapshot } = useWorkspace()
  const request = useLoad<Alert[]>('/api/alerts', visible)
  const ids = (snapshot?.alerts || [])
    .map((alert) => alert.id)
    .sort()
    .join(',')
  const previous = useRef(ids)
  useEffect(() => {
    if (visible && previous.current !== ids) request.reload()
    previous.current = ids
  }, [ids, visible, request.reload])
  useEffect(() => {
    if (!visible) return
    const timer = window.setInterval(request.reload, 15000)
    return () => clearInterval(timer)
  }, [visible, request.reload])
  const rows = useMemo(() => {
    const active = new Map(
      (snapshot?.alerts || []).map((alert) => [alert.id, alert]),
    )
    const history = (request.data || []).map((alert) => {
      const current = active.get(alert.id)
      active.delete(alert.id)
      return alert.resolved ? alert : current || alert
    })
    return [...active.values(), ...history]
  }, [snapshot?.alerts, request.data])
  return { ...request, rows }
}
