export class ApiError extends Error {
  constructor(status, code, message) {
    super(message)
    this.status = status
    this.code = code
  }
}

export async function request(path, { method = 'GET', body, token } = {}) {
  let response
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 15000)
  try {
    response = await fetch(path, {
      method,
      headers: {
        Accept: 'application/json',
        ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      signal: controller.signal,
    })
  } catch (error) {
    throw new ApiError(0, error?.name === 'AbortError' ? 'timeout' : 'network', error?.name === 'AbortError' ? '请求超时，请稍后重试' : '网络请求失败，请检查网络连接')
  } finally {
    clearTimeout(timeout)
  }
  if (response.status === 204) return null
  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    throw new ApiError(response.status, payload?.error?.code || String(response.status), payload?.error?.message || '请求失败')
  }
  return payload
}
