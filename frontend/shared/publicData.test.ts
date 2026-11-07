import assert from 'node:assert/strict'
import { test } from 'node:test'
import { clearPublicDataCache, getPublicData } from './publicData'

test('public GET requests are deduplicated and briefly cached', async () => {
  const originalFetch = globalThis.fetch
  let calls = 0
  globalThis.fetch = async () => {
    calls += 1
    return new Response(JSON.stringify({ items: ['cached'] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  try {
    clearPublicDataCache('/api/v1/friends')
    const [first, concurrent] = await Promise.all([
      getPublicData('/api/v1/friends'),
      getPublicData('/api/v1/friends'),
    ])
    const later = await getPublicData('/api/v1/friends')
    assert.equal(calls, 1)
    assert.deepEqual(first, concurrent)
    assert.deepEqual(later, first)
  } finally {
    globalThis.fetch = originalFetch
    clearPublicDataCache('/api/v1/friends')
  }
})

test('failed public GET requests are not cached', async () => {
  const originalFetch = globalThis.fetch
  let calls = 0
  globalThis.fetch = async () => {
    calls += 1
    return new Response('{}', { status: calls === 1 ? 500 : 200 })
  }

  try {
    clearPublicDataCache('/api/v1/settings?test=retry')
    await assert.rejects(getPublicData('/api/v1/settings?test=retry'))
    await getPublicData('/api/v1/settings?test=retry')
    assert.equal(calls, 2)
  } finally {
    globalThis.fetch = originalFetch
    clearPublicDataCache('/api/v1/settings?test=retry')
  }
})
