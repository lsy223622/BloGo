# 统计数据层

SQLite 由 `database/sql` 与锁定的 `modernc.org/sqlite v1.61.0` 提供，实测引擎为 SQLite 3.54.1。使用本地磁盘、WAL、每连接 5 秒 busy timeout；采集连接池最多一条写连接。每个事件的幂等标记、当日访客标记和三种计数在同一短事务中完成，任一步失败全部回滚。

数据库 schema 版本为 user_version=1，只支持空数据库初始化；非空且无版本或未知版本的数据库明确失败。内容不入库，历史聚合不依赖当前 Manifest 的外键，因此删除文章后仍能保留统计。

每日访客仅保存 `HMAC-SHA256(key, day + NUL + visitor_id)` 的 32 字节摘要，不存储浏览器原始访客标识。密钥首次启动创建为私有的 32 字节二进制文件，之后只读取同一密钥；大小或权限不合法时失败，不能靠重启随机重建。报告不需要密钥。

事件幂等记录按 expires_at 清理，访客去重记录按站点日期和保留时间清理；每日站点、页面及来源聚合不删除。报告连接使用 `mode=ro` 和 query_only，不修改业务数据；SQLite 所需 WAL/SHM 仍依照引擎规则管理，不使用 immutable。

Windows 与 Linux 非特权测试覆盖：并发重复只计一次、同日多页 UV 一次、跨日摘要变化、逐页来源、故障触发器回滚、短读快照不阻塞采集、writer 运行/停止/重启时只读报告均能读取，以及 30 ms 上下文取消的锁等待不产生部分记录。

行为依据：[SQLite WAL 与只读辅助文件](https://www.sqlite.org/wal.html)、[一致备份与 VACUUM INTO](https://www.sqlite.org/backup.html)、[驱动文档](https://pkg.go.dev/modernc.org/sqlite)。
