# iBlog

iBlog 是一个 Go + Vue 博客应用。正式部署使用 GitHub Actions 构建并发布 Docker 镜像到 GHCR；服务器只需 Docker Engine 和 Docker Compose，不需要安装 Go、Node.js，也不需要下载源码或在服务器编译。

## 部署前提

- 已发布的镜像：`ghcr.io/charlesfield19/iblog`
- 服务器已安装 Docker Engine 和 Docker Compose 插件。
- GHCR 镜像若设为 Public，服务器无需登录 GHCR；若为 Private，需先在服务器执行 `docker login ghcr.io`。

> 当前版本镜像只有在 GitHub Actions 的 CI 通过并推送版本标签后才会发布。版本标签工作流会运行测试、构建镜像，并推送版本标签和 `latest`。

## 首次部署

将 `compose.yaml` 放入服务器的 Docker 部署面板或 Compose 项目目录。镜像版本和端口已经直接写在配置里，无需额外创建 `.env` 文件。然后拉取镜像并启动：

```sh
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 blog
```

默认配置使用镜像 `ghcr.io/charlesfield19/iblog:v1.0.0`，并将主机端口 `11190` 映射到容器端口 `11190`。如需只允许本机或反向代理访问，直接在 `compose.yaml` 中修改端口：

```yaml
ports:
  - "127.0.0.1:10001:11190"
```

容器内端口固定为 `11190`。默认 `11190:11190` 会绑定服务器所有网卡；是否能从公网访问还取决于防火墙规则。

启动后访问 `http://服务器地址:端口/admin/`，按页面提示创建管理员用户名和密码。该初始化只在数据库中没有管理员时开放。管理员密码在管理界面设置，不需要写入 Compose 配置。

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

镜像发布成功后，在 `compose.yaml` 中将 `image:` 改为目标版本，然后重新拉取并重建容器：

```sh
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 blog
```

回滚时把 `image:` 改回先前版本，再执行相同命令。若新版本已迁移数据库，回滚前先确认旧版本兼容当前数据库；必要时从备份恢复。

## 发布流程（维护者）

向 `main` 推送代码会触发 CI。发布新镜像时，先确认 CI 通过，再推送版本标签，例如：

```sh
git tag v1.0.1
git push origin v1.0.1
```

Actions 会在标签发布时测试并构建 `linux/amd64` 镜像，推送 `ghcr.io/charlesfield19/iblog:v1.0.1` 和 `ghcr.io/charlesfield19/iblog:latest`。首次发布后，如需免登录拉取，请在 GitHub Packages 中将容器包可见性设为 Public。

## 本地验证

CI 会运行 Go 测试、`go vet` 和 Vue 前端构建。Docker 镜像也在 Actions 中构建；服务器部署不需要本地构建步骤。`compose.yaml` 使用已发布的 GHCR 镜像，不是源码构建配置。
