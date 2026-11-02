import assert from 'node:assert/strict'
import { test } from 'node:test'

import { readEvents, type StreamEvent } from '../src/shared/api/sse.ts'

const encoder = new TextEncoder()

test('decodes split UTF-8, CRLF, comments, multiline data and independent event IDs', async () => {
  const data = encoder.encode(
    ': keepalive\r\nevent: log\r\nid: p1\r\ndata: {"line":"مرحبا"}\r\n\r\nevent: complete\ndata: null\n\n',
  )
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const byte of data) controller.enqueue(Uint8Array.of(byte))
      controller.close()
    },
  })
  const events: StreamEvent[] = []
  await readEvents(
    body,
    (event) => events.push(event),
    new AbortController().signal,
  )
  assert.deepEqual(events, [
    { event: 'log', id: 'p1', data: '{"line":"مرحبا"}' },
    { event: 'complete', id: '', data: 'null' },
  ])
})

test('cancels a live reader when the viewer closes', async () => {
  const controller = new AbortController()
  let cancelled = false
  const body = new ReadableStream<Uint8Array>({
    start(stream) {
      stream.enqueue(encoder.encode('event: log\ndata: {}\n\n'))
    },
    cancel() {
      cancelled = true
    },
  })
  let received = 0
  await readEvents(
    body,
    () => {
      received++
      controller.abort()
    },
    controller.signal,
  )
  assert.equal(received, 1)
  assert.equal(cancelled, true)
})
