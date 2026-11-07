const expiryKey = 'blog_admin_expires_at'

export function hasSession() {
  const expires = localStorage.getItem(expiryKey)
  return Boolean(expires && Date.parse(expires) > Date.now())
}
export function saveSession(response: SessionResponse) {
  // The authentication credential lives only in the HttpOnly cookie.
  localStorage.removeItem('blog_admin_token')
  localStorage.setItem(expiryKey, response.expiresAt)
}
export function clearSession() {
  localStorage.removeItem('blog_admin_token')
  localStorage.removeItem(expiryKey)
}
import type { SessionResponse } from '../../shared/contracts'
