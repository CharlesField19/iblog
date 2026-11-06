# iBlog v1.0.5

## 更新内容

- 将 Compose 默认 Docker 镜像、环境变量示例和备份验证脚本的默认镜像版本统一到 `v1.0.5`。
- 修正仓库部署配置滞后于最新发行版的问题；本次不包含应用代码或数据库结构变更。

## 升级方式

```sh
docker compose pull
docker compose up -d
```

部署、备份和回滚说明请参阅[简体中文 README](README.md) 或 [English README](README.en.md)。
