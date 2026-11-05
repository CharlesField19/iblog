const tokenKey = 'blog_admin_token'
const expiryKey = 'blog_admin_expires_at'

export function token() { return localStorage.getItem(tokenKey) }
export function hasSession() {
  const expires = localStorage.getItem(expiryKey)
  return Boolean(token() && expires && Date.parse(expires) > Date.now())
}
export function saveSession(response) {
  localStorage.setItem(tokenKey, response.token)
  localStorage.setItem(expiryKey, response.expiresAt)
}
export function clearSession() {
  localStorage.removeItem(tokenKey)
  localStorage.removeItem(expiryKey)
}
