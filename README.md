# iBlog

iBlog 是一个 Go + Vue 博客应用。正式部署使用 GitHub Actions 构建并发布 Docker 镜像到 GHCR；服务器只需 Docker Engine 和 Docker Compose，不需要安装 Go、Node.js，也不需要下载源码或在服务器编译。

## 部署前提

- 已发布的镜像：`ghcr.io/charlesfield19/iblog`
- 服务器已安装 Docker Engine 和 Docker Compose 插件。
- GHCR 镜像若设为 Public，服务器无需登录 GHCR；若为 Private，需先在服务器执行 `docker login ghcr.io`。

> 当前版本镜像只有在 GitHub Actions 的 CI 通过并推送版本标签后才会发布。版本标签工作流会运行测试、构建镜像，并推送版本标签和 `latest`。

## 首次部署

将 `compose.yaml` 和 `.env.example` 放入服务器的独立部署目录，然后：

```sh
cp .env.example .env
# 编辑 .env：确认 GHCR_OWNER、IBLOG_VERSION 和 BLOG_PORT
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 blog
```

默认配置使用 `GHCR_OWNER=CharlesField19`、`IBLOG_VERSION=v1.0.0`、`BLOG_PORT=11190`。如需只允许本机或反向代理访问，将端口改为：

```dotenv
BLOG_PORT=127.0.0.1:10001
```

容器内端口固定为 `11190`。设置 `BLOG_PORT=11190` 时，服务会绑定服务器所有网卡；是否能从公网访问还取决于防火墙规则。

启动后访问 `http://服务器地址:端口/admin/`，按页面提示创建管理员用户名和密码。该初始化只在数据库中没有管理员时开放。管理员密码不是 `.env` 配置项。

## 数据持久化与备份

文章、账号、站点设置、SQLite 数据库和自动生成的 JWT 签名密钥保存在 Docker 命名卷 `iblog_data` 中。容器更新或重建时继续使用该卷。

**不要运行 `docker compose down -v`，不要删除 `iblog_data` 卷。** 删除卷会丢失博客数据和密钥。

升级前请先备份。可在停服后将卷打包到部署目录之外的安全位置：

```sh
BACKUP_DIR="${BLOG_BACKUP_DIR:-$HOME/blog-backups}"
mkdir -p "$BACKUP_DIR"
docker compose stop blog
docker run --rm -v iblog_data:/data:ro -v "$BACKUP_DIR:/backup" alpine:3.20 \
  tar -czf /backup/iblog-data-$(date +%F-%H%M%S).tar.gz -C / data
docker compose start blog
```

## 升级与回滚

镜像发布成功后，在服务器 `.env` 中将 `IBLOG_VERSION` 改为目标版本，然后：

```sh
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 blog
```

回滚时把 `IBLOG_VERSION` 改回先前版本，再执行相同命令。若新版本已迁移数据库，回滚前先确认旧版本兼容当前数据库；必要时从备份恢复。

## 发布流程（维护者）

向 `main` 推送代码会触发 CI。发布新镜像时，先确认 CI 通过，再推送版本标签，例如：

```sh
git tag v1.0.1
git push origin v1.0.1
```

Actions 会在标签发布时测试并构建 `linux/amd64` 镜像，推送 `ghcr.io/charlesfield19/iblog:v1.0.1` 和 `ghcr.io/charlesfield19/iblog:latest`。首次发布后，如需免登录拉取，请在 GitHub Packages 中将容器包可见性设为 Public。

## 本地验证

CI 会运行 Go 测试、`go vet` 和 Vue 前端构建。Docker 镜像也在 Actions 中构建；服务器部署不需要本地构建步骤。`compose.yaml` 使用已发布的 GHCR 镜像，不是源码构建配置。
