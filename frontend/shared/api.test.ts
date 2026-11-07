import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { ApiError, request } from './api'

const originalFetch = globalThis.fetch
after(() => {
  globalThis.fetch = originalFetch
})

test('typed JSON and empty responses use the declared response contract', async () => {
  globalThis.fetch = async () => Response.json({ html: '<p>preview</p>' })
  const result = await request<{ html: string }>('/api/v1/admin/markdown/preview', {
    method: 'POST',
    body: { contentMd: 'preview' },
  })
  assert.equal(result.html, '<p>preview</p>')
  globalThis.fetch = async () => new Response(null, { status: 204 })
  assert.equal(await request<void>('/empty'), undefined)
})

test('session requests use same-origin HttpOnly cookies without bearer tokens', async () => {
  globalThis.fetch = async (_path, options) => {
    assert.equal(options?.credentials, 'same-origin')
    assert.equal(new Headers(options?.headers).has('Authorization'), false)
    return Response.json({ expiresAt: '2030-01-01T00:00:00Z' })
  }
  await request('/api/v1/auth/login', {
    method: 'POST',
    body: { username: 'admin', password: 'test' },
  })
})

test('API, network and caller cancellation errors are distinguished', async () => {
  globalThis.fetch = async () => Response.json({ error: { code: 'unauthorized' } }, { status: 401 })
  await assert.rejects(
    request('/private'),
    (error: unknown) =>
      error instanceof ApiError && error.status === 401 && error.code === 'unauthorized',
  )
  globalThis.fetch = async () => {
    throw new TypeError('offline')
  }
  await assert.rejects(
    request('/offline'),
    (error: unknown) => error instanceof ApiError && error.code === 'network',
  )
  const controller = new AbortController()
  controller.abort()
  globalThis.fetch = async (_path, options) => {
    assert.equal(options?.signal?.aborted, true)
    throw new DOMException('cancelled', 'AbortError')
  }
  await assert.rejects(
    request('/cancelled', { signal: controller.signal }),
    (error: unknown) => error instanceof ApiError && error.code === 'aborted',
  )
})
