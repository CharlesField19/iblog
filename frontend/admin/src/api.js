import { request, ApiError } from '../../shared/api.js'
import { token, clearSession } from './session.js'

export async function adminRequest(path, options = {}) {
  try {
    return await request(path, { ...options, token: token() })
  } catch (error) {
    if (error instanceof ApiError && error.status === 401 && !path.includes('/api/v1/auth/login')) {
      clearSession()
      if (window.location.pathname !== '/admin/login') window.location.assign('/admin/login?expired=1')
    }
    throw error
  }
}
