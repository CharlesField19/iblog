import { spawnSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'

const image = process.argv[2] || 'iblog:local'
const suffix = randomUUID().slice(0, 8)
const containers = []
const volumes = []
const username = 'e2e-admin'
const password = 'e2e-password-123'
const slug = `e2e-${suffix}`

function docker(args, { input, binary = false } = {}) {
  const result = spawnSync('docker', args, { input, encoding: binary ? null : 'utf8', maxBuffer: 64 * 1024 * 1024 })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`docker ${args.join(' ')} failed: ${binary ? result.stderr?.toString() : result.stderr}`)
  return result.stdout
}

function newVolume(name) {
  docker(['volume', 'create', name])
  volumes.push(name)
  return name
}

async function startApp(name, volume) {
  docker(['run', '-d', '--name', name, '-p', '127.0.0.1::11190', '-e', 'BLOG_ADDR=:11190', '-e', 'BLOG_COOKIE_SECURE=false', '-v', `${volume}:/data`, image])
  containers.push(name)
  const mapped = docker(['port', name, '11190/tcp']).trim().split(/\r?\n/)[0]
  const port = mapped.slice(mapped.lastIndexOf(':') + 1)
  const base = `http://127.0.0.1:${port}`
  for (let attempt = 0; attempt < 30; attempt++) {
    try {
      const health = await fetch(`${base}/api/v1/health`)
      if (health.ok && (await health.json()).status === 'ok') return base
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 1000))
  }
  throw new Error(`container did not become healthy: ${name}`)
}

async function api(base, path, { method = 'GET', token, body, expected } = {}) {
  const response = await fetch(`${base}${path}`, {
    method,
    headers: {
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  })
  const payload = response.status === 204 ? null : await response.json().catch(() => null)
  if (expected !== undefined && response.status !== expected) throw new Error(`${method} ${path}: expected ${expected}, got ${response.status}: ${JSON.stringify(payload)}`)
  if (expected === undefined && !response.ok) throw new Error(`${method} ${path}: ${response.status}: ${JSON.stringify(payload)}`)
  return { status: response.status, payload }
}

async function login(base) {
  const status = await api(base, '/api/v1/auth/setup-status')
  const result = status.payload.setupRequired
    ? await api(base, '/api/v1/auth/setup', { method: 'POST', body: { username, password }, expected: 201 })
    : await api(base, '/api/v1/auth/login', { method: 'POST', body: { username, password }, expected: 200 })
  return result.payload.token
}

try {
  const dataVolume = newVolume(`iblog-e2e-data-${suffix}`)
  let base = await startApp(`iblog-e2e-${suffix}`, dataVolume)
  let token = await login(base)
  const created = await api(base, '/api/v1/admin/articles', {
    method: 'POST', token, expected: 201,
    body: { title: 'E2E draft', slug, summary: 'searchable e2e', contentMd: 'E2E body', tags: ['E2E'], status: 'draft' },
  })
  const id = created.payload.id
  await api(base, `/api/v1/articles/${slug}`, { expected: 404 })
  const hidden = await api(base, '/api/v1/articles?q=searchable&tag=E2E')
  if (hidden.payload.total !== 0) throw new Error('draft appeared in public filtered results')

  await api(base, `/api/v1/admin/articles/${id}`, {
    method: 'PUT', token, expected: 200,
    body: { title: 'E2E published', slug, summary: 'searchable e2e', contentMd: 'E2E body', tags: ['E2E'], status: 'published' },
  })
  const publicArticle = await api(base, `/api/v1/articles/${slug}`)
  if (publicArticle.payload.title !== 'E2E published') throw new Error('published article is not publicly readable')
  const filtered = await api(base, '/api/v1/articles?q=searchable&tag=E2E&page=1&pageSize=1')
  if (filtered.payload.total !== 1 || filtered.payload.items.length !== 1) throw new Error('search/filter/count contract failed')

  docker(['stop', `iblog-e2e-${suffix}`])
  const archive = docker(['run', '--rm', '-v', `${dataVolume}:/data:ro`, 'alpine:3.20', 'tar', '-czf', '-', '-C', '/', 'data'], { binary: true })
  if (!archive.length) throw new Error('backup archive is empty')
  const restoredVolume = newVolume(`iblog-e2e-restored-${suffix}`)
  docker(['run', '--rm', '-i', '-v', `${restoredVolume}:/data`, 'alpine:3.20', 'tar', '-xzf', '-', '-C', '/'], { input: archive })

  base = await startApp(`iblog-e2e-restored-${suffix}`, restoredVolume)
  token = await login(base)
  const restoredArticle = await api(base, `/api/v1/articles/${slug}`)
  if (restoredArticle.payload.title !== 'E2E published') throw new Error('published article missing after backup restore')
  const restoredFilter = await api(base, '/api/v1/articles?q=searchable&tag=E2E')
  if (restoredFilter.payload.total !== 1) throw new Error('search/filter data missing after backup restore')
  await api(base, `/api/v1/admin/articles/${id}`, { method: 'DELETE', token, expected: 204 })
  await api(base, `/api/v1/articles/${slug}`, { expected: 404 })
  console.log('Docker E2E passed: create, draft, publish, public access, search/filter, backup, restore and delete.')
} finally {
  for (const name of containers) docker(['rm', '-f', name])
  for (const name of volumes) docker(['volume', 'rm', '-f', name])
}
