# iBlog

[简体中文](README.md) · **English**

iBlog is a personal blog built to stay simple, stable, and easy to own. It is designed to keep the focus on writing while keeping content and data under your control.

The project uses Go, Vue 3, and SQLite, and runs as a single Docker container. A personal blog does not need microservices or a large infrastructure stack: one container and one persistent data volume are enough.

## What it provides

iBlog supports publishing articles and drafts, Markdown editing and preview, search and tag filtering, friend links, an about page, and site settings management.

Content is stored in SQLite. The admin console and public site are deployed together, with minimal external dependencies. Backups and restore verification are treated as part of normal operation.

## Deployment

Install Docker Engine and Docker Compose, then start the service with the repository's `compose.yaml`:

```sh
docker compose pull
docker compose up -d
```

The site listens on port `11190` by default, and the admin console is available at `/admin/`. On the first visit, follow the page instructions to create an administrator account. For production, use an HTTPS reverse proxy and set `BLOG_COOKIE_SECURE` to `true`.

## Data and backups

Articles, accounts, site settings, the database, and the session signing key are stored in the Docker volume `iblog_data`. Updating or rebuilding the container does not remove this volume; **do not run `docker compose down -v`** or delete `iblog_data` manually.

Back up before upgrades:

```sh
BLOG_BACKUP_DIR="$HOME/blog-backups" sh scripts/backup.sh
```

Regularly verify restores and keep backups outside the server. Backup files contain blog data and keys, so handle them as sensitive data.

## Upgrade and rollback

After a release, update the image version in `compose.yaml` and run:

```sh
docker compose pull
docker compose up -d
```

To roll back, change the image version back and run the same commands. Keep a backup before upgrading. If a release changes the database schema, confirm that the older version can read the current data before rolling back; restore from backup if necessary.

## License

See the repository for the applicable license and project details.
