const expiryKey = 'blog_admin_expires_at'

export function hasSession() {
  const expires = localStorage.getItem(expiryKey)
  return Boolean(expires && Date.parse(expires) > Date.now())
}
export function saveSession(response) {
  localStorage.removeItem('blog_admin_token')
  localStorage.setItem(expiryKey, response.expiresAt)
}
export function clearSession() {
  localStorage.removeItem('blog_admin_token')
  localStorage.removeItem(expiryKey)
}
