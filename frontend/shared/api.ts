export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string) {
    super(code)
    this.status = status
    this.code = code
  }
}

export interface RequestOptions {
  method?: string
  body?: unknown
  signal?: AbortSignal
}

export async function request<T = unknown>(
  path: string,
  { method = 'GET', body, signal }: RequestOptions = {},
): Promise<T> {
  let response
  const isFormData = typeof FormData !== 'undefined' && body instanceof FormData
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
        ...(body === undefined || isFormData ? {} : { 'Content-Type': 'application/json' }),
      },
      ...(body === undefined ? {} : { body: isFormData ? body : JSON.stringify(body) }),
      signal: controller.signal,
    })
  } catch (error) {
    const cancelled = signal?.aborted
    const fetchError = error as Error
    throw new ApiError(
      0,
      cancelled ? 'aborted' : fetchError?.name === 'AbortError' ? 'timeout' : 'network',
    )
  } finally {
    clearTimeout(timeout)
    signal?.removeEventListener('abort', abortFromCaller)
  }
  if (response.status === 204) return undefined as T
  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    throw new ApiError(response.status, payload?.error?.code || String(response.status))
  }
  return payload as T
}
