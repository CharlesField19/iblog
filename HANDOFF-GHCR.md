# iBlog 交接文档：由本地 PC 发布 GHCR Docker 镜像

> 目标：在有 GitHub 网络的本地 PC 上，把 iBlog 源码推送到 GitHub；配置 GitHub Actions，在版本发布时自动测试、构建并推送 Docker 镜像到 GitHub Container Registry（GHCR）；服务器只拉取镜像并用 Docker Compose 运行，不安装 Go/Node，也不在服务器编译。

## 1. 当前已知状态

- 项目源码目录：`blog-reconstruction/`（本文件所在目录）。只复制/上传该目录，不要上传它的父目录。
- 当前工作副本不是 Git 仓库；需要在本地 PC 初始化 Git 并关联新建的 GitHub 仓库。
- `Dockerfile` 已包含 Vue 前端和 Go 后端的多阶段构建，容器内应用监听端口为 `11190`。
- 当前 `compose.yaml` 仍设置了 `build: .`，属于服务器本地构建模式；发布 GHCR 后要改为仅拉取 `image:`，并删除 `build:`。
- `.github/workflows/ci.yml` 目前负责检查/构建，不会发布 GHCR 镜像。需要新增发布工作流。
- Compose 使用命名卷 `iblog_data` 持久化 SQLite 数据和 JWT 密钥。升级不可删除此卷。
- Docker 端口由 `BLOG_PORT` 配置；例如 `127.0.0.1:10001` 表示主机只监听本机 10001，并映射到容器 11190。
- iBlog 后端测试、`go vet`、前端构建和 Compose 配置检查此前通过；尚未实际构建 Docker 镜像、推送 GitHub 或发布 GHCR。

## 2. 本地 PC 准备

本地 PC 需要 Git 和可用的 GitHub 账号。创建一个 GitHub 仓库，例如 `iblog`。若希望服务器公开拉取镜像免登录，建议仓库公开，并在镜像首次发布后把 GHCR Package 设为 Public。

把本项目的 `blog-reconstruction/` 目录完整复制到本地 PC。不要复制博客工作区父目录中的恢复归档、原始镜像、数据库或工具缓存。

在项目根目录先检查文件并初始化 Git：

```sh
git status --short
git init -b main
git add .
git status --short
```

检查暂存列表，确认没有 `.env`、数据库（`*.db`）、JWT 密钥、备份、`node_modules` 或本地缓存后提交。项目 `.gitignore` 已排除大部分生成物，但仍需人工复核。

```sh
git commit -m "Initial iBlog source"
git remote add origin https://github.com/OWNER/iblog.git
git push -u origin main
```

将 `OWNER` 替换为个人用户名或组织名。仓库/镜像路径建议使用全小写，例如 `ghcr.io/owner/iblog`。

## 3. 新增 GHCR 发布工作流

在项目中新增 `.github/workflows/publish-image.yml`。下面的工作流在推送 `v*` 版本标签时运行测试，之后构建 Linux `amd64` 镜像并发布带版本号和 `latest` 标签的镜像。使用 GitHub Actions 内置的 `GITHUB_TOKEN`，不需要创建或提交 PAT。

```yaml
name: Publish Docker image

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: go test -count=1 ./...
      - run: go vet ./...
      - run: npm ci --prefix frontend --no-audit --no-fund
      - run: npm --prefix frontend run build

  publish:
    needs: test
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Set image name
        id: image
        shell: bash
        run: echo "name=ghcr.io/${GITHUB_REPOSITORY,,}" >> "$GITHUB_OUTPUT"
      - uses: docker/metadata-action@v5
        id: meta
        with:
          images: ${{ steps.image.outputs.name }}
          tags: |
            type=ref,event=tag
            type=raw,value=latest
      - uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          platforms: linux/amd64
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### 工作流要点

- 只对版本标签发布；推送普通分支不会发布正式镜像。
- `publish` 必须等 `test` 成功后才运行。
- 首次公开发布完成后，在 GitHub 的 **Packages → 对应容器包 → Package settings → Change visibility** 将包设为 **Public**。如果不公开，服务器必须先登录 GHCR 才能拉镜像。
- 这个示例先发布 `linux/amd64`。只有明确需要 ARM64 服务器时，再把构建平台扩展到 `linux/amd64,linux/arm64` 并配置 QEMU。
- 第三方 GitHub Actions 版本固定为主版本标签；对高安全要求的发布流程，可进一步固定到经过审查的完整 commit SHA。

## 4. 把 Compose 改为拉取镜像

编辑 `compose.yaml`，将服务定义中的：

```yaml
build: .
image: iblog:latest
```

改为只引用 GHCR 镜像（用自己的 `OWNER` 替换）：

```yaml
image: ghcr.io/OWNER/iblog:${IBLOG_VERSION:-v1.0.0}
```

删除 `build: .`。保留端口映射、环境变量、健康检查和 `iblog_data` 命名卷。端口项应保持类似：

```yaml
ports:
  - "${BLOG_PORT:-11190}:11190"
```

这样服务器执行 `docker compose pull` 时只拉取镜像，不会从源码构建。为本地环境配置：

```dotenv
IBLOG_VERSION=v1.0.0
BLOG_PORT=127.0.0.1:10001
```

如果要让服务直接通过服务器网卡的 11190 端口访问，可用 `BLOG_PORT=11190`；是否对公网开放还取决于防火墙规则。前面有宿主机 Nginx 时，推荐绑定 `127.0.0.1`，例如 `127.0.0.1:10001`，并让 Nginx 反向代理该端口。

镜像标签和主机端口是两件事：`IBLOG_VERSION` 选软件版本；`BLOG_PORT` 设主机对外绑定。容器内端口固定为 `11190`。

## 5. 上传配置并发布第一个镜像

将发布工作流和 Compose 修改提交并推到 GitHub：

```sh
git add .github/workflows/publish-image.yml compose.yaml
git commit -m "Publish versioned images to GHCR"
git push origin main
```

创建并推送首个版本标签：

```sh
git tag v1.0.0
git push origin v1.0.0
```

打开 GitHub 仓库的 **Actions** 页面，确认 `Publish Docker image` 的测试和发布任务均成功。之后在 **Packages** 页面确认镜像路径为：

```text
ghcr.io/OWNER/iblog:v1.0.0
ghcr.io/OWNER/iblog:latest
```

若公开安装，确认 GHCR Package 可见性为 Public。公开镜像无需在服务器运行 `docker login`。

## 6. 服务器首次部署

服务器只需要 Docker Engine 和 Compose 插件，不需要 Git、Go、Node 或源码。先将改好的 `compose.yaml` 和 `.env.example` 放到单独部署目录，并用正确的 `OWNER` 替换镜像路径，然后：

```sh
cp .env.example .env
# 编辑 .env，至少检查/设置 IBLOG_VERSION 和 BLOG_PORT
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 blog
```

首次访问 `http://服务器地址:主机端口/admin/` 创建管理员。默认端口是 11190；若 `BLOG_PORT=127.0.0.1:10001`，该服务只可从本机访问，外部通过 Nginx 等入口访问。

确保命名卷 `iblog_data` 已创建并持续使用。**不得运行 `docker compose down -v`，也不得删除该卷**。数据库与 `/data/jwt_secret` 都在该卷中。

## 7. 后续升级和回滚

发布新版本时先更新源码、测试并提交，然后创建新标签，例如：

```sh
git tag v1.0.1
git push origin v1.0.1
```

等 Actions 发布成功后，服务器把 `.env` 中版本更新为 `IBLOG_VERSION=v1.0.1`，再执行：

```sh
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 blog
```

回滚时把 `IBLOG_VERSION` 改回上一版本，再 `docker compose pull && docker compose up -d`。如果新版本已运行数据库迁移，回滚应用前必须确认旧版本兼容已迁移的数据库；升级前先按 README 备份数据卷。

## 8. 验收清单

- [ ] 源码仓库没有 `.env`、数据库、密钥、备份或缓存。
- [ ] Actions 中版本标签工作流的测试任务通过。
- [ ] GHCR 中存在版本镜像和 `latest` 镜像。
- [ ] GHCR Package 可见性符合预期（公开安装需 Public）。
- [ ] 服务器 Compose 仅有 `image:`，没有 `build:`。
- [ ] `docker compose pull` 成功，服务器无需源码和构建工具。
- [ ] `docker compose up -d` 后健康检查通过。
- [ ] 首次管理员初始化成功；重启容器后账号、文章、站点设置和 JWT 密钥仍存在。
- [ ] 已验证备份可用；没有删除 `iblog_data` 卷。

## 9. 交接边界

本方案需要在有 GitHub 网络的 PC 上执行。当前工作环境无法访问 GitHub，且没有替用户创建仓库、推送提交、创建 Release 或发布镜像。PC 操作者需要维护仓库 `OWNER/iblog` 的真实地址、镜像公开策略和发布版本。发布成功后，服务器只需 Compose 文件和 `.env`，不必保存源码。
