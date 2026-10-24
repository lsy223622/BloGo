# Linux 部署、备份与升级

## 解压并试运行

选择 Linux amd64 或 arm64 正式发行包，核对 SHA256SUMS，再解压到普通用户可读取的目录。程序为静态 Go 二进制，运行环境无需 Go、Node、Python、SQLite CLI 或数据库服务器。

```sh
sha256sum -c SHA256SUMS
tar -xzf BloGo-vX.Y.Z-linux-amd64.tar.gz
cd BloGo-vX.Y.Z-linux-amd64
./blog serve --config config.yaml
```

在 config.yaml 填写公开 origin（`server.base_url: https://blog.example.com`），在 content/site.yaml 编辑展示内容。默认监听 127.0.0.1:25646。首次启动建立私有 data、完整发布版本、统计数据库和访客密钥；重启保留这些数据。一个 data 目录只允许一个运行实例。

## 一个运行用户与 systemd

以下路径为例子，按实际安装位置调整；命令由管理员执行。输入可由站长维护，运行用户需能读取 config/content/templates/static，并写入 data。

```sh
sudo useradd --system --home /opt/blogo --shell /usr/sbin/nologin blogo
sudo install -d -m 0755 /opt/blogo
# 将已解压包的文件复制到 /opt/blogo，填写配置和内容。
sudo install -d -o blogo -g blogo -m 0700 /opt/blogo/data
sudo chown -R blogo:blogo /opt/blogo/content
sudo install -m 0644 deploy/blogo.service /etc/systemd/system/blogo.service
sudo systemctl daemon-reload
sudo systemctl enable --now blogo
sudo journalctl -u blogo -f
```

服务示例只允许写 /opt/blogo/data；若自定义 paths.data，需同步 ReadWritePaths。修改扫描间隔、报告间隔、详细开关与保留期后程序会重读配置。修改监听地址、base_url、timezone 或路径需重启；程序对这类修改整份拒绝热应用，并记录 restart required。

## 反代与 TLS

`deploy/nginx.conf.example` 是纯反向代理，域名和证书由管理员配置。所有请求转发到同一个 Go 上游，不需要发布文件路径、统计 alias 或重定向 map。Go 负责目录规范化、历史 aliases、404、资源缓存及统计端点。

默认 trusted_proxies 仅包含回环地址；跨机代理必须填写其实际 IP/CIDR。只有直接连接可信代理时才读取 X-Forwarded-For，地址链从右向左取最近的不可信节点。不要填写无边界的 0.0.0.0/0 或 ::/0。使用反代访问的 origin 必须与 server.base_url 一致。

## 运行数据和故障

`data/current` 指向 data/releases 内的成功发布目录。构建在 data/staging 进行，同一文件系统内原子切换链接；keep_releases 默认 3。输入、资源或构建错误保留旧页面，日志记录错误并在后续周期重试。运行时串行调度构建、报告和清理，HTTP 仍读取预先生成的静态文件。

公网页面只提供当前 public 目录和两个统计端点；配置、内容源文件、meta、数据库、密钥及私有报告均不可从 HTTP 获取。整个 data 目录应保持私有，不能把它作为其他文件服务器的 root。

## 备份、恢复和升级

最简单可靠的备份：停止 BloGo，复制 config.yaml、content/ 和完整 data/。数据库、visitor.key 必须来自同一备份；不要只复制运行中的 stats.db 而丢掉 WAL。在线备份方法见统计文档。

升级流程：

1. 停止旧程序，保存上述备份。
2. 新包解压到新目录，保留新包的默认 templates/static。
3. 用旧 config.yaml、content/、data/ 覆盖新包示例；自定义模板和静态资源自行合并。
4. 从新目录启动并检查日志、页面、文章身份、附件和统计。输入不变但程序或默认资源改变时会重建。

恢复时先停止进程，将备份整体还原后启动。已有非空数据库缺少原访客密钥时启动失败，不会生成替代密钥。明细迁移在事务内完成；未知 schema 明确失败。

需要人工回滚页面时先停止进程，核对保留 release 的 public/meta 后，用同目录临时符号链接原子替换 current，再将内容恢复到对应版本；重启前确保输入与目标版本一致，避免扫描重新发布新内容。

## Windows 预览包

执行 `.\blog.exe dev`，默认端口25646；可用 `--listen 127.0.0.1:另一个端口`，预览 origin 从该明确地址推导。dev 在 data/dev 存储自己的统计和预览快照，不改写正式数据。草稿可读但不采集。`build --output output/site` 可生成独立生产输出。正式 Linux serve 的发布语义在 Linux 文件系统验收。
