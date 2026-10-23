# 默认主题

页面结构参考作者使用的 Jasmine 2.5.5 修订版：桌面为窄导航、阅读区和信息栏，采用 1 / 8 / 3 的比例；移动端为单列。Go 模板和主题样式在本仓库实现，参考来源为 [Jasmine](https://github.com/liaocp666/Jasmine)（Kent Liao，GPLv3）。未复制其 PHP、字体或打包 CSS。

`templates/` 的 `.html` / `.tmpl` 文件定义 `page` 模板。普通元数据始终交给 `html/template` 转义；只有已处理的 Markdown 或作者 HTML 正文进入可信 HTML 插入点。Standalone 不套主题模板，保留原文；Unlisted Standalone 必须在 head 内自行声明 robots noindex。

站点名称、副标题和每页条数分别在 `config.yaml` 的 `site.title`、`site.subtitle`、`build.page_size` 配置。内容 `extra.cover` 是封面资源 URL；`extra.links` 是友链列表，每项包含 `name`、`url`、`image`、`description`。这些值仍为普通模板数据，不执行表达式。

友链按列表顺序显示，每项四个字段分别是名称、目标网址、头像和介绍。直接编辑对应页面的 Front Matter，例如：

```yaml
extra:
  links:
    - name: Go 项目与语言文档
      url: https://go.dev/
      image: /images/go.svg
      description: Go 编程语言
```

站点配置的 `extra.profile` 控制标签下面的“关于站长”信息，按列表顺序显示。每项包含 `type`、`label` 和可选的 `url`；支持的图标类型为 `qq`、`email`、`github`、`website`、`cc`。不填写 `url` 时显示普通文字。图标保存在本地 `static/images/profile.svg`，使用当前主题的文字颜色。

`email` 项仅填写 `label` 中的邮箱地址。生成时混淆地址，浏览器通过本地 JavaScript 还原成可选中、复制的普通文字。混淆用于减少简单采集脚本直接提取地址，不能阻止模拟浏览器的采集。

```yaml
extra:
  profile:
    - type: email
      label: hello@example.com
    - type: cc
      label: CC BY-NC-SA 4.0
      url: https://creativecommons.org/licenses/by-nc-sa/4.0/
```

导航 logo 使用原始 SVG 作为遮罩，保留 780×720 viewBox 和图形比例，透明背景上的填充跟随主题强调色；favicon 使用原始 SVG。首页与标签之间直接列出已配置且有公开文章的分类，点击进入对应文章列表。导航显示当前栏目，“关于”和“友链”只列出对应的 Public 页面，移动端链接组可换行。右栏标签区显示四行半，超出部分可用滚轮、触摸或键盘滚动；每行标签高 26px，行距 8px。RSS 和构建署名位于信息栏末尾，移动端随信息栏排在正文之后。

表格单元格保留可读的最小列宽，窄屏在表格内部横向滚动；长 URL 可在单元格内换行，避免原生 HTML 表格被压成逐字排列。

渲染回归复用仓库中的 Public、无日期独立页面和 Unlisted Standalone 样例。正文、标题转义、HTML 片段、noindex 和 tokenizer 插入位置在 `internal/render` 测试。

主题资源位于 `static/assets/`，使用系统字体链、CSS 变量、回环本地预览与原生 `<dialog>` 灯箱。深浅色选择可保存到本地存储；存储不可用时仍可切换。代码使用 Chroma 的 GitHub / GitHub Dark 配色，公式由 `static/vendor/katex-0.19.0/` 的自托管 KaTeX 增强。KaTeX 的 MIT 许可保留在该目录的 LICENSE，JS 的 SHA-256 为 `103a53763cc033bba8d175bf3f0ba597c3505c9b6747dd3f2c7bc2a6bfcc8ae7`。

主题按钮依次选择系统、浅色、深色；系统模式会跟随操作系统变化。系统图标沿左下至右上的分隔线组合太阳左上半与月亮右下半。按钮支持悬停和键盘焦点提示；触屏在点击后淡入提示，保持约 1.2 秒后淡出，再次点击重新计时，复制结果按同样规则展示。桌面导航、站点标题与信息栏保持位置，阅读区独立滚动；手机导航和同一行的站点名称、副标题固定在内容区上方，正文与信息栏一起滚动。纵向滚动条隐藏，鼠标滚轮、触摸和键盘滚动保持可用。

Mermaid 的浏览器包和 MIT 许可位于 `static/vendor/mermaid-12.2.0/`，只在包含 Mermaid 围栏的页面加载；使用官方 npm 包中的独立构建，无 CDN 运行时依赖。

站内导航使用自托管 Swup 4.10.0 与 Scroll Plugin 4.0.0，只替换 `#main`；导航、站点标题区、信息栏与主题控件保留原 DOM。正文淡入淡出为 140ms，减少动态效果时直接替换。地址、标题、canonical、Open Graph、description、robots 与当前栏目同步更新，滚动插件保存桌面阅读区和手机内容区的历史位置。完整 HTML 仍是每个 URL 的直接访问与刷新入口。

`theme.js` 的页面增强通过 `BloGoPage.init/destroy` 管理代码块、公式、图表、灯箱与阅读量；相同正文节点只初始化一次，离开时收起提示和灯箱。Mermaid 加载失败时保留源码，其他增强独立工作。`navigation.js` 在替换前取消尚未可见页面的统计，替换后按新页面的统计开关发送一次浏览事件；只提取上一页 URL 的主机名。每次访问带独立标记，延迟到达的旧统计脚本不计入新访问。热门列表保留在信息栏中，不重复添加。

作者正文包含脚本的页面、Standalone、附件、外部链接和显式 `data-no-swup` 链接沿用正常导航；请求或页面增强失败、替换容器缺失、运行时资源版本变化时回到正常页面加载。Swup 未加载或 JavaScript 关闭时仍可浏览。Swup 及滚动插件的版本目录保留各自 MIT 许可；实现依据 [Swup 生命周期](https://swup.js.org/hooks/) 和 [滚动容器说明](https://swup.js.org/plugins/scroll-plugin/)。

2026-10-23 本地 Edge/Playwright 核对首页、文章、分类、标签、归档和友链，共 1440/390 像素 × 浅色/深色的 24 组页面：无横向溢出，友链头像为 48×48，长名称换行。公式及代码高亮加载成功，灯箱可以打开并按 Escape 关闭。关闭 JavaScript 后正文、导航和公式源码仍可读。浅色辅助文字对比度为 4.67:1，强调色为 5.08:1；暗色代码残留浅色 token 的问题先复现后修正。截图保存在本地 `output/playwright/`。

构建时复制普通静态文件，并为构建器管理的 `theme.css`、`theme.js`、`navigation.js`、`stats.js`（存在时）及生成的 `highlight.css` 压缩和生成内容哈希文件名。原始静态路径仍可供作者显式引用。KaTeX 使用带版本目录，CSS 所引用的字体目录结构保持不变。CSS 本地资源通过 tokenizer 检查存在性；页面引用在完整构建阶段校验。

作者 HTML 片段、Standalone、内联脚本与样式不参与压缩或路径字符串替换。静态输入仅接受常用图片、CSS/JS、字体、媒体、PDF、JSON/XML/TXT 及 LICENSE 文件；符号链接、特殊文件和静态 HTML 不进入附件输出，HTML 页面由内容模型统一建立身份。

普通模板的 HTML 外壳在执行模板之前压缩，识别 Go 模板分隔符并保留文档结构；不注册嵌入式 JS/CSS 压缩器。作者正文在此后插入，因此正文和内联脚本/样式不被外壳压缩改写。
