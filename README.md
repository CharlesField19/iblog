# iBlog

**语言 / Language:** 简体中文 · [English](README.en.md)

iBlog 是我为自己搭建的博客。比起追求复杂的平台能力，我更希望它简单、稳定，能让我专注写作，也能让我清楚地掌握自己的内容和数据。

项目使用 Go、Vue 3 和 SQLite，并以一个 Docker 容器运行。这个选择是有意的：个人博客不需要微服务，也不应该为了部署它先维护一整套基础设施。一个容器、一份持久化数据，就能把博客运行起来。

## 我想解决的问题

写文章、管理标签和页面这些事情应该足够直接。iBlog 提供文章发布与草稿、Markdown 编辑和预览、搜索与标签筛选，以及友情链接、关于页面和站点设置管理。

内容保存在 SQLite 中，管理入口与公开站点一起部署。系统尽量减少外部依赖，并把备份和恢复作为日常使用的一部分，而不是等出问题后才考虑的补丁。

## 部署

Docker 是当前提供的部署方式。服务器安装 Docker Engine 和 Docker Compose 后，准备 `compose.yaml`（可从仓库下载），在该文件所在目录启动：

```sh
docker compose pull
docker compose up -d
```

Compose 默认使用 `latest`，拉取并启动时会使用最近发布的镜像。希望固定版本时，只需把 `compose.yaml` 中 `image` 的 `:latest` 改成对应发布 tag，例如 `:v1.1.3`；升级或回滚时修改同一处即可。仓库根目录没有额外的 `.env` 版本变量。

站点容器默认只绑定宿主机 `127.0.0.1:11190`，管理入口位于 `/admin/`。首次启动前，生产环境请设置 `BLOG_PUBLIC_ORIGIN=https://你的域名`、保留 `BLOG_COOKIE_SECURE=true`，并由 HTTPS 反向代理提供访问。Compose 默认值 `http://127.0.0.1:11190` 仅适合本机预览；不要直接开放明文端口给不可信网络。

### 站点地址与搜索引擎

`robots.txt` 和 `sitemap.xml` 需要一个公开地址，按以下顺序解析：

1. 管理后台「站点设置 → 站点地址」；
2. `BLOG_PUBLIC_URL`；
3. `BLOG_PUBLIC_ORIGIN`；
4. 请求的 `Host`；设置 `BLOG_TRUST_PROXY=true` 时改用 `X-Forwarded-Proto` / `X-Forwarded-Host`；
5. `BLOG_ADDR` 的端口，仅作本地兜底。

后台填写的值优先级最高，保存后立即生效，无需重启。`BLOG_PUBLIC_ORIGIN` 仍用于校验写请求来源；Compose 默认只绑定本机并设置 `BLOG_TRUST_PROXY=true`，由反向代理转发 `Host`、`X-Forwarded-Proto` 和 `X-Forwarded-Host`。如果使用 HTTPS 而解析结果仍是 `127.0.0.1` 或 `localhost`，启动日志会给出告警。

### v1.1.2 SEO 与服务端渲染

公开首页、文章列表、文章详情、关于和友链页面由 Go 服务端渲染。文章页包含 canonical、Open Graph、BlogPosting 和 BreadcrumbList JSON-LD；草稿、已删除文章和未知公开路径返回 HTML 404，旧 slug 返回 301。搜索页使用 `noindex,follow`，RSS 位于 `/feed.xml`。原始 HTML、Cloudflare 与 Baiduspider 验证步骤见 [`docs/seo-verification.md`](docs/seo-verification.md)。图片封面、上传、处理和 WebP/AVIF 仍暂缓。

## 数据与备份

文章、账号、站点设置、数据库和会话签名密钥保存在 Docker 数据卷 `iblog_data` 中。更新或重建容器不会删除这个数据卷；**不要使用 `docker compose down -v`**，也不要手动删除 `iblog_data`。

升级之前先备份。克隆仓库后可运行备份脚本（脚本需要 Docker 和 Alpine 镜像）：

```sh
BLOG_BACKUP_DIR="$HOME/blog-backups" sh scripts/backup.sh
```

备份也应该定期做恢复验证，并保存到服务器之外。备份文件里包含博客数据和密钥，请按敏感数据妥善保管。

恢复验证会用 `latest` 镜像启动临时容器；也可将目标镜像作为第二个参数传入，例如 `sh scripts/verify-backup.sh backup.tar.gz ghcr.io/charlesfield19/iblog:v1.1.1`。这验证的是指定应用镜像能否读取备份，不是检查备份文件中的应用版本。

### v1.1.0 搜索与 Markdown

v1.1.0 会在启动时自动迁移 SQLite 搜索结构，使用 FTS5 对标题、摘要、正文和标签进行加权搜索，并保留分页。文章 Markdown 由 Go 后端统一转换为安全 HTML；原始 HTML、脚本、事件属性和危险链接协议会被过滤。

双端前端使用严格 TypeScript 和 Tailwind CSS v4，共享浅色/深色主题与 Markdown 排版，不再加载旧 CSS 兼容层。编辑器预览与公开文章使用同一个后端渲染器。健康检查 `GET /api/v1/health` 同时返回应用版本。

备份脚本会短暂停止容器以取得一致的数据，再自动恢复其运行状态。可用 `BLOG_DATA_VOLUME`、`CONTAINER_NAME` 和 `BLOG_BACKUP_DIR` 指定目标；默认分别为 `iblog_data`、`iblog` 和 `$HOME/blog-backups`。恢复会覆盖目标卷的全部数据，先保存当前备份并确认目标，再执行：

```sh
sh scripts/verify-backup.sh /absolute/path/backup.tar.gz
CONFIRM_RESTORE=yes sh scripts/restore.sh /absolute/path/backup.tar.gz
curl -fsS http://127.0.0.1:11190/api/v1/health
```

验证脚本默认使用本地 `latest` 镜像（不存在时拉取）；需要验证某个新发布版本时，先拉取目标镜像，再将其作为第二个参数传入。恢复前会检查归档路径、条目类型及数据库文件；仍只应恢复可信来源的备份。恢复后检查登录、文章、搜索及站点设置。

目前没有单独的宿主机安装包或 systemd 服务配置。宿主机运行需要自行构建 Go 后端和前端静态文件，并在服务管理器中配置应用所需环境变量；`.env` 不是应用运行的必需品，也不会被应用自动加载。

## 升级与回滚

使用 `latest` 时，拉取并重建即可升级：

```sh
docker compose pull
docker compose up -d
```

固定版本部署时，把 `compose.yaml` 的镜像 tag 改为目标版本，再执行同样的命令。回滚也改回之前的 tag；不要依赖 `latest` 回滚。升级前保留备份；如果新版本修改了数据库结构，回滚前先确认旧版本能读取当前数据，必要时从备份恢复。
