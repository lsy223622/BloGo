# 配置与命令

配置使用严格 YAML：未知键、重复键、错误类型及非法日期明确失败；扩展展示数据写入 `extra` 映射。相对路径（包括 build/dev 的 output）以配置文件目录为基准。完整约束见 [技术设计第 6 节](design/DESIGN.md#6-配置与命令)。

| 配置 | 默认值或用途 |
| --- | --- |
| `site.base_url` | 必填 HTTP(S) origin，不含路径、查询、片段或凭据；与浏览器实际 origin 一致 |
| `site.title` | 构建必填；用于页面、Feed 与报告标题 |
| `site.subtitle` | 页头副标题，可留空 |
| `site.timezone` | `Asia/Shanghai`，IANA 时区；内置 tzdata |
| `site.language` | `zh-CN` |
| `extra.profile` | 标签下面的站长信息列表，每项为 `type`、`label` 和可选的 `url`；图标类型及示例见[主题说明](theme.md) |
| `paths.source/templates/static` | 构建输入根目录，普通目录且相互独立 |
| `paths.taxonomies` | `taxonomies.yaml` |
| `paths.staging/releases/current` | Linux 发布目录与当前版本链接；同一文件系统 |
| `build.scan_interval` | `60s`，watch 扫描周期 |
| `build.summary_max_chars` | `160`，自动摘要 Unicode 字符上限 |
| `build.page_size` | `10`，公共列表分页大小 |
| `build.strict_links` | `false`；缺失本地内容链接为警告，true 时失败；资源缺失总是失败 |
| `build.keep_releases` | `3`，保留完整版本数量，始终保留 current |
| `stats.listen_addr` | `127.0.0.1:8787`，只能使用回环 IP |
| `stats.manifest_path` | 当前 release/meta/manifest.json，只读 |
| `stats.database_path` | SQLite 主文件，位于独立私有数据库目录 |
| `stats.visitor_key_path` | 持久 32 字节二进制密钥，首次 collector 启动创建；Unix 0600 |
| `stats.public_output` | Nginx 精确 alias 的独立 stats.json |
| `stats.private_output` | 公共映射之外的 report.html |
| `stats.event_retention` | `72h`，去重事件保留时间 |
| `stats.visitor_retention` | `168h`，每日访客摘要保留时间 |
| `stats.report_interval` | `5m`，部署约定；timer 的 OnUnitActiveSec 需同步设置 |

静态构建不需要统计配置，stats/report 不需要访问内容、模板或 Git。路径关系会检查所有声明路径，但物理访问只检查当前命令的路径。数据库、密钥、报告、发布目标不得覆盖输入或落入公共输出；不能把独立 build 输出同时声明为统计所依赖的 Manifest 目标。

六个命令都支持 `--config` 与 `--help`：

| 命令 | 行为 |
| --- | --- |
| `blog build --output output/site` | 独立生产构建，生成 public/meta；不切换 current |
| `blog dev --output output/dev --listen 127.0.0.1:8080` | 包含 Draft 的回环预览；成功重建后自动刷新 |
| `blog publish` | Linux 上校验完整 release 后原子切换 current |
| `blog watch` | Linux 上持有发布锁，按指纹发现变更并发布；SIGINT/SIGTERM 停止 |
| `blog stats` | 回环事件服务，读取 Manifest 并更新 SQLite |
| `blog report` | 单次只读聚合，生成公开 JSON 与私有 HTML |

失败返回非零退出码。build/publish/watch/dev 的内容错误包含源路径与相关字段；stats 的 HTTP 错误不暴露内部路径与 SQL。
