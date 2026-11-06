import { request } from './api.js'

const CACHE_TTL = 15_000
const CACHE_LIMIT = 100
const cache = new Map()
const pending = new Map()

function cacheValue(path, value, ttl) {
  const expiresAt = Date.now() + ttl
  for (const [key, cached] of cache) {
    if (cached.expiresAt <= Date.now()) cache.delete(key)
  }
  if (cache.size >= CACHE_LIMIT) cache.delete(cache.keys().next().value)
  cache.set(path, { value, expiresAt })
}

export function getPublicData(path, { force = false, ttl = CACHE_TTL } = {}) {
  const cached = cache.get(path)
  if (!force && cached && cached.expiresAt > Date.now()) return Promise.resolve(cached.value)
  if (!force && pending.has(path)) return pending.get(path)

  const promise = request(path)
    .then((value) => {
      cacheValue(path, value, ttl)
      return value
    })
    .finally(() => {
      if (pending.get(path) === promise) pending.delete(path)
    })

  pending.set(path, promise)
  return promise
}

export function clearPublicDataCache(path) {
  if (path) cache.delete(path)
  else cache.clear()
}

export function hasPublicData(path) {
  const cached = cache.get(path)
  return Boolean(cached && cached.expiresAt > Date.now())
}
