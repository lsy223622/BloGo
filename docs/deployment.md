# Linux 部署

以下是需要管理员手动执行和审查的部署示例，不由构建器执行。先在本地隔离环境验收；应用进程使用非 root 账户，输入和发布目录位于 Linux 本地文件系统。

## 账户与目录

- `blog-build`：读取 source，写 staging/releases/current；主组 `blog-meta`。
- `blog-stats`：只读发布 Manifest，写统计数据库；加入 `blog-meta` 与 `blog-reports`。
- `blog-report`：读取 Manifest 和数据库，写公开 JSON 和私有报告；加入 `blog-meta`、`blog-reports` 与 `blog-web`。
- `www-data`：通过 `blog-web` 读取公开统计 JSON，只读 release 的 public；不加入其他应用组。

推荐目录：

```text
/srv/blog/source                 root:blog-source       0750
/srv/blog/staging                blog-build:blog-meta   0750
/srv/blog/releases               blog-build:blog-meta   0755
/srv/blog/current                -> releases/build-...
/var/lib/blog-stats              blog-stats:blog-reports 0750
/var/lib/blog-stats/database     blog-stats:blog-reports 2770
/srv/blog-stats-public           blog-report:blog-web   2750
/srv/blog-stats-private          blog-report:blog-report 0700
```

source 文件按组可读，构建账户加入 `blog-source`。把发布内容同步到 source 时排除 `.git` 和凭据；可选外部 Git 同步账户的凭据留在其自己的私有 home，构建服务不拥有或读取它们。构建结果的 release/public 可读，meta 为 0750、内部文件为 0640，Manifest 只授予 `blog-meta` 组。

watcher 使用 UMask=0022 保证 public 的目录 0755、文件 0644；metadata 由构建器显式收紧为 0750/0640。权限验收直接读取所提供 unit 的 UMask 来启动 builder，避免测试环境默认值掩盖部署问题。

数据库单独放在 database 子目录，允许 SQLite 管理 WAL/SHM；访客密钥保存在上一级，仅 `blog-stats` 可读写的 0600 文件，报告账户无需读取或替换密钥。不要将数据库、密钥、source 或私有报告配置成 Nginx alias。

统计服务使用 UMask=0007，新建数据库为 0660，WAL/SHM 继承相同权限。已有数据库由管理员核对 owner/group/mode，程序不替现有文件更改权限。报告账户需在 database 目录创建 SQLite 辅助文件；示例 report unit 同时把主数据库文件单独标记为 ReadOnlyPaths，业务连接另使用 mode=ro 与 query_only。应用主进程不需要 root。

## 安装和发布

管理员安装 Go 编译的 `blog` 到 `/usr/local/bin/blog`，将示例 `deploy/config-build.yaml` 复制为 `/etc/blog/build.yaml`，修改站点 origin 与路径，配置文件仅保存路径和普通站点数据，不嵌入密钥。使用对应非 root 账户运行：

```sh
blog build --config /etc/blog/build.yaml --output /srv/blog/manual-build
blog publish --config /etc/blog/build.yaml
blog watch --config /etc/blog/build.yaml
```

staging、releases 和 current 父目录必须同文件系统。首次 current 可以不存在，之后必须指向 releases 内的有效版本。安装 `blog-watch.service` 前审查实际账户、组、路径及 systemd 沙箱设置；由管理员执行 daemon-reload 和启动。应用不会运行 Git、修改 Nginx 或重载服务。

## Nginx

`deploy/nginx.conf` 是可由普通用户运行的完整示例，监听 8080，PID 与临时文件位于 `/tmp`。实际域名、公开端口和 TLS 证书由管理员按现有环境配置，并保持 `site.base_url` 与外部 origin 一致。已有 Nginx 部署可提取 server/http 相关指令合并，应用本身不做这些变更。

Nginx root 指向 current/public；公开统计只精确 alias 到 `/data/stats.json`。API 使用 loopback 上游，限制 2 KiB 请求和来源连接速率，不信任客户端转发头。HTML 与原路径资源使用 no-cache；含内容哈希的 CSS/JS 使用长期 immutable 缓存。

管理员先核对 release/meta/redirects.conf，再将需要的条目复制到 `/etc/blog/redirects.conf`，文件需对 Nginx 可读，执行 `nginx -t` 后人工重载。无尾斜杠和 index.html 的通用规范跳转无需逐次重载；历史 aliases 的映射由管理员审核应用。所有跳转为 301 并保留 query，未知 URL 返回站点 404 且保持 404 状态。

规则依据：[Nginx core/error_page/try_files](https://nginx.org/en/docs/http/ngx_http_core_module.html)、[rewrite/return](https://nginx.org/en/docs/http/ngx_http_rewrite_module.html)、[limit_req](https://nginx.org/en/docs/http/ngx_http_limit_req_module.html)。

## 回滚

由管理员停止 watcher，验证待选版本的 public 和 meta 完整，再在 current 同目录创建指向保留 release 的临时链接，以 `mv -Tf` 原子替换 current。核对首页、Manifest 和规范跳转；确认源内容与选定版本一致后重新启动 watcher。不得原地编辑 current/public，也不要在 watcher 运行时删除其锁文件或数据库辅助文件。

## 本地验收

先执行独立构建，再准备官方 Nginx 镜像，运行 `scripts/verify-nginx.ps1`。脚本只创建带 `blogo.validation=true` 标签的专用容器：普通 nginx 用户、只读根文件系统、仅发布到 127.0.0.1 的动态端口，退出时核对标签并清理该容器。它不会操作其他容器或线上 Nginx；默认 Go 测试不依赖 Docker 或外网。

2026-10-23 实测 Docker 29.9.2、Nginx 1.30.6（官方镜像 digest `sha256:47928bf67eb62a1863cb08d9ecb52d3e704304ba614eaee17884204677702951`），`nginx -t` 与 17 项 HTTP/缓存断言通过，包含大小写不同的 aliases、正则特殊字符转义、规范路径不循环和 query 保留；Nginx UID 为 101。测试容器直接加载本轮生成的 meta/redirects.conf。结果写入本地 `output/validation/nginx.json`。Linux 非特权测试另验证了真实本地文件系统上的原子发布、失败保留、版本保留、元数据权限和人工回滚。

统计配置见 `deploy/config-stats.yaml`，安装为 `/etc/blog/stats.yaml` 后审查 `blog-stats.service`、`blog-report.service` 和 `blog-report.timer`。先发布站点、启动 collector 产生数据库，再运行一次报告，最后启用五分钟 timer。统计仅监听回环地址；公网只接入 Nginx。报告输出不属于 release，重新发布博客不会覆盖它们。

完整本地验收在 Windows 交叉编译 Linux 二进制后运行：

```powershell
$env:GOOS='linux'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
go build -o .superpowers/linux/blog ./cmd/blog
go test -c ./internal/integration -o .superpowers/linux/integration.test
./scripts/verify-stack.ps1
Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED
```

脚本使用 network=none 的专用容器，不发布端口，源码只读挂载，测试数据在容器 /tmp 的真实 Linux 文件系统。容器 root 仅设置临时账户权限，builder、collector、reporter 和 Nginx 分别以不同非 root UID 运行。测试覆盖 WAL/SHM 权限、私有文件拒绝读取、API 代理、统计停机/重启、报告失败恢复和活动数据库备份恢复，结果写入 `output/validation/stack.json`。

Windows 可运行 `./scripts/verify-clean.ps1`：从当前 Git HEAD 导出全新源码目录，按 go.mod/go.sum 编译、运行全部 Go 测试并构建示例站点。该检查复用已下载的 Go 模块缓存，首次环境仍需能够下载锁定依赖。页面数和完整构建耗时写入 `output/validation/clean-checkout.json`；它测量示例站点，不代表大型内容库性能。
