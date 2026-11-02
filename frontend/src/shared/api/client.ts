let csrfToken = ''
export function setCSRF(value: string) {
  csrfToken = value
}
export class APIError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}
export async function api<T>(
  url: string,
  options: {
    method?: string
    body?: unknown
    signal?: AbortSignal
  } = {},
): Promise<T> {
  const response = await fetch(url, {
    method: options.method || 'GET',
    credentials: 'same-origin',
    signal: options.signal,
    headers: {
      'Content-Type': 'application/json',
      ...(options.method && options.method !== 'GET'
        ? { 'X-CSRF-Token': csrfToken }
        : {}),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  }).catch((error: unknown) => {
    if (options.signal?.aborted) {
      throw error
    }
    throw new APIError(
      'Cannot reach the server. Check the connection and try again.',
      0,
    )
  })
  if (response.status === 401) {
    window.dispatchEvent(new Event('uruflow:unauthorized'))
  }
  if (response.status === 204) {
    return undefined as T
  }
  const text = await response.text()
  let value
  try {
    value = JSON.parse(text)
  } catch {
    throw new APIError(
      response.ok
        ? 'The server returned an unreadable response. Try again.'
        : 'The server could not complete this request. Try again.',
      response.status,
    )
  }
  if (!response.ok) {
    throw new APIError(value?.error || 'The request failed.', response.status)
  }
  return value as T
}
export const segment = encodeURIComponent
