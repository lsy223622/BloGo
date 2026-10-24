---
id: release-notes
slug: publishing-notes
title: 附件、配置与升级
published: 2026-10-20
updated: 2026-10-23T19:00:00+08:00
status: public
categories:
  - hardware
tags:
  - esp32
---
[TOC]

## 内容与附件

长期维护的输入放在 `content/`。`posts/` 保存文章，`pages/` 保存独立页面。`public` 内容进入列表，`unlisted` 可直接访问并带 noindex，`draft` 只在 `dev` 预览中出现，且不采集统计。

附件 `content/assets/example.txt` 的地址是 [/media/example.txt](/media/example.txt)。图片写作 `![说明](/media/photo.png)`。个人 Logo 和 favicon 也放在 assets 中，再在 site.yaml 指定 `/media/brand/logo.svg`。

分类和标签先在 `content/site.yaml` 配置稳定 ID，再在文章的 categories/tags 列表中引用。名称可以使用中文，ID 用小写 ASCII 和连字符。未配置分类不影响文章生成。只使用但未配置的标签不会显示。

## 运行设置

```yaml
server:
  listen_addr: 127.0.0.1:25646
  base_url: https://blog.example.com
  trusted_proxies:
    - 127.0.0.1
    - "::1"
timezone: Asia/Shanghai
build:
  scan_interval: 60s
  keep_releases: 3
stats:
  enabled: true
  report_interval: 5m
  details:
    enabled: false
    retention: 168h
```

反代只需将请求转发到一个端口。详细事件默认关闭，开启后私有报告记录最近事件；公开页面只取得聚合计数。报告文件位于 `data/private/report.html`，复制到本机打开。整个 data 目录需要备份。

## 升级

1. 停止正在运行的进程。
2. 新发行包解压到新目录。
3. 迁移旧 `config.yaml`、`content/` 和 `data/`，覆盖新包中的示例输入。
4. 检查配置后从新目录启动。程序会根据新的默认主题重新构建；统计数据库和原访客密钥一起保留。

修改过默认 templates/static 时自行合并定制。输入或构建出错会保留上次成功页面，错误显示在进程日志。显式 ID `release-notes` 让本篇文章即使修改 slug，仍沿用相同统计身份。

访问 [关于页面](/about/) 查看项目说明。
