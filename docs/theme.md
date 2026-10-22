# 默认主题

页面结构参考作者使用的 Jasmine 2.5.5 修订版：桌面为窄导航、阅读区和信息栏，采用 1 / 8 / 3 的比例；移动端为单列。Go 模板和主题样式在本仓库实现，参考来源为 [Jasmine](https://github.com/liaocp666/Jasmine)（Kent Liao，GPLv3）。未复制其 PHP、字体或打包 CSS。

`templates/` 的 `.html` / `.tmpl` 文件定义 `page` 模板。普通元数据始终交给 `html/template` 转义；只有已处理的 Markdown 或作者 HTML 正文进入可信 HTML 插入点。Standalone 不套主题模板，保留原文；Unlisted Standalone 必须在 head 内自行声明 robots noindex。

可选站点 `extra` 字段为 `tagline` 和 `description`。内容 `extra.cover` 是封面资源 URL；`extra.links` 是友链列表，每项包含 `name`、`url`、`image`、`description`。这些值仍为普通模板数据，不执行表达式。

渲染回归复用仓库中的 Public、无日期独立页面和 Unlisted Standalone 样例。正文、标题转义、HTML 片段、noindex 和 tokenizer 插入位置在 `internal/render` 测试。
