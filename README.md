# BloGo

一个用 Go 预先生成完整 HTML 的单作者博客。下载发行包、解压并启动即可阅读和写作，不需要安装 Go、Node、Python 或数据库服务器。

## 开始使用

- Linux amd64/arm64：选择对应 `.tar.gz`，解压后执行 `./blog serve`。
- Windows amd64：选择 `-preview.zip`，解压后执行 `.\blog.exe dev`。

浏览器打开 <http://127.0.0.1:25646/>。首次运行会建立私有 `data/`；正式站点请先在 `config.yaml` 填写 `server.base_url`，再按 [部署说明](docs/deployment.md) 配置域名反代和 TLS。

编辑 `content/site.yaml` 设置名称、副标题、图标、每页条数、联系方式、分类和标签。在 `content/posts/` 写 Markdown 或 HTML。附件放 `content/assets/`，例如 `content/assets/photo.png` 的地址是 `/media/photo.png`。

`serve` 是一个进程、一个端口，负责页面、事件采集、内容扫描和定时报告。默认每分钟检查内容、每五分钟刷新统计。`dev` 包含草稿并自动刷新浏览器，统计只记非草稿内容，数据独立存放在 `data/dev/`。

## 写作与管理

- [配置与命令](docs/configuration.md)
- [全部文章元数据与 Markdown 示例](docs/content-example.md)
- [内容身份、分类、标签与 URL](docs/content-output.md)
- [构建、扫描与失败保留](docs/build.md)
- [默认主题和局部导航](docs/theme.md)
- [Linux 部署、备份与升级](docs/deployment.md)
- [统计与可选详细事件](docs/statistics.md)

私有报告在 `data/private/report.html`，本地预览则在 `data/dev/private/report.html`。它不通过站点 HTTP 提供，复制到本机直接打开即可。升级前停止程序并备份 `config.yaml`、`content/` 和完整 `data/`；新包解压到新目录，再迁移这三项。数据库与原访客密钥必须一起保留。

## 从源码开发

构建环境需要 Go 1.27+。JavaScript 和导入工具测试另需 Node 与 Python；这些都不是发行包的运行依赖。

```sh
go test ./...
go vet ./...
node --test scripts/*browser.test.cjs
python -m unittest discover -s scripts -p 'test_*.py'
go run ./cmd/blog dev
```

[技术设计](docs/design/DESIGN.md)、[发行实施计划](docs/design/RELEASE_IMPLEMENTATION_PLAN.md) 与 [验收说明](docs/security-validation.md) 记录架构及验证方法。

发行工作流在 `v*` 标签触发：测试、锁定模块构建、白名单打包、解压验收，最后上传发行资产。源码仓库与发行包使用通用示例身份，个人输入与运行数据不进入包。
