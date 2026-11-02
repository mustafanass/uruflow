import { useCallback, useEffect, useRef, useState } from 'react'

import { api } from '@/shared/api'
import { errorText } from '@/shared/utils'

export function useLoad<T>(url: string, enabled = true) {
  const previousURL = useRef('')
  const [data, setData] = useState<T>()
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(enabled)
  const [version, setVersion] = useState(0)
  useEffect(() => {
    if (!enabled) {
      return
    }
    const controller = new AbortController()
    setLoading(true)
    setError('')
    if (previousURL.current !== url) {
      setData(undefined)
      previousURL.current = url
    }
    api<T>(url, { signal: controller.signal })
      .then((value) => {
        if (!controller.signal.aborted) {
          setData(value)
        }
      })
      .catch((error) => {
        if (!controller.signal.aborted) {
          setError(errorText(error))
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false)
        }
      })
    return () => controller.abort()
  }, [url, enabled, version])
  const reload = useCallback(() => setVersion((v) => v + 1), [])
  return { data, error, loading, reload, setData }
}
