import { request } from './api'

const CACHE_TTL = 15_000
const CACHE_LIMIT = 100
const cache = new Map<string, { value: unknown; expiresAt: number }>()
const pending = new Map<string, Promise<unknown>>()

function cacheValue(path: string, value: unknown, ttl: number): void {
  const expiresAt = Date.now() + ttl
  for (const [key, cached] of cache) {
    if (cached.expiresAt <= Date.now()) cache.delete(key)
  }
  const oldest = cache.keys().next().value
  if (cache.size >= CACHE_LIMIT && oldest !== undefined) cache.delete(oldest)
  cache.set(path, { value, expiresAt })
}

export function getPublicData<T = unknown>(
  path: string,
  { force = false, ttl = CACHE_TTL }: { force?: boolean; ttl?: number } = {},
): Promise<T> {
  const cached = cache.get(path)
  if (!force && cached && cached.expiresAt > Date.now()) return Promise.resolve(cached.value as T)
  const inFlight = pending.get(path)
  if (!force && inFlight) return inFlight as Promise<T>

  const promise = request<T>(path)
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

export function clearPublicDataCache(path?: string): void {
  if (path) cache.delete(path)
  else cache.clear()
}

export function hasPublicData(path: string): boolean {
  const cached = cache.get(path)
  return Boolean(cached && cached.expiresAt > Date.now())
}
