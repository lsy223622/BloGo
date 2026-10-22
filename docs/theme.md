# 默认主题

页面结构参考作者使用的 Jasmine 2.5.5 修订版：桌面为窄导航、阅读区和信息栏，采用 1 / 8 / 3 的比例；移动端为单列。Go 模板和主题样式在本仓库实现，参考来源为 [Jasmine](https://github.com/liaocp666/Jasmine)（Kent Liao，GPLv3）。未复制其 PHP、字体或打包 CSS。

`templates/` 的 `.html` / `.tmpl` 文件定义 `page` 模板。普通元数据始终交给 `html/template` 转义；只有已处理的 Markdown 或作者 HTML 正文进入可信 HTML 插入点。Standalone 不套主题模板，保留原文；Unlisted Standalone 必须在 head 内自行声明 robots noindex。

可选站点 `extra` 字段为 `tagline` 和 `description`。内容 `extra.cover` 是封面资源 URL；`extra.links` 是友链列表，每项包含 `name`、`url`、`image`、`description`。这些值仍为普通模板数据，不执行表达式。

渲染回归复用仓库中的 Public、无日期独立页面和 Unlisted Standalone 样例。正文、标题转义、HTML 片段、noindex 和 tokenizer 插入位置在 `internal/render` 测试。

主题资源位于 `static/assets/`，使用系统字体链、CSS 变量、回环本地预览与原生 `<dialog>` 灯箱。深浅色选择可保存到本地存储；存储不可用时仍可切换。代码使用 Chroma 的 GitHub / GitHub Dark 配色，公式由 `static/vendor/katex-0.19.0/` 的自托管 KaTeX 增强。KaTeX 的 MIT 许可保留在该目录的 LICENSE，JS 的 SHA-256 为 `103a53763cc033bba8d175bf3f0ba597c3505c9b6747dd3f2c7bc2a6bfcc8ae7`。

2026-10-23 本地 Edge/Playwright 核对首页、文章、分类、标签、归档和友链，共 1440/390 像素 × 浅色/深色的 24 组页面：无横向溢出，友链头像为 48×48，长名称换行。公式及代码高亮加载成功，灯箱可以打开并按 Escape 关闭。关闭 JavaScript 后正文、导航和公式源码仍可读。浅色辅助文字对比度为 4.67:1，强调色为 5.08:1；暗色代码残留浅色 token 的问题先复现后修正。截图保存在本地 `output/playwright/`。
