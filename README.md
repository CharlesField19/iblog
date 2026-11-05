# iBlog

这是基于原容器中的 Go 可执行文件、前端构建产物和 SQLite 表结构，重新编写的**功能兼容实现**，不是逐字复原的原始源码。项目不会读取或导入原博客的数据。

## 当前组成

- `cmd/blog/`：可编辑的 Go 后端源码；包含公开接口、管理员登录、文章/标签/友链/设置管理、账号修改、SQLite 初始化、静态页面服务。
- `frontend/web/src/`、`frontend/admin/src/`：本次重建的 Vue 3 可编辑页面、路由、表单、管理视图和样式；外观参考原镜像 CSS，About/contacts 不写入旧的个人资料默认值。
- `frontend/web/dist/`、`frontend/admin/dist/`：从原镜像提取的只读对照构建文件；`frontend/*/dist-rebuilt/` 是由新 Vue 源码生成的独立构建结果。不要把这两类构建混淆。
- `cmd/blog/migrations/`：按版本号顺序执行并记入数据库迁移账本；启动时只应用未执行的迁移，不覆盖已有内容。
- `Dockerfile` / `compose.yaml`：多阶段构建和 Compose 部署。数据库与自动生成的 JWT 密钥保存在持久化 Docker 卷中。
- `tools/with-local-env.sh`：构建/测试的隔离环境包装器。

程序已在**临时空数据库**上与原二进制对比过建表结构（表、列、外键一致）及 17 个代表性的接口请求（状态码和 JSON 响应一致），并补充了文章、标签、友链、设置、登录、账号修改、CORS 和分页测试；仍不能据此断言所有边界行为都完全相同。原程序更新文章后会把 `createdAt`/`updatedAt` 错写为公元元年，本项目刻意不复现这一错误。新项目使用 PBKDF2 保存**新建账号**密码（原版使用 bcrypt），因此不能直接把旧数据库账号记录移植进来；本项目不需要旧数据。新实现的 SQLite 驱动使用本地已有的 `modernc.org/sqlite v1.36.1`，并非原程序的 v1.34.4。

## Docker Compose 部署

需要安装 Docker Engine 和 Docker Compose 插件。在服务器上取得 GitHub 仓库后：

```sh
cp .env.example .env
# 默认通过宿主机 11190 端口访问；再启动并自动构建镜像
docker compose up -d --build
```

默认通过 `http://服务器地址:11190/` 和 `http://服务器地址:11190/admin/` 访问。首次访问后台会要求创建管理员用户名和密码。初始化接口只在数据库中还没有管理员时开放；创建成功后不能再次初始化。JWT 签名密钥由程序随机生成，保存在卷内 `/data/jwt_secret`，不会显示在网页，也不需要写进 `.env`。

可以在 `.env` 自定义宿主机绑定地址和端口。比如只允许本机访问，并把宿主机 `10001` 映射到容器 `11190`：

```dotenv
BLOG_PORT=127.0.0.1:10001
```

对应端口映射为 `127.0.0.1:10001:11190`，然后执行 `docker compose up -d`；访问地址是 `http://127.0.0.1:10001/`。若希望映射到其他宿主机端口但监听所有网卡，例如宿主机 `10001`，设 `BLOG_PORT=10001`，映射即为 `10001:11190`。容器内应用端口固定为 `11190`。

文章、账号、设置和 JWT 密钥都在名为 `iblog_data` 的 Docker 命名卷内。更新源码后执行 `git pull` 和 `docker compose up -d --build` 会替换应用镜像，但继续挂载同一个卷。**不要执行 `docker compose down -v`，也不要删除 `iblog_data` 卷**，否则会删除持久数据。

备份时先暂停写入，打包卷到仓库目录之外的安全位置：

```sh
BACKUP_DIR="${BLOG_BACKUP_DIR:-$HOME/blog-backups}"
mkdir -p "$BACKUP_DIR"
docker compose stop blog
docker run --rm -v iblog_data:/data:ro -v "$BACKUP_DIR:/backup" alpine:3.20 \
  tar -czf /backup/blog-data-$(date +%F-%H%M%S).tar.gz -C / data
docker compose start blog
```

定期把备份复制到另一台机器或可靠的备份存储。恢复前停止博客，先保留当前卷副本，再将备份解压回卷。数据库结构迁移在每次启动时自动按版本顺序执行；重要升级前仍应先备份。

## 本地源码构建和隔离验证

Go 1.23.12 放在父目录 `../go/`；所需模块、构建缓存、临时文件、HOME、npm/pip 缓存都放在父目录 `../.cache/` 等目录里。执行 Go 命令时务必使用：

```sh
./tools/with-local-env.sh go test -count=1 ./...
./tools/with-local-env.sh go test -race -count=1 ./...
./tools/with-local-env.sh go vet ./...
./tools/with-local-env.sh go build -o ../.cache/tmp/blog-reconstructed ./cmd/blog
./tools/with-local-env.sh npm --prefix frontend run build
```

Node 24.22.2 和 npm 11.19.1 的可执行文件也保存在 `../blog-reconstruction/tools/node/` 下，项目依赖安装在 `frontend/node_modules/`，缓存留在 `../.cache/npm/`。包装器不在目录外安装工具，也不向目录外写缓存。运行本地 npm/Go 命令时使用包装器。Docker 构建会使用 Docker daemon 自己的镜像和构建缓存；本项目的自包含约束只适用于本地恢复/测试过程。不要将真实 `.env`、数据库、密钥或备份提交到 GitHub。

## 宿主机直接安装（不使用 Docker）

宿主机需要 Go 1.23+、Node.js 24+ 和 npm。先在源码目录编译后端并构建前端：

```sh
npm ci --prefix frontend
npm --prefix frontend run build
mkdir -p build
go build -trimpath -o build/iblog ./cmd/blog
```

这里生成的 `build/iblog` 是编译后的后端可执行文件；Vue 前端构建结果位于 `frontend/web/dist-rebuilt/` 和 `frontend/admin/dist-rebuilt/`。把可执行文件和这两个目录一起部署到宿主机，或直接在源码目录运行。无需预先编译后才能用 Docker：`docker compose up -d --build` 会在 Docker 构建过程中自动编译前后端。

在源码目录直接运行示例（数据会放在用户目录，不覆盖线上数据）：

```sh
mkdir -p "$HOME/.local/share/iblog"
BLOG_ADDR=:11190 \
BLOG_DB_PATH="$HOME/.local/share/iblog/blog.db" \
BLOG_WEB_DIST="$(pwd)/frontend/web/dist-rebuilt" \
BLOG_ADMIN_DIST="$(pwd)/frontend/admin/dist-rebuilt" \
./build/iblog
```

默认监听宿主机 `11190` 端口。若只想在本机 `10001` 端口监听，把 `BLOG_ADDR` 改为 `127.0.0.1:10001`；若需要从外部访问，可用 `:10001`，并按需配置防火墙。首次访问对应地址的 `/admin/` 创建管理员。JWT 密钥自动保存在数据库同目录的 `jwt_secret` 文件中；数据库和密钥必须一起备份。生产环境建议用 systemd 等服务管理器托管进程，并为数据目录设置仅服务用户可读写的权限。

**未更改 VPS 上运行的博客或原恢复备份。** 旧版遗留说明：本重建使用 PBKDF2 密码哈希，不能移植原站点用户哈希；本项目从独立空数据开始。
