---
id: welcome
title: 欢迎使用 BloGo
published: 2026-10-23
status: public
aliases:
  - /POSTS/WELCOME/
  - /Legacy.html
categories:
  - hardware
tags:
  - esp32
summary: 解压发行包，修改两份配置，即可开始写作和预览。
extra:
  cover: /images/example.svg
---
[TOC]

## 快速开始

下载对应平台的发行包并解压，程序无需额外运行时。Linux 使用 `./blog serve`；Windows 使用 `./blog.exe dev`，打开 http://127.0.0.1:25646/。

## 两份配置

`config.yaml` 保存监听地址、网站 origin、扫描间隔和统计设置。正式域名在 `server.base_url` 填写，例如 `https://blog.example.com`。

`content/site.yaml` 保存网站名称、副标题、Logo、每页条数、个人信息、分类和标签。

## 新建文章

在 `content/posts/` 新建 UTF-8 Markdown 文件：

```markdown
---
id: my-first-post
title: 我的第一篇文章
status: public
published: 2026-10-25
---
这里开始写正文。
```

保存后等待 `build.scan_interval`（默认一分钟），成功构建后新内容生效。Windows `dev` 会自动刷新打开的页面；正式页面由读者刷新或重新导航取得。

- [格式和 Markdown 完整示例](/posts/first-steps/)
- [附件、配置和升级](/posts/publishing-notes/)

![Go 示例图片](/images/go.svg)

```go
fmt.Println("BloGo")
```
