# 统计数据层

SQLite 由 `database/sql` 与锁定的 `modernc.org/sqlite v1.61.0` 提供，实测引擎为 SQLite 3.54.1。使用本地磁盘、WAL、每连接 5 秒 busy timeout；采集连接池最多一条写连接。每个事件的幂等标记、当日访客标记和三种计数在同一短事务中完成，任一步失败全部回滚。

数据库 schema 版本为 user_version=1，只支持空数据库初始化；非空且无版本或未知版本的数据库明确失败。内容不入库，历史聚合不依赖当前 Manifest 的外键，因此删除文章后仍能保留统计。

每日访客仅保存 `HMAC-SHA256(key, day + NUL + visitor_id)` 的 32 字节摘要，不存储浏览器原始访客标识。密钥首次启动创建为私有的 32 字节二进制文件，之后只读取同一密钥；大小或权限不合法时失败，不能靠重启随机重建。报告不需要密钥。

事件幂等记录按 expires_at 清理，访客去重记录按站点日期和保留时间清理；每日站点、页面及来源聚合不删除。报告连接使用 `mode=ro` 和 query_only，不修改业务数据；SQLite 所需 WAL/SHM 仍依照引擎规则管理，不使用 immutable。

Windows 与 Linux 非特权测试覆盖：并发重复只计一次、同日多页 UV 一次、跨日摘要变化、逐页来源、故障触发器回滚、短读快照不阻塞采集、writer 运行/停止/重启时只读报告均能读取，以及 30 ms 上下文取消的锁等待不产生部分记录。

行为依据：[SQLite WAL 与只读辅助文件](https://www.sqlite.org/wal.html)、[一致备份与 VACUUM INTO](https://www.sqlite.org/backup.html)、[驱动文档](https://pkg.go.dev/modernc.org/sqlite)。

## 事件 API

`blog stats --config <file>` 只读取站点时区/origin、当前 Manifest、数据库和密钥。服务仅监听配置的回环 IP，唯一采集端点为 `POST /api/events`，JSON 请求体最多 2 KiB。

请求必须恰好包含 `v`、`event_id`、`page_id`、`visitor_id`、`referrer_host`。拒绝未知键、重复键、null、类型错误、多个 JSON 值和无效 UTF-8；两个浏览器标识为 32 字符小写十六进制，版本为整数 1。页面必须存在于当前 Manifest、为 Public 或 Unlisted 且启用统计。当前 Manifest 无法读取时返回 503，而不是继续接受过期身份。

导航来源只接受主机名或 IP，规范为小写 ASCII/IDNA；空值记 direct，与站点主机一致记 internal。HTTP Referer 用于辅助检查采集请求的同源及已知页面关系，不作为导航来源，也不保存；Origin/Referer 缺失不等于请求无效。已改 URL 但仍存在的稳定 ID 可接收旧浏览器页面事件；删除的 ID 被拒绝。

Go 进程使用总量 token bucket（100 次/秒、突发 200），Nginx 另按实际连接地址限制 10 次/秒、突发 20，不信任用户转发头。数据库操作使用请求上下文与 5 秒超时。响应为 204（含幂等重复）、400、413、415、429 或 503，其他方法返回 405；客户端不会收到内部 SQL 错误或文件路径，也不开放 CORS。

## 浏览器与报告

每个启用统计的 HTML（包括分类、归档、分页、404 和 Standalone）使用同一自托管脚本。首次可见时发送一次；Beacon 入队失败至多一秒后再试一次，复用完整事件。访客标识按发送时站点时区日期保存到 localStorage，存储被拒绝时使用临时随机标识。只发送导航来源主机名，不发送 URL、查询、片段或客户端日期。浏览器拒绝脚本或网络失败时允许漏计。

```sh
blog stats --config /etc/blog/stats.yaml
blog report --config /etc/blog/stats.yaml
```

报告为单次命令，由 `blog-report.timer` 每五分钟启动。修改 `stats.report_interval` 时应同步修改 timer 的 `OnUnitActiveSec`。公开 JSON 含 Manifest 版本、所有启用统计的 Public 页面的累计 PV，以及仅文章的近 30 天排名。私有 HTML 提供每日趋势、7 天/30 天/累计 PV 与每日 UV 之和、来源总量和逐页来源，保留 Unlisted 和已删除 ID 的历史数据。跨日 UV 求和不代表跨日去重人数。

报告通过两个目标的进程锁避免竞争，短只读事务完成后才渲染。输出先写同目录临时文件，写入前再次核对 build_id；版本变化和生成失败保留旧结果。两个输出分别原子替换。公开内容按本轮 Manifest 过滤，可见性变化在下一次成功刷新后反映；上次成功报告可暂时保留旧信息。
