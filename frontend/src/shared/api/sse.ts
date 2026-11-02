export interface StreamEvent {
  event: string
  data: string
  id: string
}

export async function readEvents(
  body: ReadableStream<Uint8Array>,
  receive: (event: StreamEvent) => void,
  signal: AbortSignal,
) {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let event = 'message'
  let id = ''
  let data: string[] = []
  let size = 0
  const cancel = () => void reader.cancel().catch(() => {})
  signal.addEventListener('abort', cancel, { once: true })
  try {
    while (!signal.aborted) {
      const chunk = await reader.read()
      if (chunk.done) return
      buffer += decoder.decode(chunk.value, { stream: true })
      if (buffer.length + size > 8 << 20)
        throw new Error('Log event exceeds the size limit.')
      let end: number
      while ((end = buffer.indexOf('\n')) >= 0) {
        const line = buffer.slice(0, end).replace(/\r$/, '')
        buffer = buffer.slice(end + 1)
        if (!line) {
          if (data.length) receive({ event, data: data.join('\n'), id })
          event = 'message'
          id = ''
          data = []
          size = 0
        } else if (!line.startsWith(':')) {
          const split = line.indexOf(':')
          const field = split < 0 ? line : line.slice(0, split)
          const value = split < 0 ? '' : line.slice(split + 1).replace(/^ /, '')
          if (field === 'event') event = value
          if (field === 'data') {
            data.push(value)
            size += value.length
            if (size > 8 << 20)
              throw new Error('Log event exceeds the size limit.')
          }
          if (field === 'id' && !value.includes('\0')) id = value
        }
        if (signal.aborted) return
      }
    }
  } finally {
    signal.removeEventListener('abort', cancel)
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}
