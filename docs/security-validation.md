# V1 权限与故障验收

应用账户及目录布局见 [部署说明](deployment.md)。自动测试使用临时目录与独立容器，以下是可复现的检查入口。

| 边界 | 验证入口 | 通过条件 |
| --- | --- | --- |
| 源文件、模板、静态资源 | `TestReadSourcesRejectsSymlinks`、`TestScanRejectsLinksAndSpecialFiles` | 链接和 FIFO 明确拒绝，读取受 os.Root 约束 |
| 路径关系 | `TestInvalidIdentities`、`TestInputOutputAndPrivatePathSeparation`、`TestLinkedOutputOverlap` | 拒绝穿越、非法路由及私有路径与输出重叠 |
| 发布 | `TestPublishAtomicVersionAndRetention`、`TestFailedCurrentSwitchPreservesLink` | 失败保留旧 current，进程锁拒绝并发，HTML 与 Manifest 同版本 |
| 历史重定向 | `scripts/verify-nginx.ps1` | 生成的 map 区分路径大小写，转义点字符，规范 URL 不循环 |
| 预览 | `TestPreviewCanonicalRoutesDraftsAndPrivateFiles` | 回环访问、规范 301、404、禁止 meta 与目录列表 |
| 采集 API | `TestEventAPIRejectsMalformedAndUnwantedFields`、`TestManifestReloadAndControlledFailure` | 严格字段与大小限制，失效身份拒绝，数据库故障返回受控 503 |
| SQLite 事务 | `TestTransactionFailureRollsBackEverything`、`TestLockDeadlineDoesNotPartiallyWrite` | 错误与超时无部分更新 |
| 报告 | `TestReportVisibilityHistoryAndReadOnlySnapshot`、`TestReportLocksAndManifestChangePreserveOutputs` | 本轮公开输出仅含 Public 身份，版本变化与失败保留上次输出 |
| 实际 Linux 账户 | `scripts/verify-stack.ps1` | collector/reporter 无法读取内容与凭据，Nginx 无法读取 Manifest、数据库、密钥和私有报告 |
| Nginx 防滥用 | `scripts/verify-stack.ps1` | 2 KiB 限制返回 413，伪造转发头不能绕过连接来源限速，突发返回 429 |
| 恢复 | `TestVacuumBackupAndKeyRestoreContinueCollection` 与部署栈 | 一致快照与原密钥恢复后继续去重、UV、PV 和报告 |

Linux 测试需在本地 Linux 文件系统运行；Windows 目录挂载不能证明 Unix mode 或原子链接切换行为。隔离栈把数据放在容器 `/tmp`，根文件系统和源码只读，无外部网络，也不发布主机端口。容器 root 仅创建临时测试目录并分配身份，应用进程分别降权。

内容是受信任作者输入，HTML 与 Markdown 中的原始 HTML 可运行作者脚本。统计接口不提供内容写入或 SQL 查询能力。Unlisted 是列表可见性选择，直接 URL 仍可访问；公开报告采用五分钟最终一致性，改变可见性后，上次成功报告在刷新前可能保留旧条目。
