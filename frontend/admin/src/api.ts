import { request, ApiError, type RequestOptions } from '../../shared/api'
import { clearSession } from './session'

export async function adminRequest<T = unknown>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  try {
    return await request<T>(path, options)
  } catch (error) {
    if (error instanceof ApiError && error.status === 401 && !path.includes('/api/v1/auth/login')) {
      clearSession()
      if (window.location.pathname !== '/admin/login')
        window.location.assign('/admin/login?expired=1')
    }
    throw error
  }
}
