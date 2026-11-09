# iBlog

[简体中文](README.md) · **English**

iBlog is a personal blog built to stay simple, stable, and easy to own. It is designed to keep the focus on writing while keeping content and data under your control.

The project uses Go, Vue 3, and SQLite, and runs as a single Docker container. A personal blog does not need microservices or a large infrastructure stack: one container and one persistent data volume are enough.

## What it provides

iBlog supports publishing articles and drafts, Markdown editing and preview, search and tag filtering, friend links, an about page, and site settings management.

Content is stored in SQLite. The admin console and public site are deployed together, with minimal external dependencies. Backups and restore verification are treated as part of normal operation.

## Deployment

Docker is the currently supported deployment method. Install Docker Engine and Docker Compose, obtain `compose.yaml` from the repository, and run it from its directory:

```sh
docker compose pull
docker compose up -d
```

Compose defaults to `latest`, which pulls the most recently published image. To pin a version, change `:latest` in the `image` field to a release tag such as `:v1.1.4`. Upgrades and rollbacks then require changing that one line. There is no separate root `.env` version variable.

The container binds to host `127.0.0.1:11190` by default; the admin console is available at `/admin/`. Before first production startup, set `BLOG_PUBLIC_ORIGIN=https://your-domain`, keep `BLOG_COOKIE_SECURE=true`, and serve through an HTTPS reverse proxy. Compose's `http://127.0.0.1:11190` default is only for local preview. Do not expose an unencrypted port to untrusted networks.

### Public address and search engines

`robots.txt` and `sitemap.xml` need a public address, resolved in this order:

1. the admin console's Settings → Site URL;
2. `BLOG_PUBLIC_URL`;
3. `BLOG_PUBLIC_ORIGIN`;
4. the request `Host`, using `X-Forwarded-Proto` / `X-Forwarded-Host` when `BLOG_TRUST_PROXY=true`;
5. the `BLOG_ADDR` port, as a local-only fallback.

The admin value has the highest priority and applies immediately without a restart. `BLOG_PUBLIC_ORIGIN` still validates the origin of write requests. Compose binds to loopback only and sets `BLOG_TRUST_PROXY=true`, so the local reverse proxy must forward `Host`, `X-Forwarded-Proto` and `X-Forwarded-Host`. If HTTPS is enabled while the resolved address is still `127.0.0.1` or `localhost`, the startup log warns about it.

### v1.1.2 SEO and server-side rendering

The public home, article list, article detail, about and friends pages are server-rendered by Go. Article pages include canonical, Open Graph, BlogPosting and BreadcrumbList JSON-LD metadata; drafts, deleted articles and unknown public paths return HTML 404 responses, while old slugs return 301 redirects. Search pages use `noindex,follow`, and RSS is available at `/feed.xml`. See [`docs/seo-verification.md`](docs/seo-verification.md) for raw HTML, Cloudflare and Baiduspider verification. Cover images, uploads, processing and WebP/AVIF remain deferred.

## Data and backups

Articles, accounts, site settings, the database, and the session signing key are stored in the Docker volume `iblog_data`. Updating or rebuilding the container does not remove this volume; **do not run `docker compose down -v`** or delete `iblog_data` manually.

Back up before upgrades. After cloning the repository, run the backup script (Docker and the Alpine image are required):

```sh
BLOG_BACKUP_DIR="$HOME/blog-backups" sh scripts/backup.sh
```

Regularly verify restores and keep backups outside the server. Backup files contain blog data and keys, so handle them as sensitive data.

Restore verification starts a temporary container using the `latest` image by default. Pass an image as the second argument to test a specific version, for example `sh scripts/verify-backup.sh backup.tar.gz ghcr.io/charlesfield19/iblog:v1.1.1`. This tests whether that application image can read the backup; it does not inspect an application version stored in the archive.

### v1.1.0 search and Markdown

v1.1.0 automatically migrates the SQLite search schema on startup. FTS5 ranks title, summary, body, and tag matches while retaining pagination. The Go backend converts Markdown to sanitized HTML; raw HTML, scripts, event attributes, and dangerous URL schemes are removed.

Both frontends use strict TypeScript and Tailwind CSS v4, with shared light/dark tokens and Markdown typography and no legacy CSS layer. Editor previews and public articles use the same backend renderer. `GET /api/v1/health` also reports the application version.

Backups briefly stop the container for consistency and restart it if it was previously running. Override `BLOG_DATA_VOLUME`, `CONTAINER_NAME`, and `BLOG_BACKUP_DIR` to select the target (defaults: `iblog_data`, `iblog`, and `$HOME/blog-backups`). Restore replaces all data in that volume. Save a current backup and confirm the target before running:

```sh
sh scripts/verify-backup.sh /absolute/path/backup.tar.gz
CONFIRM_RESTORE=yes sh scripts/restore.sh /absolute/path/backup.tar.gz
curl -fsS http://127.0.0.1:11190/api/v1/health
```

Verification uses the local `latest` image, pulling it if absent. To test a newly published version, pull that image first and pass it as the second argument. Restore validates archive paths, entry types, and the database file before touching the target volume; only restore trusted backups. Check login, articles, search, and settings afterward.

There is currently no standalone host-install package or systemd unit. Running on a host requires building the Go backend and frontend assets and configuring the application's environment through a service manager. The application does not require or automatically load a `.env` file.

## Upgrade and rollback

With the default `latest` tag, pull and recreate the container to upgrade:

```sh
docker compose pull
docker compose up -d
```

For a pinned deployment, change the image tag in `compose.yaml` and run the same commands. Roll back by restoring the previous tag; do not rely on `latest` for rollback. Keep a backup before upgrading. If a release changes the database schema, confirm that the older version can read the current data before rolling back; restore from backup if necessary.

## License

See the repository for the applicable license and project details.
