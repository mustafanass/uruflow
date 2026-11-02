import assert from 'node:assert/strict'
import { test } from 'node:test'
import { ResourceStore } from '../src/features/infrastructure/resource-store.ts'

test('metric changes notify only subscribers for the changed resource', () => {
  const store = new ResourceStore()
  store.replace(
    new Map([
      ['a', { state: 'online', cpu: 0 }],
      ['b', { state: 'online', cpu: 5 }],
    ]),
  )
  const before = store.get('b')
  let a = 0,
    b = 0
  const remove = store.subscribe('a', () => a++)
  store.subscribe('b', () => b++)
  store.replace(
    new Map([
      ['a', { state: 'online', cpu: 10 }],
      ['b', { state: 'online', cpu: 5 }],
    ]),
  )
  assert.equal(a, 1)
  assert.equal(b, 0)
  assert.equal(store.get('b'), before)
  remove()
  store.replace(
    new Map([
      ['a', { state: 'offline', cpu: null }],
      ['b', { state: 'online', cpu: 5 }],
    ]),
  )
  assert.equal(a, 1)
})

test('missing resources and lost observations are distinct from zero usage', () => {
  const store = new ResourceStore()
  store.replace(
    new Map([['container', { state: 'running', cpu: 0, memory: 0 }]]),
  )
  assert.equal(store.get('container')?.cpu, 0)
  let changes = 0
  store.subscribe('container', () => changes++)
  store.replace(
    new Map([['container', { state: 'unknown', cpu: null, memory: null }]]),
  )
  assert.equal(changes, 1)
  assert.equal(store.get('container')?.cpu, null)
  store.replace(new Map())
  assert.equal(changes, 2)
  assert.equal(store.get('container'), null)
})

test('an identical observation preserves identity and does not notify', () => {
  const store = new ResourceStore()
  store.replace(new Map([['agent', { state: 'online', cpu: 0, memory: 256 }]]))
  const before = store.get('agent')
  let changes = 0
  store.subscribe('agent', () => changes++)
  for (let i = 0; i < 10; i++)
    store.replace(
      new Map([['agent', { state: 'online', cpu: 0, memory: 256 }]]),
    )
  assert.equal(store.get('agent'), before)
  assert.equal(changes, 0)
})
