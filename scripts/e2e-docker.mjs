import { spawnSync } from 'node:child_process'
import { createHash, randomUUID } from 'node:crypto'
import { fileURLToPath } from 'node:url'

const image = process.argv[2] || 'iblog:local'
const suffix = randomUUID().slice(0, 8)
const numericSuffix = Number.parseInt(createHash('sha256').update(suffix).digest('hex').slice(0, 7), 16)
const containers = []
const volumes = []
const username = 'e2e-admin'
const password = 'e2e-password-123'
const slug = `e2e-${suffix}`

function docker(args, { input, binary = false } = {}) {
  const result = spawnSync('docker', args, { input, encoding: binary ? null : 'utf8', maxBuffer: 64 * 1024 * 1024, timeout: 120_000 })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`docker ${args.join(' ')} failed: ${binary ? result.stderr?.toString() : result.stderr}`)
  return result.stdout
}

function newVolume(name) {
  docker(['volume', 'create', name])
  volumes.push(name)
  return name
}

function appBase(name) {
  const mapped = docker(['port', name, '11190/tcp']).trim().split(/\r?\n/)[0]
  const port = mapped.slice(mapped.lastIndexOf(':') + 1)
  return `http://127.0.0.1:${port}`
}

async function startApp(name, volume) {
  docker(['run', '-d', '--name', name, '-p', '127.0.0.1::11190', '-e', 'BLOG_ADDR=:11190', '-e', 'BLOG_COOKIE_SECURE=false', '-v', `${volume}:/data`, image])
  containers.push(name)
  const base = appBase(name)
  for (let attempt = 0; attempt < 30; attempt++) {
    try {
      const health = await fetch(`${base}/api/v1/health`, { headers: { Connection: 'close' } })
      if (health.ok && (await health.json()).status === 'ok') return base
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 1000))
  }
  throw new Error(`container did not become healthy: ${name}`)
}

async function api(base, path, { method = 'GET', cookie, body, expected } = {}) {
  const response = await fetch(`${base}${path}`, {
    method,
    headers: {
      Connection: 'close',
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      ...(cookie ? { Cookie: cookie } : {}),
      ...(method !== 'GET' ? { Origin: base } : {}),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  })
  const payload = response.status === 204 ? null : await response.json().catch(() => null)
  if (expected !== undefined && response.status !== expected) throw new Error(`${method} ${path}: expected ${expected}, got ${response.status}: ${JSON.stringify(payload)}`)
  if (expected === undefined && !response.ok) throw new Error(`${method} ${path}: ${response.status}: ${JSON.stringify(payload)}`)
  return { status: response.status, payload, headers: response.headers }
}

async function page(base, path, expected = 200) {
  const response = await fetch(`${base}${path}`, { headers: { Connection: 'close' }, redirect: 'manual' })
  const body = await response.text()
  if (response.status !== expected) throw new Error(`GET ${path}: expected ${expected}, got ${response.status}`)
  return { response, body }
}

async function setupOrLogin(base) {
  const status = await api(base, '/api/v1/auth/setup-status')
  const path = status.payload.setupRequired ? '/api/v1/auth/setup' : '/api/v1/auth/login'
  const result = await api(base, path, { method: 'POST', body: { username, password }, expected: status.payload.setupRequired ? 201 : 200 })
  const cookie = result.headers.get('set-cookie')?.match(/(?:^|,\s*)(iblog_session=[^;,]+)/)?.[1]
  if (!cookie) throw new Error('authentication response did not set the HttpOnly session cookie')
  return cookie
}

try {
  const dataVolume = newVolume(`iblog-e2e-data-${suffix}`)
  let base = await startApp(`iblog-e2e-${suffix}`, dataVolume)
  const imageVersion = docker(['image', 'inspect', '--format', '{{index .Config.Labels "org.opencontainers.image.version"}}', image]).trim()
  if (!imageVersion) throw new Error(`image ${image} does not declare an application version`)
  const health = await api(base, '/api/v1/health')
  if (health.payload.version !== imageVersion) throw new Error(`health endpoint reports ${health.payload.version}, image version is ${imageVersion}`)
  let cookie = await setupOrLogin(base)
  const created = await api(base, '/api/v1/admin/articles', {
    method: 'POST', cookie, expected: 201,
    body: { title: 'E2E draft', slug, summary: 'searchable e2e', contentMd: 'E2E body', tags: ['E2E'], status: 'draft' },
  })
  const id = created.payload.id
  await api(base, `/api/v1/articles/${slug}`, { expected: 404 })
  const draftPage = await page(base, `/articles/${slug}`, 404)
  if (draftPage.body.includes('E2E draft') || !draftPage.body.includes('noindex')) throw new Error('draft leaked into SSR HTML')
  const hidden = await api(base, '/api/v1/articles?q=searchable&tag=E2E')
  if (hidden.payload.total !== 0) throw new Error('draft appeared in public filtered results')

  await api(base, `/api/v1/admin/articles/${id}`, {
    method: 'PUT', cookie, expected: 200,
    body: { title: 'E2E published', slug, summary: 'searchable e2e', contentMd: 'E2E body', tags: ['E2E'], status: 'published' },
  })
  const publicArticle = await api(base, `/api/v1/articles/${slug}`)
  if (publicArticle.payload.title !== 'E2E published') throw new Error('published article is not publicly readable')
  const rawArticle = await page(base, `/articles/${slug}`)
  for (const marker of ['<title>E2E published · iBlog</title>', 'E2E body', 'application/ld+json', 'BlogPosting', '__IBLOG_INITIAL__']) {
    if (!rawArticle.body.includes(marker)) throw new Error(`SSR article HTML is missing ${marker}`)
  }
  const rawDraft = await page(base, `/articles/${slug}-draft-missing`, 404)
  if (!rawDraft.body.includes('noindex')) throw new Error('missing article did not return noindex HTML')
  const rawUnknown = await page(base, '/unknown-e2e-route', 404)
  if (!rawUnknown.body.includes('页面不存在')) throw new Error('unknown public route did not return HTML 404')
  const feed = await page(base, '/feed.xml')
  if (!feed.body.includes('<rss') || !feed.body.includes(`/articles/${slug}`) || feed.body.includes('E2E draft')) {
    throw new Error('RSS feed did not contain published-only content')
  }
  const filtered = await api(base, '/api/v1/articles?q=searchable&tag=E2E&page=1&pageSize=1')
  if (filtered.payload.total !== 1 || filtered.payload.items.length !== 1) throw new Error('search/filter/count contract failed')

  const backupVolume = newVolume(`iblog-e2e-backup-${suffix}`)
  const backupDirectory = docker(['volume', 'inspect', '-f', '{{.Mountpoint}}', backupVolume]).trim()
  const verifierContainer = `iblog-backup-check-${numericSuffix}`
  const verifierVolume = `iblog-backup-check-${numericSuffix}`
  const repository = fileURLToPath(new URL('../', import.meta.url))
  const script = (name, args = [], extraEnv = []) => docker([
    'run', '--rm', '-v', '/var/run/docker.sock:/var/run/docker.sock',
    '-v', `${repository}:/repo:ro`, '-v', `${backupVolume}:${backupDirectory}`,
    '-e', `BLOG_BACKUP_DIR=${backupDirectory}`, '-e', `BLOG_DATA_VOLUME=${dataVolume}`,
    '-e', `CONTAINER_NAME=iblog-e2e-${suffix}`, '-e', `BACKUP_CHECK_CONTAINER=${verifierContainer}`,
    '-e', `BACKUP_CHECK_VOLUME=${verifierVolume}`,
    ...extraEnv,
    '--entrypoint', 'sh', 'docker:29-cli', `/repo/scripts/${name}`, ...args,
  ])
  script('backup-volume.sh')
  const backupName = docker(['run', '--rm', '-v', `${backupVolume}:/archive`, 'alpine:3.20', 'sh', '-c', 'ls /archive/*.tar.gz']).trim().split('/').pop()
  if (!backupName) throw new Error('backup-volume.sh did not create an archive')
  const permissions = docker(['run', '--rm', '-v', `${backupVolume}:/archive`, 'alpine:3.20', 'stat', '-c', '%a', `/archive/${backupName}`]).trim()
  if (permissions !== '600') throw new Error(`backup archive permissions are ${permissions}, expected 600`)
  const quotedName = "qa ' backup.tar.gz"
  docker(['run', '--rm', '-v', `${backupVolume}:/archive`, 'alpine:3.20', 'mv', `/archive/${backupName}`, `/archive/${quotedName}`])
  const backupPath = `${backupDirectory}/${quotedName}`
  script('verify-backup.sh', [backupPath, image])
  let refused = false
  try { script('restore.sh', [backupPath]) } catch { refused = true }
  if (!refused) throw new Error('restore.sh did not require explicit confirmation')
  docker(['run', '--rm', '-v', `${backupVolume}:/archive`, 'alpine:3.20', 'sh', '-eu', '-c', 'mkdir -p /invalid/other; echo invalid > /invalid/other/file; tar -czf /archive/invalid.tar.gz -C /invalid other'])
  refused = false
  try { script('restore.sh', [`${backupDirectory}/invalid.tar.gz`], ['-e', 'CONFIRM_RESTORE=yes']) } catch { refused = true }
  if (!refused) throw new Error('restore.sh accepted an archive outside data/')
  base = appBase(`iblog-e2e-${suffix}`)
  for (let attempt = 0; attempt < 30; attempt++) {
    try { if ((await fetch(`${base}/api/v1/health`, { headers: { Connection: 'close' } })).ok) break } catch {}
    await new Promise((resolve) => setTimeout(resolve, 1000))
  }
  await api(base, `/api/v1/admin/articles/${id}`, {
    method: 'PUT', cookie,
    body: { title: 'Changed after backup', slug, contentMd: 'changed', status: 'published', tags: ['E2E'] },
  })
  script('restore.sh', [backupPath], ['-e', 'CONFIRM_RESTORE=yes'])
  base = appBase(`iblog-e2e-${suffix}`)
  for (let attempt = 0; attempt < 30; attempt++) {
    try { if ((await fetch(`${base}/api/v1/health`, { headers: { Connection: 'close' } })).ok) break } catch {}
    await new Promise((resolve) => setTimeout(resolve, 1000))
  }
  const scriptRestored = await api(base, `/api/v1/articles/${slug}`)
  if (scriptRestored.payload.title !== 'E2E published') throw new Error('restore.sh did not restore the original article')
  if (docker(['inspect', '-f', '{{.State.Running}}', `iblog-e2e-${suffix}`]).trim() !== 'true') throw new Error('backup/restore scripts did not restart the source container')

  docker(['stop', `iblog-e2e-${suffix}`])
  const archive = docker(['run', '--rm', '-v', `${dataVolume}:/data:ro`, 'alpine:3.20', 'tar', '-czf', '-', '-C', '/', 'data'], { binary: true })
  if (!archive.length) throw new Error('backup archive is empty')
  const restoredVolume = newVolume(`iblog-e2e-restored-${suffix}`)
  docker(['run', '--rm', '-i', '-v', `${restoredVolume}:/data`, 'alpine:3.20', 'tar', '-xzf', '-', '-C', '/'], { input: archive })

  base = await startApp(`iblog-e2e-restored-${suffix}`, restoredVolume)
  cookie = await setupOrLogin(base)
  const restoredArticle = await api(base, `/api/v1/articles/${slug}`)
  if (restoredArticle.payload.title !== 'E2E published') throw new Error('published article missing after backup restore')
  const restoredFilter = await api(base, '/api/v1/articles?q=searchable&tag=E2E')
  if (restoredFilter.payload.total !== 1) throw new Error('search/filter data missing after backup restore')
  await api(base, `/api/v1/admin/articles/${id}`, { method: 'DELETE', cookie, expected: 204 })
  await api(base, `/api/v1/articles/${slug}`, { expected: 404 })
  console.log('Docker E2E passed: create, draft, publish, public access, search/filter, real backup-volume/verify-backup/restore scripts, archive permissions/quoted filenames, volume restore and delete.')
} finally {
  for (const name of containers) docker(['rm', '-f', name])
  for (const name of volumes) docker(['volume', 'rm', '-f', name])
}
