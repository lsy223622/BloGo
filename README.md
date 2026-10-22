# BloGo

Go 源码及行为契约见 [技术设计](docs/design/DESIGN.md) 和 [实施计划](docs/design/IMPLEMENTATION_PLAN.md)。

使用 Go 1.27 或更新版本：

```sh
go test ./...
go vet ./...
go run ./cmd/blog --help
go run ./cmd/blog build --help
go run ./cmd/blog build --config config.yaml --output output/site
```

`config.yaml` 是静态站点配置示例。相对路径（包括 `--output`）相对于配置文件所在目录解析；输出目录必须独立于内容、模板、静态资源和配置。YAML 拒绝未知键、重复键和类型错误，扩展数据写入 `extra` 映射。日期使用配置的 IANA 时区，程序内含时区数据库。

路径关系先按所有声明路径检查，再物理解析当前命令需要访问的路径；其他组件的私有目录无需对当前进程开放。发布配置的 `current` 可以尚不存在；已存在时必须为指向 releases 内子目录的符号链接。Linux 部署还要求 staging、releases 和 current 所在目录处于同一文件系统，统计密钥不对组和其他用户开放。

内容入口为 `internal/site.LoadContent`，它读取 `content/posts/`、`content/pages/` 和分类标签文件，校验身份、规范 URL、状态、日期和引用后按稳定 ID 返回内容。生产读取过滤 Draft；预览读取包含 Draft；Public 与 Unlisted 保留独立的公共列表语义。

示例内容包含 Public 文章、无日期独立页面、Unlisted HTML 和无日期 Draft。分类标签的键使用 ASCII 稳定 ID，`name` 保留中文、大小写和 emoji。HTML 正文与 Markdown 源码在解析时保持原文。

完整静态构建、输入快照与链接校验见 [构建说明](docs/build.md)，默认主题与自托管资源见 [主题说明](docs/theme.md)。

本地预览包含 Draft，使用独立目录并在成功重建后刷新浏览器：

```sh
go run ./cmd/blog dev --config config.yaml --output output/dev --listen 127.0.0.1:8080
```

六个命令为 build、dev、publish、watch、stats、report。Linux 的 publish/watch 生成完整 release 后原子切换 current，stats 只监听回环，report 单次生成公开 JSON 与私有 HTML。安装示例分别使用 `deploy/config-build.yaml` 和 `deploy/config-stats.yaml`。

- [配置与命令](docs/configuration.md)
- [内容模型与输出](docs/content-output.md)
- [Markdown 支持](docs/markdown-support.md)
- [Linux 部署与回滚](docs/deployment.md)
- [统计、指标与备份恢复](docs/statistics.md)
- [安全与权限验收](docs/security-validation.md)

静态页面由 Nginx 提供；统计停机不阻断阅读。公开统计每五分钟刷新，只含本轮 Manifest 中启用统计的 Public 页面。Unlisted 明细与已删除页面历史留在私有报告。浏览器使用每日随机标识，服务端只持久化按日期 HMAC 的访客摘要与来源主机名。

完整测试：

```sh
go test ./...
go vet ./...
node --test scripts/stats-browser.test.cjs
```

Go 测试不依赖外网或 Docker。Linux 权限、原子发布和恢复的隔离验收步骤见部署说明；生产域名、TLS、systemd 安装与 Nginx 重载由管理员按实际环境审查执行。
