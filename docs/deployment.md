# Linux 部署、备份与升级

## 解压并试运行

选择 Linux amd64 或 arm64 正式发行包，核对 SHA256SUMS，再解压到普通用户可读取的目录。程序为静态 Go 二进制，运行环境无需 Go、Node、Python、SQLite CLI 或数据库服务器。

```sh
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf BloGo-vX.Y.Z-linux-amd64.tar.gz
cd BloGo-vX.Y.Z-linux-amd64
```

先将示例版本号与架构替换成实际文件名。在 config.yaml 填写公开 origin（`server.base_url: https://blog.example.com`），在 content/site.yaml 编辑展示内容。保存配置后再运行：

```sh
./blog serve
```

默认监听 127.0.0.1:25646。首次启动建立私有 data、完整发布版本、统计数据库和访客密钥；重启保留这些数据。一个 data 目录只允许一个运行实例。试运行结束按 Ctrl+C 停止，再安装开机服务。

## 一个运行用户与 systemd

以下使用固定安装目录 /opt/blogo，按实际位置调整；命令由有 sudo 权限的管理员执行。blogo 是无交互登录的系统账号，只用于运行服务，不需要设置密码。配置和文章由管理员或自己的维护账号编辑，服务账号只需读取 config/content/templates/static，并写入 data。

确认已解压发行包、填写配置并停止试运行。从已解压包的根目录（能看到 blog、config.yaml 和 content/）一次执行下面整段首次安装命令，将文件复制到安装位置并注册服务。若试运行创建了 data，也一并迁移并修正其所有权：

```sh
sudo useradd --system --user-group --home /opt/blogo --shell /usr/sbin/nologin blogo
sudo install -d -m 0755 /opt/blogo
sudo cp -a . /opt/blogo/
sudo install -d -o blogo -g blogo -m 0700 /opt/blogo/data
sudo chown -R blogo:blogo /opt/blogo/data
sudo install -m 0644 /opt/blogo/deploy/blogo.service /etc/systemd/system/blogo.service
sudo systemctl daemon-reload
sudo systemctl enable --now blogo
sudo journalctl -u blogo -f
```

最后一行持续显示服务日志，按 Ctrl+C 退出日志查看，服务仍在后台运行。

服务示例只允许写 /opt/blogo/data；若自定义 paths.data，需同步 ReadWritePaths。修改扫描间隔、报告间隔、详细开关与保留期后程序会重读配置。修改监听地址、base_url、timezone 或路径需重启；程序对这类修改整份拒绝热应用，并记录 restart required。

维护文章可使用 `sudoedit /opt/blogo/content/posts/文章.md`，或为自己的维护账号安排该内容目录的写权限；不需要登录 blogo 账号，也不需要让程序拥有文章的写权限。

## 反代与 TLS

`deploy/nginx.conf.example` 是纯反向代理，域名和证书由管理员配置。所有请求转发到同一个 Go 上游，不需要发布文件路径、统计 alias 或重定向 map。Go 负责目录规范化、历史 aliases、404、资源缓存及统计端点。

示例文件放在 Nginx 的 http 配置上下文中，upstream 与 server 同级；例如使用由 http 块 include 的站点配置文件。`keepalive 16` 为每个 worker 缓存最多 16 条空闲上游连接，配合 HTTP/1.1 和清空 Connection 头复用到 BloGo 的连接。这不是总连接数上限。修改配置后先执行 `nginx -t`，通过后重载 Nginx。

默认 trusted_proxies 仅包含回环地址；跨机代理必须填写其实际 IP/CIDR。只有直接连接可信代理时才读取 X-Forwarded-For，地址链从右向左取最近的不可信节点。不要填写无边界的 0.0.0.0/0 或 ::/0。使用反代访问的 origin 必须与 server.base_url 一致。

## 运行数据和故障

`data/current` 指向 data/releases 内的成功发布目录。构建在 data/staging 进行，同一文件系统内原子切换链接。主配置 config.yaml 的 `build.keep_releases` 默认 3，表示保留的网页构建版本数，包含当前版本；它与程序升级包的保留无关。输入、资源或构建错误保留旧页面，日志记录错误并在后续周期重试。运行时串行调度构建、报告和清理，HTTP 仍读取预先生成的静态文件。

公网页面只提供当前 public 目录和两个统计端点；配置、内容源文件、meta、数据库、密钥及私有报告均不可从 HTTP 获取。整个 data 目录应保持私有，不能把它作为其他文件服务器的 root。

## 备份、恢复和升级

最简单可靠的备份：停止 BloGo，复制 config.yaml、content/ 和完整 data/。数据库、visitor.key 必须来自同一备份；不要只复制运行中的 stats.db 而丢掉 WAL。在线备份方法见统计文档。

升级流程：

1. 下载并校验新包，先解压到临时工作目录，查看该版本的配置变更说明。
2. 执行 `sudo systemctl stop blogo`，备份完整 /opt/blogo 安装目录，以便同时恢复旧程序、定制资源、内容和统计。
3. 在原来的 /opt/blogo 中替换 `blog`，完整替换新版本的默认 `templates/`、`static/`，同步 README、docs、deploy 和许可证文件。替换后的 blog 需保留执行权限。自定义模板和静态资源按备份自行合并。
4. 保留原有 `config.yaml`、整个 `content/` 和 `data/`，只按版本说明手工调整必要配置。不要将新包的示例 config/content 复制到现有站点，也不要把整个包直接解压覆盖安装目录。
5. 执行 `sudo systemctl start blogo`，用 `sudo journalctl -u blogo -n 50 --no-pager` 检查日志，再检查页面、附件和统计。程序或默认资源改变时会重建页面。

安装位置保持 /opt/blogo，因此无需为每次程序升级修改开机服务路径。仅当该版本确实调整了服务配置时，合并 deploy/blogo.service 的变化，再执行 `sudo systemctl daemon-reload`。升级期间允许短暂停服；恢复服务后反代继续使用原端口。

恢复时先停止进程，将备份整体还原后启动。已有非空数据库缺少原访客密钥时启动失败，不会生成替代密钥。明细迁移在事务内完成；未知 schema 明确失败。

需要人工回滚页面时先停止进程，核对保留 release 的 public/meta 后，用同目录临时符号链接原子替换 current，再将内容恢复到对应版本；重启前确保输入与目标版本一致，避免扫描重新发布新内容。

## Windows 预览包

执行 `.\blog.exe dev`，默认端口25646；可用 `--listen 127.0.0.1:另一个端口`，预览 origin 从该明确地址推导。dev 在 data/dev 存储自己的统计和预览快照，不改写正式数据。草稿可读但不采集。`build --output output/site` 可生成独立生产输出。

正式 serve 的发布实现依赖 Linux 文件系统上的符号链接与原子替换，并只开放 Linux 入口。Windows dev 使用独立预览快照，包含草稿和浏览器刷新，因此发行包标为 preview。Linux 上也可以使用 dev。
