export class ApiError extends Error {
  constructor(status, code, message) {
    super(message)
    this.status = status
    this.code = code
  }
}

export async function request(path, { method = 'GET', body, token, signal } = {}) {
  let response
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 15000)
  const abortFromCaller = () => controller.abort(signal?.reason)
  if (signal?.aborted) abortFromCaller()
  else signal?.addEventListener('abort', abortFromCaller, { once: true })
  try {
    response = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
        ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      signal: controller.signal,
    })
  } catch (error) {
    const cancelled = signal?.aborted
    throw new ApiError(
      0,
      cancelled ? 'aborted' : error?.name === 'AbortError' ? 'timeout' : 'network',
      cancelled
        ? '请求已取消'
        : error?.name === 'AbortError'
          ? '请求超时，请稍后重试'
          : '网络请求失败，请检查网络连接',
    )
  } finally {
    clearTimeout(timeout)
    signal?.removeEventListener('abort', abortFromCaller)
  }
  if (response.status === 204) return null
  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    throw new ApiError(
      response.status,
      payload?.error?.code || String(response.status),
      payload?.error?.message || '请求失败',
    )
  }
  return payload
}
