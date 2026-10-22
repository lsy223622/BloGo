# BloGo

Go 源码及行为契约见 [技术设计](docs/design/DESIGN.md) 和 [实施计划](docs/design/IMPLEMENTATION_PLAN.md)。

使用 Go 1.27 或更新版本：

```sh
go test ./...
go vet ./...
go run ./cmd/blog --help
go run ./cmd/blog build --help
```

`config.yaml` 是静态站点配置示例。相对路径（包括 `--output`）相对于配置文件所在目录解析；输出目录必须独立于内容、模板、静态资源和配置。YAML 拒绝未知键、重复键和类型错误，扩展数据写入 `extra` 映射。日期使用配置的 IANA 时区，程序内含时区数据库。

路径关系先按所有声明路径检查，再物理解析当前命令需要访问的路径；其他组件的私有目录无需对当前进程开放。发布配置的 `current` 可以尚不存在；已存在时必须为指向 releases 内子目录的符号链接。Linux 部署还要求 staging、releases 和 current 所在目录处于同一文件系统，统计密钥不对组和其他用户开放。

内容入口为 `internal/site.LoadContent`，它读取 `content/posts/`、`content/pages/` 和分类标签文件，校验身份、规范 URL、状态、日期和引用后按稳定 ID 返回内容。生产读取过滤 Draft；预览读取包含 Draft；Public 与 Unlisted 保留独立的公共列表语义。

示例内容包含 Public 文章、无日期独立页面、Unlisted HTML 和无日期 Draft。分类标签的键使用 ASCII 稳定 ID，`name` 保留中文、大小写和 emoji。HTML 正文与 Markdown 源码在解析时保持原文。
