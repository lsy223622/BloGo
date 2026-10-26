# 配置与命令

配置使用严格 YAML：未知键、重复键、错误类型及非法日期明确失败；扩展展示数据写入 `extra` 映射。相对路径（包括 build/dev 的 output）以配置文件目录为基准。完整约束见 [技术设计第 6 节](design/DESIGN.md#6-配置与命令)。

`config.yaml` 只保存后台运行设置。所有相对路径以它所在目录为站点根目录，与启动时的工作目录无关。

| 配置 | 默认值或用途 |
| --- | --- |
| `server.listen_addr` | `127.0.0.1:25646`，回环 IP 与端口 |
| `server.base_url` | `http://127.0.0.1:25646`，不带子路径、凭据、查询或片段的 HTTP(S) origin |
| `server.trusted_proxies` | 仅 `127.0.0.1` 与 `::1`；跨机代理填写明确 IP 或 CIDR |
| `timezone` | `Asia/Shanghai`，IANA 时区，内置 tzdata |
| `paths.content` / `paths.templates` / `paths.static` | 默认 `content` / `templates` / `static` |
| `paths.data` | 默认 `data`，运行数据根目录，必须与输入分离 |
| `build.scan_interval` | `60s`，内容检查周期 |
| `build.keep_releases` | `3`，保留版本数，始终包含当前版本 |
| `build.strict_links` | `false`，严格模式下缺失本地内容链接失败；缺失资源总是失败 |
| `stats.enabled` | `true`，启用采集与报告 |
| `stats.report_interval` | `5m`，公开与私有报告刷新周期 |
| `stats.details.enabled` | `false`，可选逐事件明细 |
| `stats.details.retention` | `168h`，明细保留期 |
| `stats.event_retention` / `stats.visitor_retention` | `72h` / `168h`，去重事件及每日访客摘要保留期 |

展示设置统一在 `content/site.yaml`：`schema_version: 1`、`title`、`subtitle`、`language`（默认 `zh-CN`）、`page_size`（10）、`summary_max_chars`（160）、`logo`、`favicon`、`profile`、`categories`、`tags`。Logo 与 favicon 默认使用 `/images/logo.svg`、`/images/favicon.svg`。分类标签继续使用稳定 ASCII ID，中文显示名、分类父级和内容状态规则不变。

`content/assets/a/b.png` 的公开地址为 `/media/a/b.png`。附件与站点设置均参与构建快照和指纹；符号链接、缺失资源或与默认静态文件冲突会使构建失败。

监听地址、base_url、timezone 及路径变化需要重启；扫描/报告间隔、统计开关、详细开关与保留期可以热更新。配置错误保留最后有效设置。扫描周期缩短会被独立的配置检查及时发现。

七个命令支持 `--config` 和 `--help`：

| 命令 | 行为 |
| --- | --- |
| `blog serve` | Linux 正式服务，构建、页面、统计、报告与清理在一个进程内 |
| `blog dev` | Windows/Linux 回环预览；默认25646，包含草稿，统计使用data/dev |
| `blog build --output output/site` | 独立生产构建，生成public/meta |
| `blog publish` | 手工构建并原子发布Linux release |
| `blog watch` | 手工运行内容扫描发布 |
| `blog stats` | 手工调试采集器，使用推导的正式Manifest和数据库 |
| `blog report` | 手工生成一份公开JSON与私有HTML |

失败返回非零退出码。正常部署使用serve；手工调试命令不应与正在使用相同数据和端口的服务并行运行。内容错误包含源路径及字段，HTTP错误不暴露SQL或内部文件路径。

自动页面标题/简介、分类标签上下文和公开文章JSON-LD复用现有展示设置与文章元数据。订阅默认为全文并同时保留摘要，入口与格式见[内容输出说明](content-output.md#站点订阅)。
