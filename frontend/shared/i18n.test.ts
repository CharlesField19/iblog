import assert from 'node:assert/strict'
import { after, test } from 'node:test'

const storage = new Map<string, string>()
Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: {
    get length() {
      return storage.size
    },
    clear: () => storage.clear(),
    key: (index: number) => [...storage.keys()][index] ?? null,
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => storage.set(key, String(value)),
    removeItem: (key: string) => storage.delete(key),
  },
})
Object.defineProperty(globalThis, 'document', { configurable: true, value: undefined })
Object.defineProperty(globalThis, 'navigator', {
  configurable: true,
  value: { language: 'zh-CN' },
})

const { i18n, setLocale, toggleLocale, errorMessage } = await import('./i18n')
Object.defineProperty(globalThis, 'document', {
  configurable: true,
  value: { documentElement: { lang: '' } },
})

test('locale selection is persisted and updates document language', () => {
  setLocale('en-US')
  assert.equal(i18n.global.locale.value, 'en-US')
  assert.equal(localStorage.getItem('iblog-locale'), 'en-US')
  assert.equal(document.documentElement.lang, 'en-US')
  toggleLocale()
  assert.equal(i18n.global.locale.value, 'zh-CN')
})

test('API errors resolve to the active locale with a safe fallback', () => {
  setLocale('en-US')
  assert.equal(
    errorMessage({ code: 'unauthorized' }),
    'Your session has expired. Please sign in again.',
  )
  assert.equal(
    errorMessage({ code: 'unknown_code' }),
    'The server is temporarily unavailable. Please try again later.',
  )
  setLocale('zh-CN')
  assert.equal(errorMessage({ code: 'unauthorized' }), '登录状态已失效，请重新登录')
})

after(() => {
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: undefined })
  Object.defineProperty(globalThis, 'document', { configurable: true, value: undefined })
})
