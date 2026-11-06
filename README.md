# iBlog

**语言 / Language:** 简体中文 · [English](README.en.md)

iBlog 是我为自己搭建的博客。比起追求复杂的平台能力，我更希望它简单、稳定，能让我专注写作，也能让我清楚地掌握自己的内容和数据。

项目使用 Go、Vue 3 和 SQLite，并以一个 Docker 容器运行。这个选择是有意的：个人博客不需要微服务，也不应该为了部署它先维护一整套基础设施。一个容器、一份持久化数据，就能把博客运行起来。

## 我想解决的问题

写文章、管理标签和页面这些事情应该足够直接。iBlog 提供文章发布与草稿、Markdown 编辑和预览、搜索与标签筛选，以及友情链接、关于页面和站点设置管理。

内容保存在 SQLite 中，管理入口与公开站点一起部署。系统尽量减少外部依赖，并把备份和恢复作为日常使用的一部分，而不是等出问题后才考虑的补丁。

## 部署

我把部署保持在尽可能少的步骤里。服务器安装 Docker Engine 和 Docker Compose 后，使用仓库中的 `compose.yaml` 启动：

```sh
docker compose pull
docker compose up -d
```

站点默认监听 `11190` 端口，管理入口位于 `/admin/`。第一次打开管理入口时，可以按页面提示创建管理员账号。生产环境建议通过 HTTPS 反向代理访问；HTTPS 部署时，将 `BLOG_COOKIE_SECURE` 设置为 `true`。

## 数据与备份

文章、账号、站点设置、数据库和会话签名密钥保存在 Docker 数据卷 `iblog_data` 中。更新或重建容器不会删除这个数据卷；**不要使用 `docker compose down -v`**，也不要手动删除 `iblog_data`。

升级之前先备份。仓库提供了备份脚本：

```sh
BLOG_BACKUP_DIR="$HOME/blog-backups" sh scripts/backup.sh
```

备份也应该定期做恢复验证，并保存到服务器之外。备份文件里包含博客数据和密钥，请按敏感数据妥善保管。

## 升级与回滚

发布新版本后，将 `compose.yaml` 中的镜像版本改为目标版本，再执行：

```sh
docker compose pull
docker compose up -d
```

如果需要回滚，把镜像版本改回之前使用的版本，再执行同样的命令。升级前保留备份；如果新版本修改了数据库结构，回滚前先确认旧版本能读取当前数据，必要时从备份恢复。
