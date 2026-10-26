# BloGo

一个用 Go 预先生成完整 HTML 的单作者博客。下载发行包、解压并启动即可阅读和写作，不需要安装 Go、Node、Python 或数据库服务器。

## 开始使用

- Linux amd64/arm64：选择对应 `.tar.gz`，解压后执行 `./blog serve`。
- Windows amd64：选择 `-preview.zip`，解压后执行 `.\blog.exe dev`。

Linux 首次使用时，将下方版本号和架构替换为下载的文件名，再解压进入目录：

```sh
tar -xzf BloGo-vX.Y.Z-linux-amd64.tar.gz
cd BloGo-vX.Y.Z-linux-amd64
```

先编辑 `config.yaml` 和 `content/site.yaml`；正式部署时填写自己的 `server.base_url`。保存后启动：

```sh
./blog serve
```

浏览器打开 <http://127.0.0.1:25646/>。首次运行会建立私有 `data/`；正式站点请先在 `config.yaml` 填写 `server.base_url`，再按 [部署说明](docs/deployment.md) 配置域名反代和 TLS。

编辑 `content/site.yaml` 设置名称、副标题、图标、每页条数、联系方式、分类和标签。在 `content/posts/` 写 Markdown 或 HTML。附件放 `content/assets/`，例如 `content/assets/photo.png` 的地址是 `/media/photo.png`。

`serve` 是一个进程、一个端口，负责页面、事件采集、内容扫描和定时报告。默认每分钟检查内容、每五分钟刷新统计。`dev` 包含草稿并自动刷新浏览器，统计只记非草稿内容，数据独立存放在 `data/dev/`。

## Linux 基础部署

先自行解压发行包，填写 `config.yaml` 和 `content/site.yaml`，可在当前目录执行 `./blog serve` 试运行。准备正式安装时按 Ctrl+C 停止试运行，留在解压包的根目录（能看到 `blog`、`config.yaml` 和 `content/`），使用有 sudo 权限的账号一次执行下面整段首次安装命令：

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

这会将当前包及已有试运行数据复制到固定安装目录 `/opt/blogo`，由专用 `blogo` 账号运行，并启用开机自启。最后一行持续显示日志，按 Ctrl+C 退出日志查看，服务仍在后台运行。配置和文章由你的维护账号或管理员编辑，程序只需读取内容、写入 `data/`。

在 `config.yaml` 中将 `server.base_url` 设为自己的公开域名；通过反代将该域名的请求转发到 `127.0.0.1:25646` 并配置 TLS。Nginx 可参考 [反代示例](deploy/nginx.conf.example)，替换域名与证书路径；使用面板管理反代时，填写同一上游地址即可。跨机代理还需填写 `server.trusted_proxies`。目录调整、备份及原位升级见 [部署说明](docs/deployment.md)。

## 写作与管理

- [配置与命令](docs/configuration.md)
- [全部文章元数据与 Markdown 示例](docs/content-example.md)
- [内容身份、分类、标签与 URL](docs/content-output.md)
- [构建、扫描与失败保留](docs/build.md)
- [默认主题和局部导航](docs/theme.md)
- [Linux 部署、备份与升级](docs/deployment.md)
- [统计与可选详细事件](docs/statistics.md)

私有报告在 `data/private/report.html`，本地预览则在 `data/dev/private/report.html`。它不通过站点 HTTP 提供，复制到本机直接打开即可。升级时停止服务并备份安装目录，在原位置更新程序和默认资源，保留自己的 `config.yaml`、`content/` 和完整 `data/`，再启动服务。数据库与原访客密钥必须一起保留；具体操作见部署说明。

## 从源码开发

构建与测试使用 `go.mod` 的 `toolchain` 指定的 Go 1.27.3。CI 与发行构建读取同一版本；本地保持默认 `GOTOOLCHAIN=auto`，或安装该版本及后续安全补丁版。JavaScript 和导入工具测试另需 Node 与 Python；这些都不是发行包的运行依赖。

```sh
go test ./...
go vet ./...
node --test scripts/*browser.test.cjs
python -m unittest discover -s scripts -p 'test_*.py'
go run ./cmd/blog dev
```

[技术设计](docs/design/DESIGN.md)、[发行实施计划](docs/design/RELEASE_IMPLEMENTATION_PLAN.md) 与 [验收说明](docs/security-validation.md) 记录架构及验证方法。

发行工作流在 `v*` 标签触发：测试、锁定模块构建、白名单打包、解压验收，最后上传发行资产。源码仓库与发行包使用通用示例身份，个人输入与运行数据不进入包。
